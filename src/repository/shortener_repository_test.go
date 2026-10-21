package repository

import (
	"context"
	"errors"
	"testing"
	"urlshortener/src/domain"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	insertQuery      = "INSERT INTO URLS (LONG_URL, SHORT_URL) VALUES ($1, $2)"
	recordClickQuery = "UPDATE URLS SET CLICK_COUNT = CLICK_COUNT + 1 WHERE SHORT_URL = $1 RETURNING LONG_URL"
	existsQuery      = "SELECT SHORT_URL FROM URLS WHERE LONG_URL = $1"
)

// newMockRepository returns a repository backed by sqlmock with exact SQL matching,
// and fails the test if any expected query was not executed.
func newMockRepository(t *testing.T) (ShortenerRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		db.Close()
	})
	return NewShortenerRepository(db), mock
}

func cancelledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func TestSaveShotenedURL_InsertsRow(t *testing.T) {
	repo, mock := newMockRepository(t)
	mock.ExpectExec(insertQuery).WithArgs("https://example.com", "A7L4mBG1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := repo.SaveShotenedURL(context.Background(), "https://example.com", "A7L4mBG1")

	assert.NoError(t, err)
}

func TestSaveShotenedURL_ReturnsWrappedError_WhenInsertFails(t *testing.T) {
	repo, mock := newMockRepository(t)
	dbErr := errors.New("duplicate key value violates unique constraint")
	mock.ExpectExec(insertQuery).WithArgs("https://example.com", "A7L4mBG1").WillReturnError(dbErr)

	err := repo.SaveShotenedURL(context.Background(), "https://example.com", "A7L4mBG1")

	assert.ErrorIs(t, err, dbErr)
	assert.EqualError(t, err, `inserting short url "A7L4mBG1": duplicate key value violates unique constraint`)
}

func TestSaveShotenedURL_StopsWhenContextIsCancelled(t *testing.T) {
	repo, _ := newMockRepository(t)

	err := repo.SaveShotenedURL(cancelledContext(), "https://example.com", "A7L4mBG1")

	assert.ErrorIs(t, err, context.Canceled)
}

func TestRecordClick_IncrementsCountAndReturnsLongURL(t *testing.T) {
	repo, mock := newMockRepository(t)
	mock.ExpectQuery(recordClickQuery).WithArgs("A7L4mBG1").
		WillReturnRows(sqlmock.NewRows([]string{"long_url"}).AddRow("https://example.com"))

	longURL, err := repo.RecordClick(context.Background(), "A7L4mBG1")

	require.NoError(t, err)
	assert.Equal(t, "https://example.com", longURL)
}

func TestRecordClick_ReturnsErrURLNotFound_WhenNoRowIsUpdated(t *testing.T) {
	repo, mock := newMockRepository(t)
	mock.ExpectQuery(recordClickQuery).WithArgs("unknown1").
		WillReturnRows(sqlmock.NewRows([]string{"long_url"}))

	longURL, err := repo.RecordClick(context.Background(), "unknown1")

	assert.ErrorIs(t, err, domain.ErrURLNotFound)
	assert.Empty(t, longURL)
}

func TestRecordClick_ReturnsWrappedError_WhenQueryFails(t *testing.T) {
	repo, mock := newMockRepository(t)
	dbErr := errors.New("connection refused")
	mock.ExpectQuery(recordClickQuery).WithArgs("A7L4mBG1").WillReturnError(dbErr)

	longURL, err := repo.RecordClick(context.Background(), "A7L4mBG1")

	assert.ErrorIs(t, err, dbErr)
	assert.NotErrorIs(t, err, domain.ErrURLNotFound)
	assert.EqualError(t, err, `recording click for "A7L4mBG1": connection refused`)
	assert.Empty(t, longURL)
}

func TestRecordClick_ReturnsWrappedError_WhenScanFails(t *testing.T) {
	repo, mock := newMockRepository(t)
	mock.ExpectQuery(recordClickQuery).WithArgs("A7L4mBG1").
		WillReturnRows(sqlmock.NewRows([]string{"long_url"}).AddRow(nil))

	longURL, err := repo.RecordClick(context.Background(), "A7L4mBG1")

	require.Error(t, err)
	assert.NotErrorIs(t, err, domain.ErrURLNotFound)
	assert.Empty(t, longURL)
}

func TestRecordClick_StopsWhenContextIsCancelled(t *testing.T) {
	repo, _ := newMockRepository(t)

	longURL, err := repo.RecordClick(cancelledContext(), "A7L4mBG1")

	assert.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, longURL)
}

func TestIfURLAlreadyExists_ReturnsShortURL_WhenLongURLExists(t *testing.T) {
	repo, mock := newMockRepository(t)
	mock.ExpectQuery(existsQuery).WithArgs("https://example.com").
		WillReturnRows(sqlmock.NewRows([]string{"short_url"}).AddRow("A7L4mBG1"))

	shortURL, err := repo.IfURLAlreadyExists(context.Background(), "https://example.com")

	require.NoError(t, err)
	assert.Equal(t, "A7L4mBG1", shortURL)
}

func TestIfURLAlreadyExists_DoesNotTouchClickCount(t *testing.T) {
	// shortening an existing URL used to bump the counter; only redirects count now.
	// sqlmock fails the test on any query that isn't expected, including an UPDATE.
	repo, mock := newMockRepository(t)
	mock.ExpectQuery(existsQuery).WithArgs("https://example.com").
		WillReturnRows(sqlmock.NewRows([]string{"short_url"}).AddRow("A7L4mBG1"))

	_, err := repo.IfURLAlreadyExists(context.Background(), "https://example.com")

	require.NoError(t, err)
}

func TestIfURLAlreadyExists_ReturnsEmpty_WhenLongURLDoesNotExist(t *testing.T) {
	repo, mock := newMockRepository(t)
	mock.ExpectQuery(existsQuery).WithArgs("https://example.com").
		WillReturnRows(sqlmock.NewRows([]string{"short_url"}))

	shortURL, err := repo.IfURLAlreadyExists(context.Background(), "https://example.com")

	require.NoError(t, err)
	assert.Empty(t, shortURL)
}

func TestIfURLAlreadyExists_ReturnsWrappedError_WhenQueryFails(t *testing.T) {
	repo, mock := newMockRepository(t)
	dbErr := errors.New("connection refused")
	mock.ExpectQuery(existsQuery).WithArgs("https://example.com").WillReturnError(dbErr)

	shortURL, err := repo.IfURLAlreadyExists(context.Background(), "https://example.com")

	assert.ErrorIs(t, err, dbErr)
	assert.EqualError(t, err, "looking up existing short url: connection refused")
	assert.Empty(t, shortURL)
}

func TestIfURLAlreadyExists_StopsWhenContextIsCancelled(t *testing.T) {
	repo, _ := newMockRepository(t)

	shortURL, err := repo.IfURLAlreadyExists(cancelledContext(), "https://example.com")

	assert.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, shortURL)
}

const getLongURLQuery = "SELECT LONG_URL FROM URLS WHERE SHORT_URL = $1"

func TestGetLongURL_ReturnsLongURLWithoutCountingClick(t *testing.T) {
	// sqlmock fails the test on any unexpected query, so no UPDATE can run
	repo, mock := newMockRepository(t)
	mock.ExpectQuery(getLongURLQuery).WithArgs("A7L4mBG1").
		WillReturnRows(sqlmock.NewRows([]string{"long_url"}).AddRow("https://example.com"))

	longURL, err := repo.GetLongURL(context.Background(), "A7L4mBG1")

	require.NoError(t, err)
	assert.Equal(t, "https://example.com", longURL)
}

func TestGetLongURL_ReturnsErrURLNotFound_WhenNoRows(t *testing.T) {
	repo, mock := newMockRepository(t)
	mock.ExpectQuery(getLongURLQuery).WithArgs("unknown1").
		WillReturnRows(sqlmock.NewRows([]string{"long_url"}))

	longURL, err := repo.GetLongURL(context.Background(), "unknown1")

	assert.ErrorIs(t, err, domain.ErrURLNotFound)
	assert.Empty(t, longURL)
}

func TestGetLongURL_ReturnsWrappedError_WhenQueryFails(t *testing.T) {
	repo, mock := newMockRepository(t)
	dbErr := errors.New("connection refused")
	mock.ExpectQuery(getLongURLQuery).WithArgs("A7L4mBG1").WillReturnError(dbErr)

	longURL, err := repo.GetLongURL(context.Background(), "A7L4mBG1")

	assert.ErrorIs(t, err, dbErr)
	assert.NotErrorIs(t, err, domain.ErrURLNotFound)
	assert.EqualError(t, err, `reading long url for "A7L4mBG1": connection refused`)
	assert.Empty(t, longURL)
}

func TestGetLongURL_StopsWhenContextIsCancelled(t *testing.T) {
	repo, _ := newMockRepository(t)

	longURL, err := repo.GetLongURL(cancelledContext(), "A7L4mBG1")

	assert.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, longURL)
}
