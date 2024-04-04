package router

import (
	"urlshortener/src/controller"
	"urlshortener/src/service"

	"github.com/gin-gonic/gin"
)

func InitRouter() *gin.Engine{
	r := gin.Default()
	shortenerService:= service.NewShortenerService()
	shortenerController := controller.NewShortenerController(*shortenerService)

	r.POST("/shorten", shortenerController.Shorten)

	return r
}