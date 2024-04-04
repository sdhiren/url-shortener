package router

import (
	"urlshortener/src/controller"
	"urlshortener/src/repository"
	"urlshortener/src/service"

	"github.com/gin-gonic/gin"
)

func InitRouter() *gin.Engine{
	r := gin.Default()

	repository := repository.NewShortenerRepository()

	shortenerService:= service.NewShortenerService("http://localhost:8080/", repository)
	shortenerController := controller.NewShortenerController(*shortenerService)

	r.POST("/shorten", shortenerController.Shorten)
	r.GET("/url/:url", shortenerController.Redirect)

	return r
}