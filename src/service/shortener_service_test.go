package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"testing"

	"urlshortener/mocks"
	"urlshortener/model/request"
	"urlshortener/src/domain"
	"urlshortener/src/logging"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
)

const (
	host     = "http://localhost:8080/"
	shortURL = "A7L4mBG1"
	longURL  = "https://www.longurl.com"
)

type shortenServiceTestSuite struct {
	suite.Suite
	context                 context.Context
	mockController          *gomock.Controller
	mockShortenerRepository *mocks.MockShortenerRepository
	mockUtil                *mocks.MockUtil
	shortenerService        ShortenService
}

func TestShortenServiceTestSuite(t *testing.T) {
	suite.Run(t, new(shortenServiceTestSuite))
}

func (suite *shortenServiceTestSuite) SetupTest() {
	suite.context = context.Background()
	suite.mockController = gomock.NewController(suite.T())
	suite.mockUtil = mocks.NewMockUtil(suite.mockController)
	suite.mockShortenerRepository = mocks.NewMockShortenerRepository(suite.mockController)
	suite.shortenerService = NewShortenerService(host, suite.mockShortenerRepository, suite.mockUtil)
}

func (suite *shortenServiceTestSuite) TearDownTest() {
	suite.mockController.Finish()
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShouldReturnShortURL_WhenCodeIsStoredForThisURL() {
	// covers both a fresh insert and a URL shortened before; the repository returns longURL in both cases
	req := request.ShortenURLRequest{URL: longURL}

	suite.mockUtil.EXPECT().GenerateShortURL(longURL).Return(shortURL).Times(1)
	suite.mockShortenerRepository.EXPECT().SaveShortURL(suite.context, longURL, shortURL).Return(longURL, nil).Times(1)

	actualResult, err := suite.shortenerService.Shorten(suite.context, req)

	suite.NoError(err)
	suite.Equal(host+shortURL, actualResult)
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShouldRetryWithSaltedCode_WhenCodeCollidesWithAnotherURL() {
	req := request.ShortenURLRequest{URL: longURL}
	otherURL := "https://other.example.com"

	gomock.InOrder(
		suite.mockUtil.EXPECT().GenerateShortURL(longURL).Return(shortURL),
		suite.mockShortenerRepository.EXPECT().SaveShortURL(suite.context, longURL, shortURL).Return(otherURL, nil),
		suite.mockUtil.EXPECT().GenerateShortURL(longURL+"\x001").Return("Salted01"),
		suite.mockShortenerRepository.EXPECT().SaveShortURL(suite.context, longURL, "Salted01").Return(longURL, nil),
	)

	actualResult, err := suite.shortenerService.Shorten(suite.context, req)

	suite.NoError(err)
	suite.Equal(host+"Salted01", actualResult)
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShouldFindSaltedCodeAgain_WhenCollidedURLIsShortenedAgain() {
	// a URL that got a salted code earlier walks the same deterministic sequence and
	// stops at its own code instead of creating a new row
	req := request.ShortenURLRequest{URL: longURL}

	gomock.InOrder(
		suite.mockUtil.EXPECT().GenerateShortURL(longURL).Return(shortURL),
		suite.mockShortenerRepository.EXPECT().SaveShortURL(suite.context, longURL, shortURL).Return("https://other.example.com", nil),
		suite.mockUtil.EXPECT().GenerateShortURL(longURL+"\x001").Return("Salted01"),
		suite.mockShortenerRepository.EXPECT().SaveShortURL(suite.context, longURL, "Salted01").Return(longURL, nil),
	)

	actualResult, err := suite.shortenerService.Shorten(suite.context, req)

	suite.NoError(err)
	suite.Equal(host+"Salted01", actualResult)
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShouldReturnError_WhenAllAttemptsCollide() {
	req := request.ShortenURLRequest{URL: longURL}

	for attempt := range maxCodeAttempts {
		code := "Code000" + strconv.Itoa(attempt)
		suite.mockUtil.EXPECT().GenerateShortURL(saltedURL(longURL, attempt)).Return(code).Times(1)
		suite.mockShortenerRepository.EXPECT().SaveShortURL(suite.context, longURL, code).Return("https://other.example.com", nil).Times(1)
	}

	actualResult, err := suite.shortenerService.Shorten(suite.context, req)

	suite.EqualError(err, "no free short code after 5 attempts")
	suite.NotErrorIs(err, domain.ErrURLNotFound)
	var invalidURLErr *domain.InvalidURLError
	suite.False(errors.As(err, &invalidURLErr))
	suite.Empty(actualResult)
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShouldStopAndReturnWrappedError_WhenRepositoryFails() {
	dbErr := errors.New("db error")
	req := request.ShortenURLRequest{URL: longURL}

	suite.mockUtil.EXPECT().GenerateShortURL(longURL).Return(shortURL).Times(1)
	suite.mockShortenerRepository.EXPECT().SaveShortURL(suite.context, longURL, shortURL).Return("", dbErr).Times(1)

	actualResult, err := suite.shortenerService.Shorten(suite.context, req)

	suite.ErrorIs(err, dbErr)
	suite.EqualError(err, "saving short url: db error")
	suite.Empty(actualResult)
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShouldStopRetrying_WhenRepositoryFailsAfterCollision() {
	dbErr := errors.New("db error")
	req := request.ShortenURLRequest{URL: longURL}

	gomock.InOrder(
		suite.mockUtil.EXPECT().GenerateShortURL(longURL).Return(shortURL),
		suite.mockShortenerRepository.EXPECT().SaveShortURL(suite.context, longURL, shortURL).Return("https://other.example.com", nil),
		suite.mockUtil.EXPECT().GenerateShortURL(longURL+"\x001").Return("Salted01"),
		suite.mockShortenerRepository.EXPECT().SaveShortURL(suite.context, longURL, "Salted01").Return("", dbErr),
	)

	_, err := suite.shortenerService.Shorten(suite.context, req)

	suite.ErrorIs(err, dbErr)
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShouldNotReturnErrorTextAsShortURL_WhenRepositoryFails() {
	req := request.ShortenURLRequest{URL: longURL}

	suite.mockUtil.EXPECT().GenerateShortURL(longURL).Return(shortURL).Times(1)
	suite.mockShortenerRepository.EXPECT().SaveShortURL(suite.context, longURL, shortURL).Return("", errors.New("pq: connection refused")).Times(1)

	actualResult, err := suite.shortenerService.Shorten(suite.context, req)

	suite.Error(err)
	suite.Empty(actualResult)
}
func (suite *shortenServiceTestSuite) Test_ResolveShortURL_ShouldRecordClickAndReturnLongURL_WhenCodeExists() {
	suite.mockShortenerRepository.EXPECT().RecordClick(suite.context, shortURL).Return(longURL, nil).Times(1)

	actualResult, err := suite.shortenerService.ResolveShortURL(suite.context, shortURL)

	suite.NoError(err)
	suite.Equal(longURL, actualResult)
}

func (suite *shortenServiceTestSuite) Test_ResolveShortURL_ShouldReturnErrURLNotFound_WhenCodeDoesNotExist() {
	suite.mockShortenerRepository.EXPECT().RecordClick(suite.context, "unknown1").Return("", domain.ErrURLNotFound).Times(1)

	actualResult, err := suite.shortenerService.ResolveShortURL(suite.context, "unknown1")

	suite.ErrorIs(err, domain.ErrURLNotFound)
	suite.EqualError(err, "short url not found")
	suite.Empty(actualResult)
}

func (suite *shortenServiceTestSuite) Test_ResolveShortURL_ShouldKeepErrURLNotFound_WhenRepositoryWrapsIt() {
	wrapped := errors.Join(errors.New("lookup"), domain.ErrURLNotFound)
	suite.mockShortenerRepository.EXPECT().RecordClick(suite.context, shortURL).Return("", wrapped).Times(1)

	_, err := suite.shortenerService.ResolveShortURL(suite.context, shortURL)

	suite.ErrorIs(err, domain.ErrURLNotFound)
}

func (suite *shortenServiceTestSuite) Test_ResolveShortURL_ShouldReturnWrappedError_WhenRepositoryReturnsError() {
	dbErr := errors.New("db error")
	suite.mockShortenerRepository.EXPECT().RecordClick(suite.context, shortURL).Return("", dbErr).Times(1)

	actualResult, err := suite.shortenerService.ResolveShortURL(suite.context, shortURL)

	suite.ErrorIs(err, dbErr)
	suite.NotErrorIs(err, domain.ErrURLNotFound)
	suite.EqualError(err, "resolving short url: db error")
	suite.Empty(actualResult)
}

func (suite *shortenServiceTestSuite) Test_ResolveShortURL_ShouldAcceptAllBase64URLCharacters() {
	codes := []string{"a", "Z", "0", "abcXYZ09", "-_-_", "Lc4KTFBE", "qv3-bns9", "a_b-c", strings.Repeat("x", 20)}

	for _, code := range codes {
		suite.Run(code, func() {
			suite.SetupTest()
			suite.mockShortenerRepository.EXPECT().RecordClick(suite.context, code).Return(longURL, nil).Times(1)

			actualResult, err := suite.shortenerService.ResolveShortURL(suite.context, code)

			suite.NoError(err)
			suite.Equal(longURL, actualResult)
		})
	}
}

func (suite *shortenServiceTestSuite) Test_ResolveShortURL_ShouldReturnErrURLNotFoundWithoutQuerying_WhenCodeIsMalformed() {
	codes := map[string]string{
		"empty":                 "",
		"too long":              strings.Repeat("x", 21),
		"space":                 "abc def",
		"slash":                 "abc/def",
		"dot":                   "abc.def",
		"percent":               "abc%20",
		"plus":                  "abc+def",
		"equals padding":        "abcd==",
		"sql injection attempt": "x' OR '1'='1",
		"unicode":               "café",
		"null byte":             "abc\x00",
		"newline":               "abc\n",
	}

	for name, code := range codes {
		suite.Run(name, func() {
			suite.SetupTest()
			suite.mockShortenerRepository.EXPECT().RecordClick(gomock.Any(), gomock.Any()).Times(0)

			actualResult, err := suite.shortenerService.ResolveShortURL(suite.context, code)

			suite.ErrorIs(err, domain.ErrURLNotFound)
			suite.Empty(actualResult)
		})
	}
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShouldReturnInvalidURLErrorWithoutTouchingRepository_WhenURLIsInvalid() {
	invalidURLs := map[string]string{
		"missing scheme":    "www.longurl.com",
		"ftp scheme":        "ftp://files.example.com",
		"javascript scheme": "javascript:alert(1)",
		"whitespace only":   "   ",
		"self link":         host + "Z9cJpkVn",
	}

	for name, invalidURL := range invalidURLs {
		suite.Run(name, func() {
			suite.SetupTest()
			suite.mockUtil.EXPECT().GenerateShortURL(gomock.Any()).Times(0)
			suite.mockShortenerRepository.EXPECT().SaveShortURL(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

			actualResult, err := suite.shortenerService.Shorten(suite.context, request.ShortenURLRequest{URL: invalidURL})

			var invalidURLErr *domain.InvalidURLError
			suite.ErrorAs(err, &invalidURLErr)
			suite.Empty(actualResult)
		})
	}
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShouldUseTrimmedURL_WhenURLHasSurroundingWhitespace() {
	req := request.ShortenURLRequest{URL: "  \t" + longURL + "\n "}

	suite.mockUtil.EXPECT().GenerateShortURL(longURL).Return(shortURL).Times(1)
	suite.mockShortenerRepository.EXPECT().SaveShortURL(suite.context, longURL, shortURL).Return(longURL, nil).Times(1)

	actualResult, err := suite.shortenerService.Shorten(suite.context, req)

	suite.NoError(err)
	suite.Equal(host+shortURL, actualResult)
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShouldSkipSelfLinkCheck_WhenHostIsNotAValidURL() {
	service := NewShortenerService("http://[::1", suite.mockShortenerRepository, suite.mockUtil)
	req := request.ShortenURLRequest{URL: longURL}

	suite.mockUtil.EXPECT().GenerateShortURL(longURL).Return(shortURL).Times(1)
	suite.mockShortenerRepository.EXPECT().SaveShortURL(suite.context, longURL, shortURL).Return(longURL, nil).Times(1)

	actualResult, err := service.Shorten(suite.context, req)

	suite.NoError(err)
	suite.Equal("http://[::1"+shortURL, actualResult)
}

func (suite *shortenServiceTestSuite) Test_LookupShortURL_ShouldReturnLongURLWithoutRecordingClick() {
	suite.mockShortenerRepository.EXPECT().GetLongURL(suite.context, shortURL).Return(longURL, nil).Times(1)
	suite.mockShortenerRepository.EXPECT().RecordClick(gomock.Any(), gomock.Any()).Times(0)

	actualResult, err := suite.shortenerService.LookupShortURL(suite.context, shortURL)

	suite.NoError(err)
	suite.Equal(longURL, actualResult)
}

func (suite *shortenServiceTestSuite) Test_LookupShortURL_ShouldReturnErrURLNotFound_WhenCodeDoesNotExist() {
	suite.mockShortenerRepository.EXPECT().GetLongURL(suite.context, "unknown1").Return("", domain.ErrURLNotFound).Times(1)

	actualResult, err := suite.shortenerService.LookupShortURL(suite.context, "unknown1")

	suite.ErrorIs(err, domain.ErrURLNotFound)
	suite.Empty(actualResult)
}

func (suite *shortenServiceTestSuite) Test_LookupShortURL_ShouldReturnWrappedError_WhenRepositoryReturnsError() {
	dbErr := errors.New("db error")
	suite.mockShortenerRepository.EXPECT().GetLongURL(suite.context, shortURL).Return("", dbErr).Times(1)

	actualResult, err := suite.shortenerService.LookupShortURL(suite.context, shortURL)

	suite.ErrorIs(err, dbErr)
	suite.EqualError(err, "resolving short url: db error")
	suite.Empty(actualResult)
}

func (suite *shortenServiceTestSuite) Test_LookupShortURL_ShouldReturnErrURLNotFoundWithoutQuerying_WhenCodeIsMalformed() {
	for _, code := range []string{"", "abc.def", strings.Repeat("x", 21)} {
		suite.Run(code, func() {
			suite.SetupTest()
			suite.mockShortenerRepository.EXPECT().GetLongURL(gomock.Any(), gomock.Any()).Times(0)

			_, err := suite.shortenerService.LookupShortURL(suite.context, code)

			suite.ErrorIs(err, domain.ErrURLNotFound)
		})
	}
}

func TestSaltedURL(t *testing.T) {
	assert.Equal(t, "https://example.com", saltedURL("https://example.com", 0))
	assert.Equal(t, "https://example.com\x001", saltedURL("https://example.com", 1))
	assert.Equal(t, "https://example.com\x004", saltedURL("https://example.com", 4))
	// deterministic: the same attempt always gives the same input
	assert.Equal(t, saltedURL("https://example.com", 2), saltedURL("https://example.com", 2))
	// every attempt gives a different input, so a different code
	seen := map[string]bool{}
	for attempt := range maxCodeAttempts {
		seen[saltedURL("https://example.com", attempt)] = true
	}
	assert.Len(t, seen, maxCodeAttempts)
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShouldLogCollisionWithRequestLogger() {
	var logs bytes.Buffer
	ctx := logging.WithLogger(suite.context, slog.New(slog.NewJSONHandler(&logs, nil)).With("request_id", "req-123"))
	req := request.ShortenURLRequest{URL: longURL}

	gomock.InOrder(
		suite.mockUtil.EXPECT().GenerateShortURL(longURL).Return(shortURL),
		suite.mockShortenerRepository.EXPECT().SaveShortURL(ctx, longURL, shortURL).Return("https://other.example.com", nil),
		suite.mockUtil.EXPECT().GenerateShortURL(longURL+"\x001").Return("Salted01"),
		suite.mockShortenerRepository.EXPECT().SaveShortURL(ctx, longURL, "Salted01").Return(longURL, nil),
	)

	_, err := suite.shortenerService.Shorten(ctx, req)

	suite.Require().NoError(err)
	var entry map[string]any
	suite.Require().NoError(json.Unmarshal(logs.Bytes(), &entry))
	suite.Equal("WARN", entry["level"])
	suite.Equal("short code collides with another url, retrying", entry["msg"])
	suite.Equal(shortURL, entry["code"])
	suite.InDelta(1, entry["attempt"], 0)
	suite.InDelta(maxCodeAttempts, entry["max_attempts"], 0)
	suite.Equal("req-123", entry["request_id"])
}
