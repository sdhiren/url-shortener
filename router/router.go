package router

import (
	"database/sql"
	"urlshortener/src/controller"
	"urlshortener/src/repository"
	"urlshortener/src/service"
	"urlshortener/src/util"

	"github.com/gin-gonic/gin"
)

func InitRouter(db *sql.DB) *gin.Engine{
	r := gin.Default()

	repository := repository.NewShortenerRepository(db)
	util := util.NewUtil()

	shortenerService:= service.NewShortenerService("http://localhost:8080/", repository, util)
	shortenerController := controller.NewShortenerController(*shortenerService)

	r.POST("/shorten", shortenerController.Shorten)
	r.GET("/url/:url", shortenerController.Redirect)

	return r
}