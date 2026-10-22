package router

import (
	"database/sql"
	"net/url"
	"strings"
	"urlshortener/src/controller"
	"urlshortener/src/repository"
	"urlshortener/src/service"
	"urlshortener/src/util"

	"github.com/gin-gonic/gin"
)

func InitRouter(db *sql.DB, baseURL string) *gin.Engine {
	r := gin.Default()

	repository := repository.NewShortenerRepository(db)
	util := util.NewUtil()

	shortenerService := service.NewShortenerService(baseURL, repository, util)
	shortenerController := controller.NewShortenerController(shortenerService)

	r.POST("/shorten", shortenerController.Shorten)
	// short links are BASE_URL + code, so serve them under BASE_URL's path
	links := r.Group(basePath(baseURL))
	links.GET("/:code", shortenerController.Redirect)
	links.HEAD("/:code", shortenerController.Redirect)

	return r
}

// basePath returns the path of baseURL without its trailing slash, e.g. "" for
// "https://sho.rt/" and "/s" for "https://example.com/s/".
func basePath(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(u.Path, "/")
}
