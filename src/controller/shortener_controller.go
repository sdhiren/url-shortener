package controller

import (
	"errors"
	"log"
	"net/http"
	model "urlshortener/model/request"
	response "urlshortener/model/response"
	"urlshortener/src/domain"
	"urlshortener/src/service"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

const (
	errURLRequired    = "url is required"
	errInvalidBody    = `request body must be JSON like {"url": "https://example.com"}`
	errURLNotFound    = "short url not found"
	errInternalServer = "internal server error"
)

type ShortenerController struct {
	service        service.ShortenService
	redirectStatus int
}

// NewShortenerController builds the controller. redirectStatus is the status code
// short links redirect with; 0 means 302 Found.
func NewShortenerController(service service.ShortenService, redirectStatus int) *ShortenerController {
	if redirectStatus == 0 {
		redirectStatus = http.StatusFound
	}
	return &ShortenerController{service: service, redirectStatus: redirectStatus}
}

func (sc *ShortenerController) Shorten(c *gin.Context) {
	var shortenURLRequest model.ShortenURLRequest
	if err := c.ShouldBindJSON(&shortenURLRequest); err != nil {
		var validationErrs validator.ValidationErrors
		if errors.As(err, &validationErrs) {
			c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: errURLRequired})
			return
		}
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: errInvalidBody})
		return
	}
	// reject obviously bad URLs here, so they never reach the service or the database
	if err := service.CheckLongURL(shortenURLRequest.URL); err != nil {
		respondWithError(c, err)
		return
	}

	shortened, err := sc.service.Shorten(c, shortenURLRequest)
	if err != nil {
		respondWithError(c, err)
		return
	}

	c.JSON(http.StatusCreated, shortened)
}

// Redirect sends the client to the long URL for the short code in the path, with
// the configured status (302 by default). 302 is the default because browsers cache
// 301s and skip the server on later visits, so those clicks would never be counted.
// HEAD requests (link checkers, previews) get the same redirect but aren't counted as clicks.
func (sc *ShortenerController) Redirect(c *gin.Context) {
	resolve := sc.service.ResolveShortURL
	if c.Request.Method == http.MethodHead {
		resolve = sc.service.LookupShortURL
	}
	longURL, err := resolve(c, c.Param("code"))
	if err != nil {
		respondWithError(c, err)
		return
	}

	c.Header("Cache-Control", "no-store")
	c.Redirect(sc.redirectStatus, longURL)
}

// Lookup returns the long URL for the short code in the path as JSON, without
// redirecting or counting a click.
func (sc *ShortenerController) Lookup(c *gin.Context) {
	code := c.Param("code")
	longURL, err := sc.service.LookupShortURL(c, code)
	if err != nil {
		respondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, response.LookupResponse{Code: code, LongURL: longURL})
}

// respondWithError maps domain errors to HTTP status codes. Unexpected errors are
// logged and hidden behind a generic message so internal details never reach clients.
func respondWithError(c *gin.Context, err error) {
	var invalidURLErr *domain.InvalidURLError
	if errors.As(err, &invalidURLErr) {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: invalidURLErr.Error()})
		return
	}
	if errors.Is(err, domain.ErrURLNotFound) {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Error: errURLNotFound})
		return
	}
	log.Printf("%s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: errInternalServer})
}
