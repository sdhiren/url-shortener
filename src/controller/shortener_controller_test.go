package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"urlshortener/mocks"
	"urlshortener/model/request"
	"urlshortener/src/domain"
	"urlshortener/src/logging"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
)

type shortenerControllerTestSuite struct {
	suite.Suite
	context             *gin.Context
	recorder            *httptest.ResponseRecorder
	controller          *gomock.Controller
	mockShortenService  *mocks.MockShortenService
	shortenerController ShortenerController
}

func TestShortenerControllerTestSuite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	suite.Run(t, new(shortenerControllerTestSuite))
}

func (suite *shortenerControllerTestSuite) SetupTest() {
	suite.recorder = httptest.NewRecorder()
	suite.context, _ = gin.CreateTestContext(suite.recorder)
	suite.controller = gomock.NewController(suite.T())
	suite.mockShortenService = mocks.NewMockShortenService(suite.controller)
	suite.shortenerController = *NewShortenerController(suite.mockShortenService)
}

func (suite *shortenerControllerTestSuite) TearDownTest() {
	suite.controller.Finish()
}

func (suite *shortenerControllerTestSuite) setShortenRequest(body string) {
	suite.context.Request = httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(body))
	suite.context.Request.Header.Set("Content-Type", "application/json")
}

func (suite *shortenerControllerTestSuite) setRedirectRequest(code string) {
	suite.context.Request = httptest.NewRequest(http.MethodGet, "/"+code, nil)
	suite.context.Params = []gin.Param{{Key: "code", Value: code}}
}

// requestContext is the context of the current HTTP request, which handlers must
// pass to the service instead of the *gin.Context itself.
func (suite *shortenerControllerTestSuite) requestContext() context.Context {
	return suite.context.Request.Context()
}

func (suite *shortenerControllerTestSuite) assertResponse(wantStatus int, wantBody string) {
	suite.Equal(wantStatus, suite.recorder.Code)
	suite.JSONEq(wantBody, suite.recorder.Body.String())
	suite.Equal("application/json; charset=utf-8", suite.recorder.Header().Get("Content-Type"))
}

func (suite *shortenerControllerTestSuite) Test_Shorten_ShouldReturnOKWithShortURL() {
	suite.setShortenRequest(`{"url": "www.test.com"}`)
	req := request.ShortenURLRequest{URL: "www.test.com"}
	suite.mockShortenService.EXPECT().Shorten(suite.requestContext(), req).Return("http://localhost:8080/Z9cJpkVn", nil).Times(1)

	suite.shortenerController.Shorten(suite.context)

	suite.assertResponse(http.StatusOK, `{"short_url": "http://localhost:8080/Z9cJpkVn"}`)
}

func (suite *shortenerControllerTestSuite) Test_Shorten_ShouldIgnoreUnknownFields() {
	suite.setShortenRequest(`{"url": "www.test.com", "extra": true}`)
	req := request.ShortenURLRequest{URL: "www.test.com"}
	suite.mockShortenService.EXPECT().Shorten(suite.requestContext(), req).Return("http://localhost:8080/Z9cJpkVn", nil).Times(1)

	suite.shortenerController.Shorten(suite.context)

	suite.assertResponse(http.StatusOK, `{"short_url": "http://localhost:8080/Z9cJpkVn"}`)
}

func (suite *shortenerControllerTestSuite) Test_Shorten_ShouldReturnBadRequest_WhenURLIsMissingOrEmpty() {
	bodies := map[string]string{
		"missing url field": `{}`,
		"empty url":         `{"url": ""}`,
		"null url":          `{"url": null}`,
		"other field only":  `{"link": "www.test.com"}`,
	}

	for name, body := range bodies {
		suite.Run(name, func() {
			suite.SetupTest()
			suite.setShortenRequest(body)
			suite.mockShortenService.EXPECT().Shorten(gomock.Any(), gomock.Any()).Times(0)

			suite.shortenerController.Shorten(suite.context)

			suite.assertResponse(http.StatusBadRequest, `{"error": "url is required"}`)
		})
	}
}

func (suite *shortenerControllerTestSuite) Test_Shorten_ShouldReturnBadRequest_WhenBodyIsInvalid() {
	bodies := map[string]string{
		"empty body":     ``,
		"malformed json": `{"url": "www.test.com"`,
		"plain text":     `www.test.com`,
		"json array":     `["www.test.com"]`,
		"url is number":  `{"url": 123}`,
		"url is object":  `{"url": {"href": "www.test.com"}}`,
		"url is array":   `{"url": ["www.test.com"]}`,
	}

	for name, body := range bodies {
		suite.Run(name, func() {
			suite.SetupTest()
			suite.setShortenRequest(body)
			suite.mockShortenService.EXPECT().Shorten(gomock.Any(), gomock.Any()).Times(0)

			suite.shortenerController.Shorten(suite.context)

			suite.assertResponse(http.StatusBadRequest, `{"error": "request body must be JSON like {\"url\": \"https://example.com\"}"}`)
		})
	}
}

func (suite *shortenerControllerTestSuite) Test_Shorten_ShouldReturnInternalServerErrorWithoutLeakingDetails_WhenServiceFails() {
	suite.setShortenRequest(`{"url": "www.test.com"}`)
	suite.mockShortenService.EXPECT().Shorten(suite.requestContext(), gomock.Any()).
		Return("", errors.New("saving short url: pq: password authentication failed for user primus")).Times(1)

	suite.shortenerController.Shorten(suite.context)

	suite.assertResponse(http.StatusInternalServerError, `{"error": "internal server error"}`)
	suite.NotContains(suite.recorder.Body.String(), "pq:")
}

func (suite *shortenerControllerTestSuite) Test_Shorten_ShouldReturnNotFound_WhenServiceReturnsErrURLNotFound() {
	suite.setShortenRequest(`{"url": "www.test.com"}`)
	suite.mockShortenService.EXPECT().Shorten(suite.requestContext(), gomock.Any()).Return("", domain.ErrURLNotFound).Times(1)

	suite.shortenerController.Shorten(suite.context)

	suite.assertResponse(http.StatusNotFound, `{"error": "short url not found"}`)
}

func (suite *shortenerControllerTestSuite) Test_Redirect_ShouldRedirectWith302ToLongURL() {
	suite.setRedirectRequest("7RtNmD6Z")
	suite.mockShortenService.EXPECT().ResolveShortURL(suite.requestContext(), "7RtNmD6Z").Return("https://www.example.com/3", nil).Times(1)

	suite.shortenerController.Redirect(suite.context)
	suite.context.Writer.WriteHeaderNow()

	suite.Equal(http.StatusFound, suite.recorder.Code)
	suite.Equal("https://www.example.com/3", suite.recorder.Header().Get("Location"))
	suite.Equal("no-store", suite.recorder.Header().Get("Cache-Control"))
}

func (suite *shortenerControllerTestSuite) Test_Redirect_ShouldKeepLongURLExactlyAsStored() {
	longURLs := []string{
		"https://example.com/a/b?q=1&r=two#section",
		"https://example.com/a%20b?q=hello%20world",
		"http://example.com:8080/",
	}

	for _, longURL := range longURLs {
		suite.Run(longURL, func() {
			suite.SetupTest()
			suite.setRedirectRequest("7RtNmD6Z")
			suite.mockShortenService.EXPECT().ResolveShortURL(suite.requestContext(), "7RtNmD6Z").Return(longURL, nil).Times(1)

			suite.shortenerController.Redirect(suite.context)
			suite.context.Writer.WriteHeaderNow()

			suite.Equal(http.StatusFound, suite.recorder.Code)
			suite.Equal(longURL, suite.recorder.Header().Get("Location"))
		})
	}
}

func (suite *shortenerControllerTestSuite) Test_Redirect_ShouldPercentEncodeNonASCIIInLocation() {
	// HTTP headers must be ASCII; browsers decode this back to https://münchen.de/straße
	suite.setRedirectRequest("7RtNmD6Z")
	suite.mockShortenService.EXPECT().ResolveShortURL(suite.requestContext(), "7RtNmD6Z").Return("https://münchen.de/straße", nil).Times(1)

	suite.shortenerController.Redirect(suite.context)
	suite.context.Writer.WriteHeaderNow()

	suite.Equal(http.StatusFound, suite.recorder.Code)
	suite.Equal("https://m%c3%bcnchen.de/stra%c3%9fe", suite.recorder.Header().Get("Location"))
}

func (suite *shortenerControllerTestSuite) Test_Redirect_ShouldPassCodeFromPathToService() {
	suite.setRedirectRequest("qv3-bns9")
	suite.mockShortenService.EXPECT().ResolveShortURL(suite.requestContext(), "qv3-bns9").Return("https://example.com", nil).Times(1)

	suite.shortenerController.Redirect(suite.context)
	suite.context.Writer.WriteHeaderNow()

	suite.Equal(http.StatusFound, suite.recorder.Code)
}

func (suite *shortenerControllerTestSuite) Test_Redirect_ShouldReturnNotFoundWithoutRedirect_WhenCodeDoesNotExist() {
	suite.setRedirectRequest("unknown1")
	suite.mockShortenService.EXPECT().ResolveShortURL(suite.requestContext(), "unknown1").Return("", domain.ErrURLNotFound).Times(1)

	suite.shortenerController.Redirect(suite.context)

	suite.assertResponse(http.StatusNotFound, `{"error": "short url not found"}`)
	suite.Empty(suite.recorder.Header().Get("Location"))
}

func (suite *shortenerControllerTestSuite) Test_Redirect_ShouldReturnNotFound_WhenErrURLNotFoundIsWrapped() {
	suite.setRedirectRequest("unknown1")
	wrapped := fmt.Errorf("lookup: %w", domain.ErrURLNotFound)
	suite.mockShortenService.EXPECT().ResolveShortURL(suite.requestContext(), "unknown1").Return("", wrapped).Times(1)

	suite.shortenerController.Redirect(suite.context)

	suite.assertResponse(http.StatusNotFound, `{"error": "short url not found"}`)
}

func (suite *shortenerControllerTestSuite) Test_Redirect_ShouldReturnInternalServerErrorWithoutLeakingDetails_WhenServiceFails() {
	suite.setRedirectRequest("7RtNmD6Z")
	suite.mockShortenService.EXPECT().ResolveShortURL(suite.requestContext(), "7RtNmD6Z").
		Return("", errors.New("resolving short url: pq: relation \"urls\" does not exist")).Times(1)

	suite.shortenerController.Redirect(suite.context)

	suite.assertResponse(http.StatusInternalServerError, `{"error": "internal server error"}`)
	suite.NotContains(suite.recorder.Body.String(), "relation")
	suite.Empty(suite.recorder.Header().Get("Location"))
}

func (suite *shortenerControllerTestSuite) Test_Shorten_ShouldReturnBadRequestWithReason_WhenURLIsInvalid() {
	suite.setShortenRequest(`{"url": "ftp://example.com"}`)
	invalidErr := &domain.InvalidURLError{Reason: "url must start with http:// or https://"}
	suite.mockShortenService.EXPECT().Shorten(suite.requestContext(), gomock.Any()).Return("", invalidErr).Times(1)

	suite.shortenerController.Shorten(suite.context)

	suite.assertResponse(http.StatusBadRequest, `{"error": "invalid url: url must start with http:// or https://"}`)
}

func (suite *shortenerControllerTestSuite) Test_Shorten_ShouldReturnBadRequestWithoutWrapperText_WhenInvalidURLErrorIsWrapped() {
	suite.setShortenRequest(`{"url": "https://sho.rt/abc"}`)
	wrapped := fmt.Errorf("internal context: %w", &domain.InvalidURLError{Reason: "url must not point to this url shortener"})
	suite.mockShortenService.EXPECT().Shorten(suite.requestContext(), gomock.Any()).Return("", wrapped).Times(1)

	suite.shortenerController.Shorten(suite.context)

	suite.assertResponse(http.StatusBadRequest, `{"error": "invalid url: url must not point to this url shortener"}`)
	suite.NotContains(suite.recorder.Body.String(), "internal context")
}

func (suite *shortenerControllerTestSuite) Test_Redirect_HeadRequestShouldRedirectWithoutRecordingClick() {
	suite.context.Request = httptest.NewRequest(http.MethodHead, "/7RtNmD6Z", nil)
	suite.context.Params = []gin.Param{{Key: "code", Value: "7RtNmD6Z"}}
	suite.mockShortenService.EXPECT().LookupShortURL(suite.requestContext(), "7RtNmD6Z").Return("https://www.example.com/3", nil).Times(1)
	suite.mockShortenService.EXPECT().ResolveShortURL(gomock.Any(), gomock.Any()).Times(0)

	suite.shortenerController.Redirect(suite.context)
	suite.context.Writer.WriteHeaderNow()

	suite.Equal(http.StatusFound, suite.recorder.Code)
	suite.Equal("https://www.example.com/3", suite.recorder.Header().Get("Location"))
	suite.Equal("no-store", suite.recorder.Header().Get("Cache-Control"))
}

func (suite *shortenerControllerTestSuite) Test_Redirect_HeadRequestShouldReturnNotFound_WhenCodeDoesNotExist() {
	suite.context.Request = httptest.NewRequest(http.MethodHead, "/unknown1", nil)
	suite.context.Params = []gin.Param{{Key: "code", Value: "unknown1"}}
	suite.mockShortenService.EXPECT().LookupShortURL(suite.requestContext(), "unknown1").Return("", domain.ErrURLNotFound).Times(1)

	suite.shortenerController.Redirect(suite.context)

	suite.Equal(http.StatusNotFound, suite.recorder.Code)
	suite.Empty(suite.recorder.Header().Get("Location"))
}

func (suite *shortenerControllerTestSuite) Test_Redirect_GetRequestShouldRecordClickNotJustLookup() {
	suite.setRedirectRequest("7RtNmD6Z")
	suite.mockShortenService.EXPECT().ResolveShortURL(suite.requestContext(), "7RtNmD6Z").Return("https://www.example.com/3", nil).Times(1)
	suite.mockShortenService.EXPECT().LookupShortURL(gomock.Any(), gomock.Any()).Times(0)

	suite.shortenerController.Redirect(suite.context)
	suite.context.Writer.WriteHeaderNow()

	suite.Equal(http.StatusFound, suite.recorder.Code)
}

type ctxKey struct{}

// Q-01: handlers pass the request's context, so cancellation and request-scoped
// values (like the request logger) reach the service and the database.
func (suite *shortenerControllerTestSuite) Test_Shorten_ShouldPassRequestContextToService() {
	suite.setShortenRequest(`{"url": "https://example.com"}`)
	suite.context.Request = suite.context.Request.WithContext(context.WithValue(suite.requestContext(), ctxKey{}, "marker"))
	suite.mockShortenService.EXPECT().Shorten(gomock.Cond(func(ctx context.Context) bool {
		return ctx.Value(ctxKey{}) == "marker"
	}), gomock.Any()).Return("http://localhost:8080/Z9cJpkVn", nil).Times(1)

	suite.shortenerController.Shorten(suite.context)

	suite.Equal(http.StatusOK, suite.recorder.Code)
}

func (suite *shortenerControllerTestSuite) Test_Redirect_ShouldPassCancellationToService() {
	suite.setRedirectRequest("7RtNmD6Z")
	ctx, cancel := context.WithCancel(suite.requestContext())
	cancel()
	suite.context.Request = suite.context.Request.WithContext(ctx)
	suite.mockShortenService.EXPECT().ResolveShortURL(gomock.Cond(func(ctx context.Context) bool {
		return errors.Is(ctx.Err(), context.Canceled)
	}), "7RtNmD6Z").Return("", context.Canceled).Times(1)

	suite.shortenerController.Redirect(suite.context)

	suite.Equal(http.StatusInternalServerError, suite.recorder.Code)
}

func (suite *shortenerControllerTestSuite) Test_RespondWithError_ShouldLogInternalErrorsWithRequestLogger() {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil)).With("request_id", "req-123")
	suite.setRedirectRequest("7RtNmD6Z")
	suite.context.Request = suite.context.Request.WithContext(logging.WithLogger(suite.requestContext(), logger))
	suite.mockShortenService.EXPECT().ResolveShortURL(gomock.Any(), "7RtNmD6Z").
		Return("", errors.New("resolving short url: connection refused")).Times(1)

	suite.shortenerController.Redirect(suite.context)

	suite.Equal(http.StatusInternalServerError, suite.recorder.Code)
	var entry map[string]any
	suite.Require().NoError(json.Unmarshal(logs.Bytes(), &entry))
	suite.Equal("ERROR", entry["level"])
	suite.Equal("request failed", entry["msg"])
	suite.Equal("resolving short url: connection refused", entry["error"])
	suite.Equal("req-123", entry["request_id"])
}

func (suite *shortenerControllerTestSuite) Test_RespondWithError_ShouldNotLogClientErrors() {
	var logs bytes.Buffer
	suite.setRedirectRequest("unknown1")
	suite.context.Request = suite.context.Request.WithContext(
		logging.WithLogger(suite.requestContext(), slog.New(slog.NewJSONHandler(&logs, nil))))
	suite.mockShortenService.EXPECT().ResolveShortURL(gomock.Any(), "unknown1").Return("", domain.ErrURLNotFound).Times(1)

	suite.shortenerController.Redirect(suite.context)

	suite.Equal(http.StatusNotFound, suite.recorder.Code)
	suite.Empty(logs.String())
}
