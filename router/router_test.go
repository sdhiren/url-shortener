package router

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestInitRouter_RegistersRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := InitRouter(&sql.DB{}, "https://sho.rt/")

	var routes []string
	for _, route := range r.Routes() {
		routes = append(routes, route.Method+" "+route.Path)
	}
	assert.ElementsMatch(t, []string{"POST /shorten", "GET /url/:url"}, routes)
}

func TestInitRouter_UnknownRouteReturnsNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := InitRouter(&sql.DB{}, "https://sho.rt/")

	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/does-not-exist", nil))

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}
