package router

import (
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"

	"urlshortener/model/response"
	"urlshortener/src/controller"
	"urlshortener/src/logging"
	"urlshortener/src/repository"
	"urlshortener/src/service"
	"urlshortener/src/util"

	"github.com/gin-gonic/gin"
)

func InitRouter(db *sql.DB, baseURL string, logger *slog.Logger) *gin.Engine {
	r := gin.New()
	// the logging middleware runs first, so a recovered panic is still logged
	// with its request ID and the request line shows the 500
	r.Use(logging.Middleware(logger), gin.CustomRecoveryWithWriter(io.Discard, recoverPanic))

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

// recoverPanic turns a panic in a handler into a logged error and a generic 500.
func recoverPanic(c *gin.Context, recovered any) {
	logging.FromContext(c.Request.Context()).Error("panic recovered",
		"panic", fmt.Sprint(recovered),
		"stack", string(debug.Stack()),
	)
	c.AbortWithStatusJSON(http.StatusInternalServerError, response.ErrorResponse{Error: "internal server error"})
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
