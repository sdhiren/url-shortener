package controller

import (
	"net/http"
	model "urlshortener/model/request"
	"urlshortener/src/service"

	"github.com/gin-gonic/gin"
)

type ShortenerController struct {
	service service.ShortenService
}

func NewShortenerController(service service.ShortenService) *ShortenerController{
	return &ShortenerController{service: service}
}

func (sc *ShortenerController) Shorten(c *gin.Context){

	var shortenURLRequest model.ShortenURLRequest
	c.ShouldBind(&shortenURLRequest)
	shortenUrl := sc.service.Shorten(c, shortenURLRequest)

	c.JSON(http.StatusOK, shortenUrl)
}

func (sc *ShortenerController) Redirect(c *gin.Context){

	url := c.Param("url")
	originalUrl := sc.service.GetOriginalURL(c, url)

	c.JSON(http.StatusOK, originalUrl)
}