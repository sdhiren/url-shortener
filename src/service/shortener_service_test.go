package service

import (
	"context"
	"errors"
	"testing"
	"urlshortener/mocks"
	model "urlshortener/model/request"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
)

const (
	host = "http://localhost:8080/"
)


type shortenServiceTestSuite struct {
	suite.Suite
	context context.Context
	mockController *gomock.Controller
	mockShortenerRepository *mocks.MockShortenerRepository
	mockUtil *mocks.MockUtil
	shortenerService ShortenService
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

func (suite *shortenServiceTestSuite) Test_Shorten_ShourlReturnShortenedURL_WhenUrlDoesNotAlreadyExists(){
	shortUrl := "A7L4mBG1"
	longUrl:= "www.longurl.com"

	req := model.ShortenURLRequest{
		URL: longUrl,
	}

	suite.mockUtil.EXPECT().GenerateShortURL(longUrl).Return(shortUrl).Times(1)
	suite.mockShortenerRepository.EXPECT().IfURLAlreadyExists(suite.context, longUrl).Return("", nil).Times(1)
	suite.mockShortenerRepository.EXPECT().SaveShotenedURL(suite.context,longUrl, shortUrl).Return(nil).Times(1)

	expectedResult := suite.shortenerService.Shorten(suite.context, req)

	suite.Equal(expectedResult, host + shortUrl)
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShourlReturnShortenedURL_WhenUrlAlreadyExists(){
	shortUrl := "A7L4mBG1"
	longUrl:= "www.longurl.com"

	req := model.ShortenURLRequest{
		URL: longUrl,
	}

	suite.mockUtil.EXPECT().GenerateShortURL(longUrl).Return(shortUrl).Times(1)
	suite.mockShortenerRepository.EXPECT().IfURLAlreadyExists(suite.context, longUrl).Return(shortUrl, nil).Times(1)

	expectedResult := suite.shortenerService.Shorten(suite.context, req)

	suite.Equal(expectedResult, host + shortUrl)
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShourlReturnShortenedURL_WhenIfURLAlreadyExistsCallreturnsError(){
	shortUrl := "A7L4mBG1"
	longUrl:= "www.longurl.com"

	req := model.ShortenURLRequest{
		URL: longUrl,
	}

	expectedErr := errors.New("db error")

	suite.mockUtil.EXPECT().GenerateShortURL(longUrl).Return(shortUrl).Times(1)
	suite.mockShortenerRepository.EXPECT().IfURLAlreadyExists(suite.context, longUrl).Return("", errors.New("db error")).Times(1)

	actualError := suite.shortenerService.Shorten(suite.context, req)

	suite.Equal(expectedErr.Error(), actualError)
}

func (suite *shortenServiceTestSuite) Test_Shorten_ShourlReturnShortenedURL_WhenSaveShotenedURLCallReturnError(){
	shortUrl := "A7L4mBG1"
	longUrl:= "www.longurl.com"

	req := model.ShortenURLRequest{
		URL: longUrl,
	}

	expectedErr := errors.New("db error")

	suite.mockUtil.EXPECT().GenerateShortURL(longUrl).Return(shortUrl).Times(1)
	suite.mockShortenerRepository.EXPECT().IfURLAlreadyExists(suite.context, longUrl).Return("", nil).Times(1)
	suite.mockShortenerRepository.EXPECT().SaveShotenedURL(suite.context,longUrl, shortUrl).Return(errors.New("db error")).Times(1)

	actualError := suite.shortenerService.Shorten(suite.context, req)

	suite.Equal(expectedErr.Error(), actualError)
}