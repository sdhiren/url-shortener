package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"urlshortener/mocks"
	model "urlshortener/model/request"
	response "urlshortener/model/response"
	"urlshortener/src/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
)

const (
	host     = "http://localhost:8080/"
	shortUrl = "A7L4mBG1"
	longUrl  = "https://www.longurl.com"
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

// shortened is the expected Shorten result for code under the test host.
func shortened(code string) response.ShortenURLResponse {
	return response.ShortenURLResponse{ShortURL: host + code, Code: code}
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShouldReturnShortURL_WhenCodeIsStoredForThisURL() {
	// covers both a fresh insert and a URL shortened before; the repository returns longUrl in both cases
	req := model.ShortenURLRequest{URL: longUrl}

	suite.mockUtil.EXPECT().GenerateShortURL(longUrl).Return(shortUrl).Times(1)
	suite.mockShortenerRepository.EXPECT().SaveShortURL(suite.context, longUrl, shortUrl).Return(longUrl, nil).Times(1)

	actualResult, err := suite.shortenerService.Shorten(suite.context, req)

	suite.NoError(err)
	suite.Equal(response.ShortenURLResponse{ShortURL: host + shortUrl, Code: shortUrl}, actualResult)
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShouldRetryWithSaltedCode_WhenCodeCollidesWithAnotherURL() {
	req := model.ShortenURLRequest{URL: longUrl}
	otherUrl := "https://other.example.com"

	gomock.InOrder(
		suite.mockUtil.EXPECT().GenerateShortURL(longUrl).Return(shortUrl),
		suite.mockShortenerRepository.EXPECT().SaveShortURL(suite.context, longUrl, shortUrl).Return(otherUrl, nil),
		suite.mockUtil.EXPECT().GenerateShortURL(longUrl+"\x001").Return("Salted01"),
		suite.mockShortenerRepository.EXPECT().SaveShortURL(suite.context, longUrl, "Salted01").Return(longUrl, nil),
	)

	actualResult, err := suite.shortenerService.Shorten(suite.context, req)

	suite.NoError(err)
	suite.Equal(shortened("Salted01"), actualResult)
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShouldFindSaltedCodeAgain_WhenCollidedURLIsShortenedAgain() {
	// a URL that got a salted code earlier walks the same deterministic sequence and
	// stops at its own code instead of creating a new row
	req := model.ShortenURLRequest{URL: longUrl}

	gomock.InOrder(
		suite.mockUtil.EXPECT().GenerateShortURL(longUrl).Return(shortUrl),
		suite.mockShortenerRepository.EXPECT().SaveShortURL(suite.context, longUrl, shortUrl).Return("https://other.example.com", nil),
		suite.mockUtil.EXPECT().GenerateShortURL(longUrl+"\x001").Return("Salted01"),
		suite.mockShortenerRepository.EXPECT().SaveShortURL(suite.context, longUrl, "Salted01").Return(longUrl, nil),
	)

	actualResult, err := suite.shortenerService.Shorten(suite.context, req)

	suite.NoError(err)
	suite.Equal(shortened("Salted01"), actualResult)
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShouldReturnError_WhenAllAttemptsCollide() {
	req := model.ShortenURLRequest{URL: longUrl}

	for attempt := range maxCodeAttempts {
		code := "Code000" + strconv.Itoa(attempt)
		suite.mockUtil.EXPECT().GenerateShortURL(saltedURL(longUrl, attempt)).Return(code).Times(1)
		suite.mockShortenerRepository.EXPECT().SaveShortURL(suite.context, longUrl, code).Return("https://other.example.com", nil).Times(1)
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
	req := model.ShortenURLRequest{URL: longUrl}

	suite.mockUtil.EXPECT().GenerateShortURL(longUrl).Return(shortUrl).Times(1)
	suite.mockShortenerRepository.EXPECT().SaveShortURL(suite.context, longUrl, shortUrl).Return("", dbErr).Times(1)

	actualResult, err := suite.shortenerService.Shorten(suite.context, req)

	suite.ErrorIs(err, dbErr)
	suite.EqualError(err, "saving short url: db error")
	suite.Empty(actualResult)
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShouldStopRetrying_WhenRepositoryFailsAfterCollision() {
	dbErr := errors.New("db error")
	req := model.ShortenURLRequest{URL: longUrl}

	gomock.InOrder(
		suite.mockUtil.EXPECT().GenerateShortURL(longUrl).Return(shortUrl),
		suite.mockShortenerRepository.EXPECT().SaveShortURL(suite.context, longUrl, shortUrl).Return("https://other.example.com", nil),
		suite.mockUtil.EXPECT().GenerateShortURL(longUrl+"\x001").Return("Salted01"),
		suite.mockShortenerRepository.EXPECT().SaveShortURL(suite.context, longUrl, "Salted01").Return("", dbErr),
	)

	_, err := suite.shortenerService.Shorten(suite.context, req)

	suite.ErrorIs(err, dbErr)
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShouldNotReturnErrorTextAsShortURL_WhenRepositoryFails() {
	req := model.ShortenURLRequest{URL: longUrl}

	suite.mockUtil.EXPECT().GenerateShortURL(longUrl).Return(shortUrl).Times(1)
	suite.mockShortenerRepository.EXPECT().SaveShortURL(suite.context, longUrl, shortUrl).Return("", errors.New("pq: connection refused")).Times(1)

	actualResult, err := suite.shortenerService.Shorten(suite.context, req)

	suite.Error(err)
	suite.Empty(actualResult)
}
func (suite *shortenServiceTestSuite) Test_ResolveShortURL_ShouldRecordClickAndReturnLongURL_WhenCodeExists() {
	suite.mockShortenerRepository.EXPECT().RecordClick(suite.context, shortUrl).Return(longUrl, nil).Times(1)

	actualResult, err := suite.shortenerService.ResolveShortURL(suite.context, shortUrl)

	suite.NoError(err)
	suite.Equal(longUrl, actualResult)
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
	suite.mockShortenerRepository.EXPECT().RecordClick(suite.context, shortUrl).Return("", wrapped).Times(1)

	_, err := suite.shortenerService.ResolveShortURL(suite.context, shortUrl)

	suite.ErrorIs(err, domain.ErrURLNotFound)
}

func (suite *shortenServiceTestSuite) Test_ResolveShortURL_ShouldReturnWrappedError_WhenRepositoryReturnsError() {
	dbErr := errors.New("db error")
	suite.mockShortenerRepository.EXPECT().RecordClick(suite.context, shortUrl).Return("", dbErr).Times(1)

	actualResult, err := suite.shortenerService.ResolveShortURL(suite.context, shortUrl)

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
			suite.mockShortenerRepository.EXPECT().RecordClick(suite.context, code).Return(longUrl, nil).Times(1)

			actualResult, err := suite.shortenerService.ResolveShortURL(suite.context, code)

			suite.NoError(err)
			suite.Equal(longUrl, actualResult)
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
	invalidUrls := map[string]string{
		"missing scheme":    "www.longurl.com",
		"ftp scheme":        "ftp://files.example.com",
		"javascript scheme": "javascript:alert(1)",
		"whitespace only":   "   ",
		"self link":         host + "Z9cJpkVn",
	}

	for name, invalidUrl := range invalidUrls {
		suite.Run(name, func() {
			suite.SetupTest()
			suite.mockUtil.EXPECT().GenerateShortURL(gomock.Any()).Times(0)
			suite.mockShortenerRepository.EXPECT().SaveShortURL(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

			actualResult, err := suite.shortenerService.Shorten(suite.context, model.ShortenURLRequest{URL: invalidUrl})

			var invalidURLErr *domain.InvalidURLError
			suite.ErrorAs(err, &invalidURLErr)
			suite.Empty(actualResult)
		})
	}
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShouldUseTrimmedURL_WhenURLHasSurroundingWhitespace() {
	req := model.ShortenURLRequest{URL: "  \t" + longUrl + "\n "}

	suite.mockUtil.EXPECT().GenerateShortURL(longUrl).Return(shortUrl).Times(1)
	suite.mockShortenerRepository.EXPECT().SaveShortURL(suite.context, longUrl, shortUrl).Return(longUrl, nil).Times(1)

	actualResult, err := suite.shortenerService.Shorten(suite.context, req)

	suite.NoError(err)
	suite.Equal(shortened(shortUrl), actualResult)
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShouldSkipSelfLinkCheck_WhenHostIsNotAValidURL() {
	service := NewShortenerService("http://[::1", suite.mockShortenerRepository, suite.mockUtil)
	req := model.ShortenURLRequest{URL: longUrl}

	suite.mockUtil.EXPECT().GenerateShortURL(longUrl).Return(shortUrl).Times(1)
	suite.mockShortenerRepository.EXPECT().SaveShortURL(suite.context, longUrl, shortUrl).Return(longUrl, nil).Times(1)

	actualResult, err := service.Shorten(suite.context, req)

	suite.NoError(err)
	suite.Equal(response.ShortenURLResponse{ShortURL: "http://[::1" + shortUrl, Code: shortUrl}, actualResult)
}

func (suite *shortenServiceTestSuite) Test_LookupShortURL_ShouldReturnLongURLWithoutRecordingClick() {
	suite.mockShortenerRepository.EXPECT().GetLongURL(suite.context, shortUrl).Return(longUrl, nil).Times(1)
	suite.mockShortenerRepository.EXPECT().RecordClick(gomock.Any(), gomock.Any()).Times(0)

	actualResult, err := suite.shortenerService.LookupShortURL(suite.context, shortUrl)

	suite.NoError(err)
	suite.Equal(longUrl, actualResult)
}

func (suite *shortenServiceTestSuite) Test_LookupShortURL_ShouldReturnErrURLNotFound_WhenCodeDoesNotExist() {
	suite.mockShortenerRepository.EXPECT().GetLongURL(suite.context, "unknown1").Return("", domain.ErrURLNotFound).Times(1)

	actualResult, err := suite.shortenerService.LookupShortURL(suite.context, "unknown1")

	suite.ErrorIs(err, domain.ErrURLNotFound)
	suite.Empty(actualResult)
}

func (suite *shortenServiceTestSuite) Test_LookupShortURL_ShouldReturnWrappedError_WhenRepositoryReturnsError() {
	dbErr := errors.New("db error")
	suite.mockShortenerRepository.EXPECT().GetLongURL(suite.context, shortUrl).Return("", dbErr).Times(1)

	actualResult, err := suite.shortenerService.LookupShortURL(suite.context, shortUrl)

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
