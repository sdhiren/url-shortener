package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newEngine returns a gin engine using Middleware with a JSON logger writing to logs.
func newEngine(logs *bytes.Buffer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware(slog.New(slog.NewJSONHandler(logs, nil))))
	return r
}

func serve(r *gin.Engine, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	r.ServeHTTP(recorder, req)
	return recorder
}

// logLines parses the JSON log lines written to logs.
func logLines(t *testing.T, logs *bytes.Buffer) []map[string]any {
	t.Helper()
	var lines []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		if line == "" {
			continue
		}
		var entry map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &entry), line)
		lines = append(lines, entry)
	}
	return lines
}

func TestNew_ReleaseModeWritesJSON(t *testing.T) {
	var out bytes.Buffer

	New(gin.ReleaseMode, &out).Info("hello", "key", "value")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &entry))
	assert.Equal(t, "INFO", entry["level"])
	assert.Equal(t, "hello", entry["msg"])
	assert.Equal(t, "value", entry["key"])
}

func TestNew_DebugAndTestModesWriteText(t *testing.T) {
	for _, mode := range []string{gin.DebugMode, gin.TestMode} {
		t.Run(mode, func(t *testing.T) {
			var out bytes.Buffer

			New(mode, &out).Info("hello", "key", "value")

			assert.Contains(t, out.String(), "level=INFO")
			assert.Contains(t, out.String(), "msg=hello")
			assert.Contains(t, out.String(), "key=value")
			assert.False(t, json.Valid(out.Bytes()))
		})
	}
}

func TestFromContext_ReturnsStoredLogger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))

	ctx := WithLogger(context.Background(), logger)

	assert.Same(t, logger, FromContext(ctx))
}

func TestFromContext_FallsBackToDefault(t *testing.T) {
	assert.Same(t, slog.Default(), FromContext(context.Background()))
}

func TestWithLogger_DoesNotChangeParentContext(t *testing.T) {
	parent := context.Background()

	_ = WithLogger(parent, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))

	assert.Same(t, slog.Default(), FromContext(parent))
}

func TestMiddleware_LogsOneLinePerRequest(t *testing.T) {
	var logs bytes.Buffer
	r := newEngine(&logs)
	r.GET("/items/:id", func(c *gin.Context) { c.String(http.StatusOK, "hello") })

	recorder := serve(r, http.MethodGet, "/items/42", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	lines := logLines(t, &logs)
	require.Len(t, lines, 1)
	entry := lines[0]
	assert.Equal(t, "INFO", entry["level"])
	assert.Equal(t, "request", entry["msg"])
	assert.Equal(t, "GET", entry["method"])
	assert.Equal(t, "/items/42", entry["path"])
	assert.Equal(t, "/items/:id", entry["route"])
	assert.InDelta(t, 200, entry["status"], 0)
	assert.InDelta(t, 5, entry["bytes"], 0)
	assert.Equal(t, "192.0.2.1", entry["client_ip"])
	assert.GreaterOrEqual(t, entry["duration_ms"], float64(0))
	assert.Equal(t, recorder.Header().Get(RequestIDHeader), entry["request_id"])
}

func TestMiddleware_GeneratesRequestID(t *testing.T) {
	var logs bytes.Buffer
	r := newEngine(&logs)
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	first := serve(r, http.MethodGet, "/", nil).Header().Get(RequestIDHeader)
	second := serve(r, http.MethodGet, "/", nil).Header().Get(RequestIDHeader)

	assert.Regexp(t, `^[0-9a-f]{32}$`, first)
	assert.Regexp(t, `^[0-9a-f]{32}$`, second)
	assert.NotEqual(t, first, second)
}

func TestMiddleware_ReusesValidIncomingRequestID(t *testing.T) {
	for _, id := range []string{"abc-123", "req_1.2", "A", strings.Repeat("x", 64), "0f8fad5b-d9cb-469f-a165-70867728950e"} {
		t.Run(id, func(t *testing.T) {
			var logs bytes.Buffer
			r := newEngine(&logs)
			r.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

			recorder := serve(r, http.MethodGet, "/", map[string]string{RequestIDHeader: id})

			assert.Equal(t, id, recorder.Header().Get(RequestIDHeader))
			assert.Equal(t, id, logLines(t, &logs)[0]["request_id"])
		})
	}
}

func TestMiddleware_ReplacesInvalidIncomingRequestID(t *testing.T) {
	invalid := map[string]string{
		"too long":      strings.Repeat("x", 65),
		"space":         "abc def",
		"log injection": `abc" level=ERROR msg="forged`,
		"unicode":       "ïd",
		"slash":         "a/b",
		"empty value":   "",
	}

	for name, id := range invalid {
		t.Run(name, func(t *testing.T) {
			var logs bytes.Buffer
			r := newEngine(&logs)
			r.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

			recorder := serve(r, http.MethodGet, "/", map[string]string{RequestIDHeader: id})

			got := recorder.Header().Get(RequestIDHeader)
			assert.Regexp(t, `^[0-9a-f]{32}$`, got)
			assert.Equal(t, got, logLines(t, &logs)[0]["request_id"])
		})
	}
}

func TestMiddleware_HandlerLogsCarryRequestID(t *testing.T) {
	var logs bytes.Buffer
	r := newEngine(&logs)
	r.GET("/", func(c *gin.Context) {
		FromContext(c.Request.Context()).Warn("inside handler", "detail", "x")
		c.Status(http.StatusNoContent)
	})

	recorder := serve(r, http.MethodGet, "/", map[string]string{RequestIDHeader: "req-123"})

	require.Equal(t, http.StatusNoContent, recorder.Code)
	lines := logLines(t, &logs)
	require.Len(t, lines, 2)
	assert.Equal(t, "inside handler", lines[0]["msg"])
	assert.Equal(t, "WARN", lines[0]["level"])
	assert.Equal(t, "req-123", lines[0]["request_id"])
	assert.Equal(t, "request", lines[1]["msg"])
	assert.Equal(t, "req-123", lines[1]["request_id"])
}

func TestMiddleware_LogLevelByStatus(t *testing.T) {
	tests := []struct {
		status    int
		wantLevel string
	}{
		{http.StatusOK, "INFO"},
		{http.StatusFound, "INFO"},
		{http.StatusBadRequest, "INFO"},
		{http.StatusNotFound, "INFO"},
		{http.StatusInternalServerError, "ERROR"},
		{http.StatusServiceUnavailable, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			var logs bytes.Buffer
			r := newEngine(&logs)
			r.GET("/", func(c *gin.Context) { c.Status(tt.status) })

			serve(r, http.MethodGet, "/", nil)

			entry := logLines(t, &logs)[0]
			assert.Equal(t, tt.wantLevel, entry["level"])
			assert.InDelta(t, tt.status, entry["status"], 0)
		})
	}
}

func TestMiddleware_LogsUnmatchedRoutes(t *testing.T) {
	var logs bytes.Buffer
	r := newEngine(&logs)

	recorder := serve(r, http.MethodGet, "/does-not-exist", nil)

	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.NotEmpty(t, recorder.Header().Get(RequestIDHeader))
	entry := logLines(t, &logs)[0]
	assert.Equal(t, "/does-not-exist", entry["path"])
	assert.Empty(t, entry["route"])
	assert.InDelta(t, 404, entry["status"], 0)
}

func TestMiddleware_LogsZeroBytesWhenNoBody(t *testing.T) {
	var logs bytes.Buffer
	r := newEngine(&logs)
	r.HEAD("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	serve(r, http.MethodHead, "/", nil)

	assert.InDelta(t, 0, logLines(t, &logs)[0]["bytes"], 0)
}
