package controller

import (
	"errors"
	"net/http"

	"urlshortener/model/request"
	"urlshortener/model/response"
	"urlshortener/src/domain"
	"urlshortener/src/logging"
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
	service service.ShortenService
}

func NewShortenerController(service service.ShortenService) *ShortenerController {
	return &ShortenerController{service: service}
}

func (sc *ShortenerController) Shorten(c *gin.Context) {
	var shortenURLRequest request.ShortenURLRequest
	if err := c.ShouldBindJSON(&shortenURLRequest); err != nil {
		var validationErrs validator.ValidationErrors
		if errors.As(err, &validationErrs) {
			c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: errURLRequired})
			return
		}
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: errInvalidBody})
		return
	}

	// c.Request.Context() (not c itself) is canceled when the client goes away
	shortURL, err := sc.service.Shorten(c.Request.Context(), shortenURLRequest)
	if err != nil {
		respondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, response.ShortenURLResponse{ShortURL: shortURL})
}

// Redirect sends the client to the long URL for the short code in the path.
// It uses 302 rather than 301 because browsers cache 301s and skip the server
// on later visits, so those clicks would never be counted. HEAD requests (link
// checkers, previews) get the same redirect but aren't counted as clicks.
func (sc *ShortenerController) Redirect(c *gin.Context) {
	resolve := sc.service.ResolveShortURL
	if c.Request.Method == http.MethodHead {
		resolve = sc.service.LookupShortURL
	}
	longURL, err := resolve(c.Request.Context(), c.Param("code"))
	if err != nil {
		respondWithError(c, err)
		return
	}

	c.Header("Cache-Control", "no-store")
	c.Redirect(http.StatusFound, longURL)
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
	logging.FromContext(c.Request.Context()).Error("request failed", "error", err)
	c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: errInternalServer})
}
