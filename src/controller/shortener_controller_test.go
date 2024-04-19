package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"urlshortener/mocks"
	model "urlshortener/model/request"

	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
)

type shortenerControllerTestSuite struct {
	suite.Suite
	context *gin.Context
	recorder *httptest.ResponseRecorder
	controller *gomock.Controller
	mockShortenService  *mocks.MockShortenService
	shortenerController ShortenerController
}

func TestShortenerControllerTestSuite(t *testing.T){
	suite.Run(t, new(shortenerControllerTestSuite))
}

func (suite *shortenerControllerTestSuite) SetupTest() {
	suite.recorder = httptest.NewRecorder()
	suite.context, _ = gin.CreateTestContext(suite.recorder)

	reqString := `{"url": "www.test.com"}`
	suite.context.Request = httptest.NewRequest("POST", "/shorten", strings.NewReader(reqString))
	suite.context.Request.Header.Set("Content-Type", "application/json")
	suite.controller = gomock.NewController(suite.T())
	suite.mockShortenService = mocks.NewMockShortenService(suite.controller)
	suite.shortenerController = *NewShortenerController(suite.mockShortenService)
}

func (suite *shortenerControllerTestSuite) TeardownTest() {
	suite.controller.Finish()
}


func (suite *shortenerControllerTestSuite) Test_Shorten_ShouldReturnResultOK() {
	
	req := model.ShortenURLRequest{
		URL: "www.test.com",
	}

	shortUrl := "Z9cJpkVn"
	suite.mockShortenService.EXPECT().Shorten(suite.context, req).Return(shortUrl).Times(1)

	suite.shortenerController.Shorten(suite.context)

	suite.Equal(http.StatusOK, suite.recorder.Result().StatusCode)
}

func (suite *shortenerControllerTestSuite) Test_Redirect_ShouldReturnResultOK() {

	suite.context.Request = httptest.NewRequest("GET", "/url", nil)
	suite.context.Params = []gin.Param{
		{
			Key: "url",
			Value: "7RtNmD6Z",
		},
	}

	suite.context.Request.Header.Set("Content-Type", "application/json")

	url := "7RtNmD6Z"
	originalUrl := "https://www.example.com/3"
	suite.mockShortenService.EXPECT().GetOriginalURL(suite.context, url).Return(originalUrl).Times(1)

	suite.shortenerController.Redirect(suite.context)

	suite.Equal(http.StatusOK, suite.recorder.Result().StatusCode)
}

