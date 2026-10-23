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
	insertQuery      = "INSERT INTO URLS (LONG_URL, SHORT_URL) VALUES ($1, $2) ON CONFLICT (SHORT_URL) DO NOTHING"
	recordClickQuery = "UPDATE URLS SET CLICK_COUNT = CLICK_COUNT + 1 WHERE SHORT_URL = $1 RETURNING LONG_URL"
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

func TestSaveShortURL_ReturnsLongURLWithoutReading_WhenInserted(t *testing.T) {
	// sqlmock fails the test if the follow-up SELECT runs
	repo, mock := newMockRepository(t)
	mock.ExpectExec(insertQuery).WithArgs("https://example.com", "A7L4mBG1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	storedURL, err := repo.SaveShortURL(context.Background(), "https://example.com", "A7L4mBG1")

	require.NoError(t, err)
	assert.Equal(t, "https://example.com", storedURL)
}

func TestSaveShortURL_ReturnsSameURL_WhenCodeAlreadyStoredForIt(t *testing.T) {
	// e.g. the URL was shortened before, or a concurrent request inserted it first
	repo, mock := newMockRepository(t)
	mock.ExpectExec(insertQuery).WithArgs("https://example.com", "A7L4mBG1").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(getLongURLQuery).WithArgs("A7L4mBG1").
		WillReturnRows(sqlmock.NewRows([]string{"long_url"}).AddRow("https://example.com"))

	storedURL, err := repo.SaveShortURL(context.Background(), "https://example.com", "A7L4mBG1")

	require.NoError(t, err)
	assert.Equal(t, "https://example.com", storedURL)
}

func TestSaveShortURL_ReturnsOtherURL_WhenCodeCollides(t *testing.T) {
	repo, mock := newMockRepository(t)
	mock.ExpectExec(insertQuery).WithArgs("https://example.com", "A7L4mBG1").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(getLongURLQuery).WithArgs("A7L4mBG1").
		WillReturnRows(sqlmock.NewRows([]string{"long_url"}).AddRow("https://other.example.com"))

	storedURL, err := repo.SaveShortURL(context.Background(), "https://example.com", "A7L4mBG1")

	require.NoError(t, err)
	assert.Equal(t, "https://other.example.com", storedURL)
}

func TestSaveShortURL_ReturnsInternalError_WhenTakenRowVanished(t *testing.T) {
	repo, mock := newMockRepository(t)
	mock.ExpectExec(insertQuery).WithArgs("https://example.com", "A7L4mBG1").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(getLongURLQuery).WithArgs("A7L4mBG1").
		WillReturnRows(sqlmock.NewRows([]string{"long_url"}))

	storedURL, err := repo.SaveShortURL(context.Background(), "https://example.com", "A7L4mBG1")

	// must not be ErrURLNotFound, or POST /shorten would answer 404
	assert.NotErrorIs(t, err, domain.ErrURLNotFound)
	assert.EqualError(t, err, `short url "A7L4mBG1" was taken but no longer exists`)
	assert.Empty(t, storedURL)
}

func TestSaveShortURL_ReturnsWrappedError_WhenInsertFails(t *testing.T) {
	repo, mock := newMockRepository(t)
	dbErr := errors.New("connection refused")
	mock.ExpectExec(insertQuery).WithArgs("https://example.com", "A7L4mBG1").WillReturnError(dbErr)

	storedURL, err := repo.SaveShortURL(context.Background(), "https://example.com", "A7L4mBG1")

	assert.ErrorIs(t, err, dbErr)
	assert.EqualError(t, err, `inserting short url "A7L4mBG1": connection refused`)
	assert.Empty(t, storedURL)
}

func TestSaveShortURL_ReturnsWrappedError_WhenRowsAffectedFails(t *testing.T) {
	repo, mock := newMockRepository(t)
	resultErr := errors.New("rows affected unavailable")
	mock.ExpectExec(insertQuery).WithArgs("https://example.com", "A7L4mBG1").
		WillReturnResult(sqlmock.NewErrorResult(resultErr))

	storedURL, err := repo.SaveShortURL(context.Background(), "https://example.com", "A7L4mBG1")

	assert.ErrorIs(t, err, resultErr)
	assert.EqualError(t, err, `inserting short url "A7L4mBG1": rows affected unavailable`)
	assert.Empty(t, storedURL)
}

func TestSaveShortURL_ReturnsWrappedError_WhenReadingTakenCodeFails(t *testing.T) {
	repo, mock := newMockRepository(t)
	dbErr := errors.New("connection refused")
	mock.ExpectExec(insertQuery).WithArgs("https://example.com", "A7L4mBG1").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(getLongURLQuery).WithArgs("A7L4mBG1").WillReturnError(dbErr)

	storedURL, err := repo.SaveShortURL(context.Background(), "https://example.com", "A7L4mBG1")

	assert.ErrorIs(t, err, dbErr)
	assert.EqualError(t, err, `reading taken short url: reading long url for "A7L4mBG1": connection refused`)
	assert.Empty(t, storedURL)
}

func TestSaveShortURL_StopsWhenContextIsCancelled(t *testing.T) {
	repo, _ := newMockRepository(t)

	storedURL, err := repo.SaveShortURL(cancelledContext(), "https://example.com", "A7L4mBG1")

	assert.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, storedURL)
}
