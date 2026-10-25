package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"urlshortener/mocks"
	model "urlshortener/model/request"
	response "urlshortener/model/response"
	"urlshortener/src/domain"

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
	suite.shortenerController = *NewShortenerController(suite.mockShortenService, 0)
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

func (suite *shortenerControllerTestSuite) setLookupRequest(code string) {
	suite.context.Request = httptest.NewRequest(http.MethodGet, "/url/"+code, nil)
	suite.context.Params = []gin.Param{{Key: "code", Value: code}}
}

// captureLog redirects the standard logger into a buffer until the test ends.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	original := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(original) })
	return &buf
}

func (suite *shortenerControllerTestSuite) assertResponse(wantStatus int, wantBody string) {
	suite.Equal(wantStatus, suite.recorder.Code)
	suite.JSONEq(wantBody, suite.recorder.Body.String())
	suite.Equal("application/json; charset=utf-8", suite.recorder.Header().Get("Content-Type"))
}

var shortened = response.ShortenURLResponse{ShortURL: "http://localhost:8080/Z9cJpkVn", Code: "Z9cJpkVn"}

func (suite *shortenerControllerTestSuite) Test_Shorten_ShouldReturnCreatedWithShortURLAndCode() {
	suite.setShortenRequest(`{"url": "https://www.test.com"}`)
	req := model.ShortenURLRequest{URL: "https://www.test.com"}
	suite.mockShortenService.EXPECT().Shorten(suite.context, req).Return(shortened, nil).Times(1)

	suite.shortenerController.Shorten(suite.context)

	suite.assertResponse(http.StatusCreated, `{"short_url": "http://localhost:8080/Z9cJpkVn", "code": "Z9cJpkVn"}`)
}

func (suite *shortenerControllerTestSuite) Test_Shorten_ShouldIgnoreUnknownFields() {
	suite.setShortenRequest(`{"url": "https://www.test.com", "extra": true}`)
	req := model.ShortenURLRequest{URL: "https://www.test.com"}
	suite.mockShortenService.EXPECT().Shorten(suite.context, req).Return(shortened, nil).Times(1)

	suite.shortenerController.Shorten(suite.context)

	suite.assertResponse(http.StatusCreated, `{"short_url": "http://localhost:8080/Z9cJpkVn", "code": "Z9cJpkVn"}`)
}

func (suite *shortenerControllerTestSuite) Test_Shorten_ShouldReturnBadRequestWithoutCallingService_WhenURLIsInvalid() {
	tests := []struct {
		name   string
		url    string
		reason string
	}{
		{"not a url", "not a url", "url must not contain spaces or control characters"},
		{"no scheme", "www.test.com", "url must start with http:// or https://"},
		{"javascript", "javascript:alert(1)", "url must start with http:// or https://"},
		{"ftp", "ftp://example.com", "url must start with http:// or https://"},
		{"whitespace only", "   ", "url is required"},
		{"too long", "https://example.com/" + strings.Repeat("a", 981), "url must be at most 1000 characters"},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.SetupTest()
			body, err := json.Marshal(model.ShortenURLRequest{URL: tt.url})
			suite.Require().NoError(err)
			suite.setShortenRequest(string(body))
			suite.mockShortenService.EXPECT().Shorten(gomock.Any(), gomock.Any()).Times(0)

			suite.shortenerController.Shorten(suite.context)

			suite.assertResponse(http.StatusBadRequest, `{"error": "invalid url: `+tt.reason+`"}`)
		})
	}
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
	logs := captureLog(suite.T())
	suite.setShortenRequest(`{"url": "https://www.test.com"}`)
	suite.mockShortenService.EXPECT().Shorten(suite.context, gomock.Any()).
		Return(response.ShortenURLResponse{}, errors.New("saving short url: pq: password authentication failed for user primus")).Times(1)

	suite.shortenerController.Shorten(suite.context)

	suite.assertResponse(http.StatusInternalServerError, `{"error": "internal server error"}`)
	suite.NotContains(suite.recorder.Body.String(), "pq:")
	suite.NotContains(suite.recorder.Body.String(), "password authentication failed")
	suite.Contains(logs.String(), "POST /shorten: saving short url: pq: password authentication failed for user primus")
}

func (suite *shortenerControllerTestSuite) Test_Shorten_ShouldReturnNotFound_WhenServiceReturnsErrURLNotFound() {
	suite.setShortenRequest(`{"url": "https://www.test.com"}`)
	suite.mockShortenService.EXPECT().Shorten(suite.context, gomock.Any()).Return(response.ShortenURLResponse{}, domain.ErrURLNotFound).Times(1)

	suite.shortenerController.Shorten(suite.context)

	suite.assertResponse(http.StatusNotFound, `{"error": "short url not found"}`)
}

func (suite *shortenerControllerTestSuite) Test_Redirect_ShouldRedirectWith302ToLongURL() {
	suite.setRedirectRequest("7RtNmD6Z")
	suite.mockShortenService.EXPECT().ResolveShortURL(suite.context, "7RtNmD6Z").Return("https://www.example.com/3", nil).Times(1)

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
			suite.mockShortenService.EXPECT().ResolveShortURL(suite.context, "7RtNmD6Z").Return(longURL, nil).Times(1)

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
	suite.mockShortenService.EXPECT().ResolveShortURL(suite.context, "7RtNmD6Z").Return("https://münchen.de/straße", nil).Times(1)

	suite.shortenerController.Redirect(suite.context)
	suite.context.Writer.WriteHeaderNow()

	suite.Equal(http.StatusFound, suite.recorder.Code)
	suite.Equal("https://m%c3%bcnchen.de/stra%c3%9fe", suite.recorder.Header().Get("Location"))
}

func (suite *shortenerControllerTestSuite) Test_Redirect_ShouldPassCodeFromPathToService() {
	suite.setRedirectRequest("qv3-bns9")
	suite.mockShortenService.EXPECT().ResolveShortURL(suite.context, "qv3-bns9").Return("https://example.com", nil).Times(1)

	suite.shortenerController.Redirect(suite.context)
	suite.context.Writer.WriteHeaderNow()

	suite.Equal(http.StatusFound, suite.recorder.Code)
}

func (suite *shortenerControllerTestSuite) Test_Redirect_ShouldReturnNotFoundWithoutRedirect_WhenCodeDoesNotExist() {
	suite.setRedirectRequest("unknown1")
	suite.mockShortenService.EXPECT().ResolveShortURL(suite.context, "unknown1").Return("", domain.ErrURLNotFound).Times(1)

	suite.shortenerController.Redirect(suite.context)

	suite.assertResponse(http.StatusNotFound, `{"error": "short url not found"}`)
	suite.Empty(suite.recorder.Header().Get("Location"))
}

func (suite *shortenerControllerTestSuite) Test_Redirect_ShouldReturnNotFound_WhenErrURLNotFoundIsWrapped() {
	suite.setRedirectRequest("unknown1")
	wrapped := fmt.Errorf("lookup: %w", domain.ErrURLNotFound)
	suite.mockShortenService.EXPECT().ResolveShortURL(suite.context, "unknown1").Return("", wrapped).Times(1)

	suite.shortenerController.Redirect(suite.context)

	suite.assertResponse(http.StatusNotFound, `{"error": "short url not found"}`)
}

func (suite *shortenerControllerTestSuite) Test_Redirect_ShouldReturnInternalServerErrorWithoutLeakingDetails_WhenServiceFails() {
	suite.setRedirectRequest("7RtNmD6Z")
	suite.mockShortenService.EXPECT().ResolveShortURL(suite.context, "7RtNmD6Z").
		Return("", errors.New("resolving short url: pq: relation \"urls\" does not exist")).Times(1)

	suite.shortenerController.Redirect(suite.context)

	suite.assertResponse(http.StatusInternalServerError, `{"error": "internal server error"}`)
	suite.NotContains(suite.recorder.Body.String(), "relation")
	suite.Empty(suite.recorder.Header().Get("Location"))
}

func (suite *shortenerControllerTestSuite) Test_Shorten_ShouldReturnBadRequestWithReason_WhenServiceRejectsURL() {
	// passes the controller's checks; the private IP rule is enforced by the service
	suite.setShortenRequest(`{"url": "http://127.0.0.1"}`)
	invalidErr := &domain.InvalidURLError{Reason: "url must not point to a private, loopback or reserved IP address"}
	suite.mockShortenService.EXPECT().Shorten(suite.context, gomock.Any()).Return(response.ShortenURLResponse{}, invalidErr).Times(1)

	suite.shortenerController.Shorten(suite.context)

	suite.assertResponse(http.StatusBadRequest, `{"error": "invalid url: url must not point to a private, loopback or reserved IP address"}`)
}

func (suite *shortenerControllerTestSuite) Test_Shorten_ShouldReturnBadRequestWithoutWrapperText_WhenInvalidURLErrorIsWrapped() {
	suite.setShortenRequest(`{"url": "https://sho.rt/abc"}`)
	wrapped := fmt.Errorf("internal context: %w", &domain.InvalidURLError{Reason: "url must not point to this url shortener"})
	suite.mockShortenService.EXPECT().Shorten(suite.context, gomock.Any()).Return(response.ShortenURLResponse{}, wrapped).Times(1)

	suite.shortenerController.Shorten(suite.context)

	suite.assertResponse(http.StatusBadRequest, `{"error": "invalid url: url must not point to this url shortener"}`)
	suite.NotContains(suite.recorder.Body.String(), "internal context")
}

func (suite *shortenerControllerTestSuite) Test_Redirect_HeadRequestShouldRedirectWithoutRecordingClick() {
	suite.context.Request = httptest.NewRequest(http.MethodHead, "/7RtNmD6Z", nil)
	suite.context.Params = []gin.Param{{Key: "code", Value: "7RtNmD6Z"}}
	suite.mockShortenService.EXPECT().LookupShortURL(suite.context, "7RtNmD6Z").Return("https://www.example.com/3", nil).Times(1)
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
	suite.mockShortenService.EXPECT().LookupShortURL(suite.context, "unknown1").Return("", domain.ErrURLNotFound).Times(1)

	suite.shortenerController.Redirect(suite.context)

	suite.Equal(http.StatusNotFound, suite.recorder.Code)
	suite.Empty(suite.recorder.Header().Get("Location"))
}

func (suite *shortenerControllerTestSuite) Test_Redirect_GetRequestShouldRecordClickNotJustLookup() {
	suite.setRedirectRequest("7RtNmD6Z")
	suite.mockShortenService.EXPECT().ResolveShortURL(suite.context, "7RtNmD6Z").Return("https://www.example.com/3", nil).Times(1)
	suite.mockShortenService.EXPECT().LookupShortURL(gomock.Any(), gomock.Any()).Times(0)

	suite.shortenerController.Redirect(suite.context)
	suite.context.Writer.WriteHeaderNow()

	suite.Equal(http.StatusFound, suite.recorder.Code)
}

func (suite *shortenerControllerTestSuite) Test_Redirect_ShouldUseConfiguredRedirectStatus() {
	for _, status := range []int{http.StatusMovedPermanently, http.StatusFound, http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		suite.Run(http.StatusText(status), func() {
			suite.SetupTest()
			controller := NewShortenerController(suite.mockShortenService, status)
			suite.setRedirectRequest("7RtNmD6Z")
			suite.mockShortenService.EXPECT().ResolveShortURL(suite.context, "7RtNmD6Z").Return("https://www.example.com/3", nil).Times(1)

			controller.Redirect(suite.context)
			suite.context.Writer.WriteHeaderNow()

			suite.Equal(status, suite.recorder.Code)
			suite.Equal("https://www.example.com/3", suite.recorder.Header().Get("Location"))
			suite.Equal("no-store", suite.recorder.Header().Get("Cache-Control"))
		})
	}
}

func (suite *shortenerControllerTestSuite) Test_Lookup_ShouldReturnCodeAndLongURLWithoutRecordingClick() {
	suite.setLookupRequest("7RtNmD6Z")
	suite.mockShortenService.EXPECT().LookupShortURL(suite.context, "7RtNmD6Z").Return("https://www.example.com/3", nil).Times(1)
	suite.mockShortenService.EXPECT().ResolveShortURL(gomock.Any(), gomock.Any()).Times(0)

	suite.shortenerController.Lookup(suite.context)

	suite.assertResponse(http.StatusOK, `{"code": "7RtNmD6Z", "long_url": "https://www.example.com/3"}`)
	suite.Empty(suite.recorder.Header().Get("Location"))
}

func (suite *shortenerControllerTestSuite) Test_Lookup_ShouldReturnNotFound_WhenCodeDoesNotExist() {
	suite.setLookupRequest("unknown1")
	suite.mockShortenService.EXPECT().LookupShortURL(suite.context, "unknown1").Return("", domain.ErrURLNotFound).Times(1)

	suite.shortenerController.Lookup(suite.context)

	suite.assertResponse(http.StatusNotFound, `{"error": "short url not found"}`)
}

func (suite *shortenerControllerTestSuite) Test_Lookup_ShouldReturnInternalServerErrorWithoutLeakingDetails_WhenServiceFails() {
	logs := captureLog(suite.T())
	suite.setLookupRequest("7RtNmD6Z")
	suite.mockShortenService.EXPECT().LookupShortURL(suite.context, "7RtNmD6Z").
		Return("", errors.New("resolving short url: pq: connection refused")).Times(1)

	suite.shortenerController.Lookup(suite.context)

	suite.assertResponse(http.StatusInternalServerError, `{"error": "internal server error"}`)
	suite.NotContains(suite.recorder.Body.String(), "connection refused")
	suite.Contains(logs.String(), "GET /url/7RtNmD6Z: resolving short url: pq: connection refused")
}
