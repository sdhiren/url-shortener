package repository

import (
	"context"
	"errors"
	"testing"

	"urlshortener/src/domain"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/lib/pq/pqerror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	saveQuery = "INSERT INTO URLS (LONG_URL, SHORT_URL) VALUES ($1, $2) " +
		"ON CONFLICT ((md5(LONG_URL))) DO UPDATE SET LONG_URL = URLS.LONG_URL " +
		"RETURNING SHORT_URL, LONG_URL"
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

func expectSave(mock sqlmock.Sqlmock, longURL, code string) *sqlmock.ExpectedQuery {
	return mock.ExpectQuery(saveQuery).WithArgs(longURL, code)
}

func savedRow(code, longURL string) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"short_url", "long_url"}).AddRow(code, longURL)
}

func TestSaveShortURL_RunsOneStatementAndReturnsCode_WhenInserted(t *testing.T) {
	// no ExpectPrepare and no second query: sqlmock fails the test on any unexpected call,
	// and ExpectationsWereMet (in newMockRepository) fails it if the upsert didn't run
	repo, mock := newMockRepository(t)
	expectSave(mock, "https://example.com", "A7L4mBG1").WillReturnRows(savedRow("A7L4mBG1", "https://example.com"))

	storedCode, err := repo.SaveShortURL(context.Background(), "https://example.com", "A7L4mBG1")

	require.NoError(t, err)
	assert.Equal(t, "A7L4mBG1", storedCode)
}

func TestSaveShortURL_ReturnsStoredCode_WhenURLWasShortenedBefore(t *testing.T) {
	// e.g. the URL got a salted code after a collision: its existing code wins over the candidate
	repo, mock := newMockRepository(t)
	expectSave(mock, "https://example.com", "A7L4mBG1").WillReturnRows(savedRow("Salted01", "https://example.com"))

	storedCode, err := repo.SaveShortURL(context.Background(), "https://example.com", "A7L4mBG1")

	require.NoError(t, err)
	assert.Equal(t, "Salted01", storedCode)
}

func TestSaveShortURL_ReturnsErrShortURLTaken_WhenCodeBelongsToAnotherURL(t *testing.T) {
	repo, mock := newMockRepository(t)
	expectSave(mock, "https://example.com", "A7L4mBG1").
		WillReturnError(&pq.Error{Code: pqerror.UniqueViolation, Constraint: "urls_short_url_key"})

	storedCode, err := repo.SaveShortURL(context.Background(), "https://example.com", "A7L4mBG1")

	assert.ErrorIs(t, err, ErrShortURLTaken)
	assert.EqualError(t, err, `saving short url "A7L4mBG1": short url is taken by another url`)
	assert.Empty(t, storedCode)
}

func TestSaveShortURL_ReturnsErrorUnchanged_WhenUniqueViolationIsOnAnotherConstraint(t *testing.T) {
	repo, mock := newMockRepository(t)
	dbErr := &pq.Error{Code: pqerror.UniqueViolation, Constraint: "urls_pkey"}
	expectSave(mock, "https://example.com", "A7L4mBG1").WillReturnError(dbErr)

	storedCode, err := repo.SaveShortURL(context.Background(), "https://example.com", "A7L4mBG1")

	assert.ErrorIs(t, err, dbErr)
	assert.NotErrorIs(t, err, ErrShortURLTaken)
	assert.Empty(t, storedCode)
}

func TestSaveShortURL_ReturnsErrorUnchanged_WhenOtherPostgresErrorNamesShortURLConstraint(t *testing.T) {
	// only a unique violation means the code is taken
	repo, mock := newMockRepository(t)
	dbErr := &pq.Error{Code: pqerror.TRSerializationFailure, Constraint: "urls_short_url_key"}
	expectSave(mock, "https://example.com", "A7L4mBG1").WillReturnError(dbErr)

	_, err := repo.SaveShortURL(context.Background(), "https://example.com", "A7L4mBG1")

	assert.ErrorIs(t, err, dbErr)
	assert.NotErrorIs(t, err, ErrShortURLTaken)
}

func TestSaveShortURL_ReturnsWrappedError_WhenQueryFails(t *testing.T) {
	repo, mock := newMockRepository(t)
	dbErr := errors.New("connection refused")
	expectSave(mock, "https://example.com", "A7L4mBG1").WillReturnError(dbErr)

	storedCode, err := repo.SaveShortURL(context.Background(), "https://example.com", "A7L4mBG1")

	assert.ErrorIs(t, err, dbErr)
	assert.NotErrorIs(t, err, ErrShortURLTaken)
	assert.EqualError(t, err, `saving short url "A7L4mBG1": connection refused`)
	assert.Empty(t, storedCode)
}

func TestSaveShortURL_ReturnsErrLongURLHashCollision_WhenStoredURLDiffers(t *testing.T) {
	// a different URL with the same md5 owns the row; its code must not be handed out
	repo, mock := newMockRepository(t)
	expectSave(mock, "https://example.com", "A7L4mBG1").WillReturnRows(savedRow("Other001", "https://other.example.com"))

	storedCode, err := repo.SaveShortURL(context.Background(), "https://example.com", "A7L4mBG1")

	assert.ErrorIs(t, err, ErrLongURLHashCollision)
	assert.NotErrorIs(t, err, ErrShortURLTaken)
	assert.Empty(t, storedCode)
}

func TestSaveShortURL_ReturnsWrappedError_WhenScanFails(t *testing.T) {
	repo, mock := newMockRepository(t)
	expectSave(mock, "https://example.com", "A7L4mBG1").
		WillReturnRows(sqlmock.NewRows([]string{"short_url", "long_url"}).AddRow(nil, "https://example.com"))

	storedCode, err := repo.SaveShortURL(context.Background(), "https://example.com", "A7L4mBG1")

	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrShortURLTaken)
	assert.Empty(t, storedCode)
}

func TestSaveShortURL_StopsWhenContextIsCancelled(t *testing.T) {
	repo, _ := newMockRepository(t)

	storedCode, err := repo.SaveShortURL(cancelledContext(), "https://example.com", "A7L4mBG1")

	assert.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, storedCode)
}
