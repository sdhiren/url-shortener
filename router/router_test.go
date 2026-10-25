package router

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"urlshortener/src/logging"
	"urlshortener/src/util"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const recordClickQuery = "UPDATE URLS SET CLICK_COUNT = CLICK_COUNT + 1 WHERE SHORT_URL = $1 RETURNING LONG_URL"

// newTestRouter builds the full router (controller, service, repository) on top of
// sqlmock, and fails the test if an expected query isn't run or an unexpected one is.
func newTestRouter(t *testing.T, baseURL string) (*gin.Engine, sqlmock.Sqlmock) {
	t.Helper()
	return newTestRouterWithRedirectStatus(t, baseURL, 0)
}

func newTestRouterWithRedirectStatus(t *testing.T, baseURL string, redirectStatus int) (*gin.Engine, sqlmock.Sqlmock) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		db.Close()
	})
	return InitRouter(db, baseURL, redirectStatus, discardLogger()), mock
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func serve(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(recorder, req)
	return recorder
}

func routesOf(r *gin.Engine) []string {
	var routes []string
	for _, route := range r.Routes() {
		routes = append(routes, route.Method+" "+route.Path)
	}
	return routes
}

func TestInitRouter_RegistersRoutes(t *testing.T) {
	tests := []struct {
		baseURL string
		want    []string
	}{
		{"https://sho.rt/", []string{"POST /shorten", "GET /url/:code", "GET /:code", "HEAD /:code"}},
		{"https://example.com/s/", []string{"POST /shorten", "GET /url/:code", "GET /s/:code", "HEAD /s/:code"}},
		{"https://example.com/a/b/", []string{"POST /shorten", "GET /url/:code", "GET /a/b/:code", "HEAD /a/b/:code"}},
	}

	for _, tt := range tests {
		t.Run(tt.baseURL, func(t *testing.T) {
			gin.SetMode(gin.TestMode)

			r := InitRouter(&sql.DB{}, tt.baseURL, 0, discardLogger())

			assert.ElementsMatch(t, tt.want, routesOf(r))
		})
	}
}

func TestBasePath(t *testing.T) {
	tests := map[string]string{
		"https://sho.rt/":          "",
		"https://sho.rt":           "",
		"http://localhost:8080/":   "",
		"https://example.com/s/":   "/s",
		"https://example.com/s":    "/s",
		"https://example.com/a/b/": "/a/b",
		"http://[::1":              "",
	}

	for baseURL, want := range tests {
		t.Run(baseURL, func(t *testing.T) {
			assert.Equal(t, want, basePath(baseURL))
		})
	}
}

func TestRedirect_RedirectsAndRecordsClick(t *testing.T) {
	r, mock := newTestRouter(t, "https://sho.rt/")
	mock.ExpectQuery(recordClickQuery).WithArgs("Lc4KTFBE").
		WillReturnRows(sqlmock.NewRows([]string{"long_url"}).AddRow("https://example.com/a"))

	recorder := serve(r, http.MethodGet, "/Lc4KTFBE", "")

	assert.Equal(t, http.StatusFound, recorder.Code)
	assert.Equal(t, "https://example.com/a", recorder.Header().Get("Location"))
	assert.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
}

func TestRedirect_EachRequestRecordsOneClick(t *testing.T) {
	r, mock := newTestRouter(t, "https://sho.rt/")
	for range 3 {
		mock.ExpectQuery(recordClickQuery).WithArgs("Lc4KTFBE").
			WillReturnRows(sqlmock.NewRows([]string{"long_url"}).AddRow("https://example.com/a"))
	}

	for range 3 {
		assert.Equal(t, http.StatusFound, serve(r, http.MethodGet, "/Lc4KTFBE", "").Code)
	}
}

func TestRedirect_HeadRedirectsWithoutRecordingClick(t *testing.T) {
	// only the SELECT is expected; sqlmock fails the test if the click UPDATE runs
	r, mock := newTestRouter(t, "https://sho.rt/")
	mock.ExpectQuery(getLongURLQuery).WithArgs("Lc4KTFBE").
		WillReturnRows(sqlmock.NewRows([]string{"long_url"}).AddRow("https://example.com/a"))

	recorder := serve(r, http.MethodHead, "/Lc4KTFBE", "")

	assert.Equal(t, http.StatusFound, recorder.Code)
	assert.Equal(t, "https://example.com/a", recorder.Header().Get("Location"))
}

func TestRedirect_UnderBasePath(t *testing.T) {
	r, mock := newTestRouter(t, "https://example.com/s/")
	mock.ExpectQuery(recordClickQuery).WithArgs("Lc4KTFBE").
		WillReturnRows(sqlmock.NewRows([]string{"long_url"}).AddRow("https://example.com/a"))

	recorder := serve(r, http.MethodGet, "/s/Lc4KTFBE", "")

	assert.Equal(t, http.StatusFound, recorder.Code)
	assert.Equal(t, "https://example.com/a", recorder.Header().Get("Location"))
}

func TestRedirect_NotMountedAtRoot_WhenBaseURLHasPath(t *testing.T) {
	r, _ := newTestRouter(t, "https://example.com/s/")

	recorder := serve(r, http.MethodGet, "/Lc4KTFBE", "")

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestRedirect_UnknownCodeReturnsNotFound(t *testing.T) {
	r, mock := newTestRouter(t, "https://sho.rt/")
	mock.ExpectQuery(recordClickQuery).WithArgs("unknown1").
		WillReturnRows(sqlmock.NewRows([]string{"long_url"}))

	recorder := serve(r, http.MethodGet, "/unknown1", "")

	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.JSONEq(t, `{"error": "short url not found"}`, recorder.Body.String())
	assert.Empty(t, recorder.Header().Get("Location"))
}

func TestRedirect_MalformedCodeReturnsNotFoundWithoutQuerying(t *testing.T) {
	// sqlmock fails the test if any query runs
	r, _ := newTestRouter(t, "https://sho.rt/")

	for _, path := range []string{"/abc.def", "/abc%20def", "/" + strings.Repeat("x", 21), "/abc+def"} {
		t.Run(path, func(t *testing.T) {
			recorder := serve(r, http.MethodGet, path, "")

			assert.Equal(t, http.StatusNotFound, recorder.Code)
		})
	}
}

func TestRedirect_DatabaseErrorReturnsInternalServerError(t *testing.T) {
	r, mock := newTestRouter(t, "https://sho.rt/")
	mock.ExpectQuery(recordClickQuery).WithArgs("Lc4KTFBE").WillReturnError(sql.ErrConnDone)

	recorder := serve(r, http.MethodGet, "/Lc4KTFBE", "")

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.JSONEq(t, `{"error": "internal server error"}`, recorder.Body.String())
}

const (
	insertQuery     = "INSERT INTO URLS (LONG_URL, SHORT_URL) VALUES ($1, $2) ON CONFLICT (SHORT_URL) DO NOTHING"
	getLongURLQuery = "SELECT LONG_URL FROM URLS WHERE SHORT_URL = $1"
)

func TestShorten_ReturnsShortURLUnderBaseURL(t *testing.T) {
	r, mock := newTestRouter(t, "https://example.com/s/")
	code := util.NewUtil().GenerateShortURL("https://example.com/a")
	mock.ExpectExec(insertQuery).WithArgs("https://example.com/a", code).WillReturnResult(sqlmock.NewResult(1, 1))

	recorder := serve(r, http.MethodPost, "/shorten", `{"url": "https://example.com/a"}`)

	assert.Equal(t, http.StatusCreated, recorder.Code)
	assert.JSONEq(t, `{"short_url": "https://example.com/s/`+code+`", "code": "`+code+`"}`, recorder.Body.String())
}

func TestShorten_ExistingURLReturnsSameCodeWithoutNewRowOrClick(t *testing.T) {
	// the insert hits the unique constraint, then the stored row is read; no UPDATE runs
	r, mock := newTestRouter(t, "https://sho.rt/")
	code := util.NewUtil().GenerateShortURL("https://example.com/a")
	mock.ExpectExec(insertQuery).WithArgs("https://example.com/a", code).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(getLongURLQuery).WithArgs(code).
		WillReturnRows(sqlmock.NewRows([]string{"long_url"}).AddRow("https://example.com/a"))

	recorder := serve(r, http.MethodPost, "/shorten", `{"url": "https://example.com/a"}`)

	assert.Equal(t, http.StatusCreated, recorder.Code)
	assert.JSONEq(t, `{"short_url": "https://sho.rt/`+code+`", "code": "`+code+`"}`, recorder.Body.String())
}

func TestShorten_CollisionGetsDifferentCode(t *testing.T) {
	r, mock := newTestRouter(t, "https://sho.rt/")
	firstCode := util.NewUtil().GenerateShortURL("https://example.com/a")
	saltedCode := util.NewUtil().GenerateShortURL("https://example.com/a\x001")
	require.NotEqual(t, firstCode, saltedCode)
	mock.ExpectExec(insertQuery).WithArgs("https://example.com/a", firstCode).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(getLongURLQuery).WithArgs(firstCode).
		WillReturnRows(sqlmock.NewRows([]string{"long_url"}).AddRow("https://other.example.com"))
	mock.ExpectExec(insertQuery).WithArgs("https://example.com/a", saltedCode).WillReturnResult(sqlmock.NewResult(1, 1))

	recorder := serve(r, http.MethodPost, "/shorten", `{"url": "https://example.com/a"}`)

	assert.Equal(t, http.StatusCreated, recorder.Code)
	assert.JSONEq(t, `{"short_url": "https://sho.rt/`+saltedCode+`", "code": "`+saltedCode+`"}`, recorder.Body.String())
}

func TestShorten_DatabaseErrorReturnsInternalServerError(t *testing.T) {
	r, mock := newTestRouter(t, "https://sho.rt/")
	mock.ExpectExec(insertQuery).WithArgs("https://example.com/a", sqlmock.AnyArg()).WillReturnError(sql.ErrConnDone)

	recorder := serve(r, http.MethodPost, "/shorten", `{"url": "https://example.com/a"}`)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.JSONEq(t, `{"error": "internal server error"}`, recorder.Body.String())
}

func TestShorten_InvalidInputReturnsBadRequestWithoutQuerying(t *testing.T) {
	// sqlmock fails the test if any query runs, so nothing reaches the database
	r, _ := newTestRouter(t, "https://sho.rt/")
	bodies := map[string]string{
		"empty object":   `{}`,
		"not a url":      `{"url": "not a url"}`,
		"no scheme":      `{"url": "www.test.com"}`,
		"javascript":     `{"url": "javascript:alert(1)"}`,
		"ftp":            `{"url": "ftp://example.com"}`,
		"too long":       `{"url": "https://example.com/` + strings.Repeat("a", 981) + `"}`,
		"malformed json": `{"url": "https://example.com"`,
		"private ip":     `{"url": "http://127.0.0.1"}`,
		"self link":      `{"url": "https://sho.rt/Lc4KTFBE"}`,
	}

	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			recorder := serve(r, http.MethodPost, "/shorten", body)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Equal(t, "application/json; charset=utf-8", recorder.Header().Get("Content-Type"))
			assert.Contains(t, recorder.Body.String(), `"error":`)
		})
	}
}

func TestRedirect_UsesConfiguredRedirectStatus(t *testing.T) {
	r, mock := newTestRouterWithRedirectStatus(t, "https://sho.rt/", http.StatusMovedPermanently)
	mock.ExpectQuery(recordClickQuery).WithArgs("Lc4KTFBE").
		WillReturnRows(sqlmock.NewRows([]string{"long_url"}).AddRow("https://example.com/a"))

	recorder := serve(r, http.MethodGet, "/Lc4KTFBE", "")

	assert.Equal(t, http.StatusMovedPermanently, recorder.Code)
	assert.Equal(t, "https://example.com/a", recorder.Header().Get("Location"))
}

func TestLookup_ReturnsLongURLWithoutRecordingClick(t *testing.T) {
	// only the SELECT is expected; sqlmock fails the test if the click UPDATE runs
	r, mock := newTestRouter(t, "https://sho.rt/")
	mock.ExpectQuery(getLongURLQuery).WithArgs("Lc4KTFBE").
		WillReturnRows(sqlmock.NewRows([]string{"long_url"}).AddRow("https://example.com/a"))

	recorder := serve(r, http.MethodGet, "/url/Lc4KTFBE", "")

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"code": "Lc4KTFBE", "long_url": "https://example.com/a"}`, recorder.Body.String())
	assert.Empty(t, recorder.Header().Get("Location"))
}

func TestLookup_ServedAtRoot_WhenBaseURLHasPath(t *testing.T) {
	r, mock := newTestRouter(t, "https://example.com/s/")
	mock.ExpectQuery(getLongURLQuery).WithArgs("Lc4KTFBE").
		WillReturnRows(sqlmock.NewRows([]string{"long_url"}).AddRow("https://example.com/a"))

	recorder := serve(r, http.MethodGet, "/url/Lc4KTFBE", "")

	assert.Equal(t, http.StatusOK, recorder.Code)
}

func TestLookup_UnknownCodeReturnsNotFound(t *testing.T) {
	r, mock := newTestRouter(t, "https://sho.rt/")
	mock.ExpectQuery(getLongURLQuery).WithArgs("unknown1").WillReturnRows(sqlmock.NewRows([]string{"long_url"}))

	recorder := serve(r, http.MethodGet, "/url/unknown1", "")

	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Equal(t, "application/json; charset=utf-8", recorder.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"error": "short url not found"}`, recorder.Body.String())
}

func TestLookup_MalformedCodeReturnsNotFoundWithoutQuerying(t *testing.T) {
	r, _ := newTestRouter(t, "https://sho.rt/")

	recorder := serve(r, http.MethodGet, "/url/"+strings.Repeat("x", 21), "")

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestLookup_DatabaseErrorReturnsInternalServerError(t *testing.T) {
	r, mock := newTestRouter(t, "https://sho.rt/")
	mock.ExpectQuery(getLongURLQuery).WithArgs("Lc4KTFBE").WillReturnError(sql.ErrConnDone)

	recorder := serve(r, http.MethodGet, "/url/Lc4KTFBE", "")

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.JSONEq(t, `{"error": "internal server error"}`, recorder.Body.String())
}

func TestRouter_UnknownRouteReturnsNotFound(t *testing.T) {
	r, _ := newTestRouter(t, "https://sho.rt/")

	for _, path := range []string{"/a/b", "/url/a/b"} {
		t.Run(path, func(t *testing.T) {
			assert.Equal(t, http.StatusNotFound, serve(r, http.MethodGet, path, "").Code)
		})
	}
}

func TestInitRouter_LogsRequestsWithRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery(recordClickQuery).WithArgs("Lc4KTFBE").
		WillReturnRows(sqlmock.NewRows([]string{"long_url"}).AddRow("https://example.com/a"))
	r := InitRouter(db, "https://sho.rt/", 0, slog.New(slog.NewJSONHandler(&logs, nil)))

	recorder := serve(r, http.MethodGet, "/Lc4KTFBE", "")

	assert.Equal(t, http.StatusFound, recorder.Code)
	requestID := recorder.Header().Get("X-Request-ID")
	assert.Regexp(t, `^[0-9a-f]{32}$`, requestID)
	var entry map[string]any
	require.NoError(t, json.Unmarshal(logs.Bytes(), &entry))
	assert.Equal(t, "request", entry["msg"])
	assert.Equal(t, "/:code", entry["route"])
	assert.InDelta(t, 302, entry["status"], 0)
	assert.Equal(t, requestID, entry["request_id"])
}

func TestRecoverPanic_ReturnsGeneric500AndLogsPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	r := gin.New()
	r.Use(logging.Middleware(slog.New(slog.NewJSONHandler(&logs, nil))), gin.CustomRecoveryWithWriter(io.Discard, recoverPanic))
	r.GET("/panic", func(*gin.Context) { panic("boom: secret internal detail") })

	recorder := serve(r, http.MethodGet, "/panic", "")

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.JSONEq(t, `{"error": "internal server error"}`, recorder.Body.String())
	assert.NotContains(t, recorder.Body.String(), "secret")

	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	require.Len(t, lines, 2)
	var panicEntry, requestEntry map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &panicEntry))
	require.NoError(t, json.Unmarshal([]byte(lines[1]), &requestEntry))
	assert.Equal(t, "panic recovered", panicEntry["msg"])
	assert.Equal(t, "ERROR", panicEntry["level"])
	assert.Equal(t, "boom: secret internal detail", panicEntry["panic"])
	assert.Contains(t, panicEntry["stack"], "router_test.go")
	assert.Equal(t, recorder.Header().Get("X-Request-ID"), panicEntry["request_id"])
	assert.Equal(t, "ERROR", requestEntry["level"])
	assert.InDelta(t, 500, requestEntry["status"], 0)
}
