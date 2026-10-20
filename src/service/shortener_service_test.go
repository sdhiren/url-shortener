package service

import (
	"context"
	"errors"
	"testing"
	"urlshortener/mocks"
	model "urlshortener/model/request"
	"urlshortener/src/domain"

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

func (suite *shortenServiceTestSuite) Test_Shorten_ShouldSaveAndReturnShortURL_WhenUrlDoesNotAlreadyExist() {
	req := model.ShortenURLRequest{URL: longUrl}

	suite.mockUtil.EXPECT().GenerateShortURL(longUrl).Return(shortUrl).Times(1)
	suite.mockShortenerRepository.EXPECT().IfURLAlreadyExists(suite.context, longUrl).Return("", nil).Times(1)
	suite.mockShortenerRepository.EXPECT().SaveShotenedURL(suite.context, longUrl, shortUrl).Return(nil).Times(1)

	actualResult, err := suite.shortenerService.Shorten(suite.context, req)

	suite.NoError(err)
	suite.Equal(host+shortUrl, actualResult)
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShouldReturnExistingShortURLWithoutSaving_WhenUrlAlreadyExists() {
	existingShortUrl := "Xy12Ab34"
	req := model.ShortenURLRequest{URL: longUrl}

	suite.mockUtil.EXPECT().GenerateShortURL(longUrl).Return(shortUrl).Times(1)
	suite.mockShortenerRepository.EXPECT().IfURLAlreadyExists(suite.context, longUrl).Return(existingShortUrl, nil).Times(1)
	suite.mockShortenerRepository.EXPECT().SaveShotenedURL(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	actualResult, err := suite.shortenerService.Shorten(suite.context, req)

	suite.NoError(err)
	suite.Equal(host+existingShortUrl, actualResult)
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShouldReturnWrappedError_WhenIfURLAlreadyExistsReturnsError() {
	dbErr := errors.New("db error")
	req := model.ShortenURLRequest{URL: longUrl}

	suite.mockUtil.EXPECT().GenerateShortURL(longUrl).Return(shortUrl).Times(1)
	suite.mockShortenerRepository.EXPECT().IfURLAlreadyExists(suite.context, longUrl).Return("", dbErr).Times(1)
	suite.mockShortenerRepository.EXPECT().SaveShotenedURL(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	actualResult, err := suite.shortenerService.Shorten(suite.context, req)

	suite.ErrorIs(err, dbErr)
	suite.EqualError(err, "checking if url exists: db error")
	suite.Empty(actualResult)
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShouldReturnWrappedError_WhenSaveShotenedURLReturnsError() {
	dbErr := errors.New("db error")
	req := model.ShortenURLRequest{URL: longUrl}

	suite.mockUtil.EXPECT().GenerateShortURL(longUrl).Return(shortUrl).Times(1)
	suite.mockShortenerRepository.EXPECT().IfURLAlreadyExists(suite.context, longUrl).Return("", nil).Times(1)
	suite.mockShortenerRepository.EXPECT().SaveShotenedURL(suite.context, longUrl, shortUrl).Return(dbErr).Times(1)

	actualResult, err := suite.shortenerService.Shorten(suite.context, req)

	suite.ErrorIs(err, dbErr)
	suite.EqualError(err, "saving short url: db error")
	suite.Empty(actualResult)
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShouldNotReturnErrorTextAsShortURL_WhenRepositoryFails() {
	req := model.ShortenURLRequest{URL: longUrl}

	suite.mockUtil.EXPECT().GenerateShortURL(longUrl).Return(shortUrl).Times(1)
	suite.mockShortenerRepository.EXPECT().IfURLAlreadyExists(suite.context, longUrl).Return("", errors.New("pq: connection refused")).Times(1)

	actualResult, err := suite.shortenerService.Shorten(suite.context, req)

	suite.Error(err)
	suite.Empty(actualResult)
}

func (suite *shortenServiceTestSuite) Test_GetOriginalURL_ShouldReturnLongURL_WhenShortURLExists() {
	suite.mockShortenerRepository.EXPECT().GetOriginalURL(suite.context, shortUrl).Return(longUrl, nil).Times(1)

	actualResult, err := suite.shortenerService.GetOriginalURL(suite.context, shortUrl)

	suite.NoError(err)
	suite.Equal(longUrl, actualResult)
}

func (suite *shortenServiceTestSuite) Test_GetOriginalURL_ShouldReturnErrURLNotFound_WhenShortURLDoesNotExist() {
	suite.mockShortenerRepository.EXPECT().GetOriginalURL(suite.context, "unknown1").Return("", domain.ErrURLNotFound).Times(1)

	actualResult, err := suite.shortenerService.GetOriginalURL(suite.context, "unknown1")

	suite.ErrorIs(err, domain.ErrURLNotFound)
	suite.EqualError(err, "short url not found")
	suite.Empty(actualResult)
}

func (suite *shortenServiceTestSuite) Test_GetOriginalURL_ShouldKeepErrURLNotFound_WhenRepositoryWrapsIt() {
	wrapped := errors.Join(errors.New("lookup"), domain.ErrURLNotFound)
	suite.mockShortenerRepository.EXPECT().GetOriginalURL(suite.context, shortUrl).Return("", wrapped).Times(1)

	_, err := suite.shortenerService.GetOriginalURL(suite.context, shortUrl)

	suite.ErrorIs(err, domain.ErrURLNotFound)
}

func (suite *shortenServiceTestSuite) Test_GetOriginalURL_ShouldReturnWrappedError_WhenRepositoryReturnsError() {
	dbErr := errors.New("db error")
	suite.mockShortenerRepository.EXPECT().GetOriginalURL(suite.context, shortUrl).Return("", dbErr).Times(1)

	actualResult, err := suite.shortenerService.GetOriginalURL(suite.context, shortUrl)

	suite.ErrorIs(err, dbErr)
	suite.NotErrorIs(err, domain.ErrURLNotFound)
	suite.EqualError(err, "getting original url: db error")
	suite.Empty(actualResult)
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
			suite.mockShortenerRepository.EXPECT().IfURLAlreadyExists(gomock.Any(), gomock.Any()).Times(0)
			suite.mockShortenerRepository.EXPECT().SaveShotenedURL(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

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
	suite.mockShortenerRepository.EXPECT().IfURLAlreadyExists(suite.context, longUrl).Return("", nil).Times(1)
	suite.mockShortenerRepository.EXPECT().SaveShotenedURL(suite.context, longUrl, shortUrl).Return(nil).Times(1)

	actualResult, err := suite.shortenerService.Shorten(suite.context, req)

	suite.NoError(err)
	suite.Equal(host+shortUrl, actualResult)
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShouldSkipSelfLinkCheck_WhenHostIsNotAValidURL() {
	service := NewShortenerService("http://[::1", suite.mockShortenerRepository, suite.mockUtil)
	req := model.ShortenURLRequest{URL: longUrl}

	suite.mockUtil.EXPECT().GenerateShortURL(longUrl).Return(shortUrl).Times(1)
	suite.mockShortenerRepository.EXPECT().IfURLAlreadyExists(suite.context, longUrl).Return("", nil).Times(1)
	suite.mockShortenerRepository.EXPECT().SaveShotenedURL(suite.context, longUrl, shortUrl).Return(nil).Times(1)

	actualResult, err := service.Shorten(suite.context, req)

	suite.NoError(err)
	suite.Equal("http://[::1"+shortUrl, actualResult)
}
