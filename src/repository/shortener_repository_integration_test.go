//go:build integration

package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"urlshortener/model/request"
	"urlshortener/model/response"
	"urlshortener/src/repository"
	"urlshortener/src/service"
	"urlshortener/src/util"
)

// These tests run the real SQL against Postgres: make test-integration with
// TEST_DATABASE_URL set, e.g. postgres://user:pass@localhost:5432/main?sslmode=disable.
// Each test gets its own schema with the Liquibase migrations applied, dropped afterwards.

const testHost = "https://sho.rt/"

// newTestDB creates a throwaway schema, applies every migration in
// infrastructure/changelog/sql to it, and returns a pool that uses only that schema.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { admin.Close() })
	schema := fmt.Sprintf("it_%d", time.Now().UnixNano())
	_, err = admin.Exec("CREATE SCHEMA " + schema)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := admin.Exec("DROP SCHEMA " + schema + " CASCADE")
		assert.NoError(t, err)
	})

	// lib/pq sends unknown URL parameters as run-time settings, so every pooled
	// connection starts with this search_path
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	migrations, err := filepath.Glob("../../infrastructure/changelog/sql/*.sql")
	require.NoError(t, err)
	require.NotEmpty(t, migrations)
	sort.Strings(migrations)
	for _, migration := range migrations {
		script, err := os.ReadFile(migration)
		require.NoError(t, err)
		// without arguments lib/pq uses the simple protocol, which runs multi-statement scripts
		_, err = db.Exec(string(script))
		require.NoError(t, err, migration)
	}
	return db
}

func newTestService(db *sql.DB) service.ShortenService {
	return service.NewShortenerService(testHost, repository.NewShortenerRepository(db), util.NewUtil())
}

func countRows(t *testing.T, db *sql.DB, longURL string) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM URLS WHERE LONG_URL = $1", longURL).Scan(&n))
	return n
}

func TestIntegration_ConcurrentShortensOfNewURLStoreOneRowAndReturnOneCode(t *testing.T) {
	db := newTestDB(t)
	svc := newTestService(db)
	const longURL = "https://example.com/concurrent"
	const requests = 50

	results := make([]response.ShortenURLResponse, requests)
	errs := make([]error, requests)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range requests {
		wg.Go(func() {
			<-start
			results[i], errs[i] = svc.Shorten(context.Background(), request.ShortenURLRequest{URL: longURL})
		})
	}
	close(start)
	wg.Wait()

	for i := range requests {
		require.NoError(t, errs[i])
		assert.Equal(t, results[0], results[i])
	}
	assert.Equal(t, util.NewUtil().GenerateShortURL(longURL, 0), results[0].Code)
	assert.Equal(t, 1, countRows(t, db, longURL))
	var clicks int
	require.NoError(t, db.QueryRow("SELECT CLICK_COUNT FROM URLS WHERE LONG_URL = $1", longURL).Scan(&clicks))
	assert.Zero(t, clicks, "shortening must not count clicks")
}

func TestIntegration_SaveShortURLReturnsStoredCode_WhenURLExists(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewShortenerRepository(db)
	ctx := context.Background()

	inserted, err := repo.SaveShortURL(ctx, "https://example.com/a", "Code0001")
	require.NoError(t, err)
	again, err := repo.SaveShortURL(ctx, "https://example.com/a", "Code0002")
	require.NoError(t, err)

	assert.Equal(t, "Code0001", inserted)
	assert.Equal(t, "Code0001", again, "the stored code wins over the new candidate")
	assert.Equal(t, 1, countRows(t, db, "https://example.com/a"))
}

func TestIntegration_TakenCodeReturnsErrShortURLTakenAndServiceUsesSaltedCode(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewShortenerRepository(db)
	svc := newTestService(db)
	ctx := context.Background()
	const victimURL = "https://example.com/victim"
	firstCode := util.NewUtil().GenerateShortURL(victimURL, 0)
	saltedCode := util.NewUtil().GenerateShortURL(victimURL, 1)
	// a different URL already owns the victim's first code
	_, err := db.Exec("INSERT INTO URLS (LONG_URL, SHORT_URL) VALUES ($1, $2)", "https://other.example.com", firstCode)
	require.NoError(t, err)

	_, err = repo.SaveShortURL(ctx, victimURL, firstCode)
	require.ErrorIs(t, err, repository.ErrShortURLTaken)

	shortened, err := svc.Shorten(ctx, request.ShortenURLRequest{URL: victimURL})
	require.NoError(t, err)
	assert.Equal(t, saltedCode, shortened.Code)

	// shortening again finds the salted code in one statement, without a new row
	again, err := repo.SaveShortURL(ctx, victimURL, firstCode)
	require.NoError(t, err)
	assert.Equal(t, saltedCode, again)
	assert.Equal(t, 1, countRows(t, db, victimURL))
	owner, err := repo.GetLongURL(ctx, firstCode)
	require.NoError(t, err)
	assert.Equal(t, "https://other.example.com", owner, "the existing link is not taken over")
}

func TestIntegration_RecordClickAndGetLongURL(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewShortenerRepository(db)
	ctx := context.Background()
	code, err := repo.SaveShortURL(ctx, "https://example.com/clicks", "Click001")
	require.NoError(t, err)

	for range 2 {
		longURL, err := repo.RecordClick(ctx, code)
		require.NoError(t, err)
		assert.Equal(t, "https://example.com/clicks", longURL)
	}
	longURL, err := repo.GetLongURL(ctx, code)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/clicks", longURL)

	var clicks int
	require.NoError(t, db.QueryRow("SELECT CLICK_COUNT FROM URLS WHERE SHORT_URL = $1", code).Scan(&clicks))
	assert.Equal(t, 2, clicks)
}

func TestIntegration_QueriesUseUniqueIndexes(t *testing.T) {
	db := newTestDB(t)
	_, err := db.Exec(`INSERT INTO URLS (LONG_URL, SHORT_URL)
		SELECT 'https://example.com/' || g, 'c' || g FROM generate_series(1, 10000) g`)
	require.NoError(t, err)
	_, err = db.Exec("ANALYZE URLS")
	require.NoError(t, err)

	lookupPlan := explain(t, db, "SELECT LONG_URL FROM URLS WHERE SHORT_URL = 'x'")
	assert.Contains(t, lookupPlan, "Index Scan using urls_short_url_key")

	clickPlan := explain(t, db, "UPDATE URLS SET CLICK_COUNT = CLICK_COUNT + 1 WHERE SHORT_URL = 'x' RETURNING LONG_URL")
	assert.Contains(t, clickPlan, "Index Scan using urls_short_url_key")

	savePlan := explain(t, db, "INSERT INTO URLS (LONG_URL, SHORT_URL) VALUES ('https://example.com/x', 'x') "+
		"ON CONFLICT ((md5(LONG_URL))) DO UPDATE SET LONG_URL = URLS.LONG_URL RETURNING SHORT_URL, LONG_URL")
	assert.Contains(t, savePlan, "Conflict Arbiter Indexes: ux_urls_long_url_md5")
}

// explain returns the query plan for query as one string, without running it.
func explain(t *testing.T, db *sql.DB, query string) string {
	t.Helper()
	rows, err := db.Query("EXPLAIN " + query)
	require.NoError(t, err)
	defer rows.Close()
	var plan string
	for rows.Next() {
		var line string
		require.NoError(t, rows.Scan(&line))
		plan += line + "\n"
	}
	require.NoError(t, rows.Err())
	return plan
}
