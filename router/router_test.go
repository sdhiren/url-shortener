package router

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	gin.SetMode(gin.TestMode)
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		db.Close()
	})
	return InitRouter(db, baseURL), mock
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
		{"https://sho.rt/", []string{"POST /shorten", "GET /:code", "HEAD /:code"}},
		{"https://example.com/s/", []string{"POST /shorten", "GET /s/:code", "HEAD /s/:code"}},
		{"https://example.com/a/b/", []string{"POST /shorten", "GET /a/b/:code", "HEAD /a/b/:code"}},
	}

	for _, tt := range tests {
		t.Run(tt.baseURL, func(t *testing.T) {
			gin.SetMode(gin.TestMode)

			r := InitRouter(&sql.DB{}, tt.baseURL)

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

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"short_url": "https://example.com/s/`+code+`"}`, recorder.Body.String())
}

func TestShorten_ExistingURLReturnsSameCodeWithoutNewRowOrClick(t *testing.T) {
	// the insert hits the unique constraint, then the stored row is read; no UPDATE runs
	r, mock := newTestRouter(t, "https://sho.rt/")
	code := util.NewUtil().GenerateShortURL("https://example.com/a")
	mock.ExpectExec(insertQuery).WithArgs("https://example.com/a", code).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(getLongURLQuery).WithArgs(code).
		WillReturnRows(sqlmock.NewRows([]string{"long_url"}).AddRow("https://example.com/a"))

	recorder := serve(r, http.MethodPost, "/shorten", `{"url": "https://example.com/a"}`)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"short_url": "https://sho.rt/`+code+`"}`, recorder.Body.String())
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

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"short_url": "https://sho.rt/`+saltedCode+`"}`, recorder.Body.String())
}

func TestShorten_DatabaseErrorReturnsInternalServerError(t *testing.T) {
	r, mock := newTestRouter(t, "https://sho.rt/")
	mock.ExpectExec(insertQuery).WithArgs("https://example.com/a", sqlmock.AnyArg()).WillReturnError(sql.ErrConnDone)

	recorder := serve(r, http.MethodPost, "/shorten", `{"url": "https://example.com/a"}`)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.JSONEq(t, `{"error": "internal server error"}`, recorder.Body.String())
}

func TestRouter_UnknownRouteReturnsNotFound(t *testing.T) {
	r, _ := newTestRouter(t, "https://sho.rt/")

	for _, path := range []string{"/a/b", "/url/Lc4KTFBE"} {
		t.Run(path, func(t *testing.T) {
			assert.Equal(t, http.StatusNotFound, serve(r, http.MethodGet, path, "").Code)
		})
	}
}
