package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"urlshortener/src/domain"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var getOriginalURLQuery = regexp.QuoteMeta("SELECT LONG_URL FROM URLS WHERE SHORT_URL = $1")

func newMockRepository(t *testing.T) (ShortenerRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		db.Close()
	})
	return NewShortenerRepository(db), mock
}

func TestGetOriginalURL_ReturnsLongURL_WhenShortURLExists(t *testing.T) {
	repo, mock := newMockRepository(t)
	mock.ExpectQuery(getOriginalURLQuery).WithArgs("A7L4mBG1").
		WillReturnRows(sqlmock.NewRows([]string{"long_url"}).AddRow("https://example.com"))

	longURL, err := repo.GetOriginalURL(context.Background(), "A7L4mBG1")

	require.NoError(t, err)
	assert.Equal(t, "https://example.com", longURL)
}

func TestGetOriginalURL_ReturnsErrURLNotFound_WhenNoRows(t *testing.T) {
	repo, mock := newMockRepository(t)
	mock.ExpectQuery(getOriginalURLQuery).WithArgs("unknown1").
		WillReturnRows(sqlmock.NewRows([]string{"long_url"}))

	longURL, err := repo.GetOriginalURL(context.Background(), "unknown1")

	assert.ErrorIs(t, err, domain.ErrURLNotFound)
	assert.Empty(t, longURL)
}

func TestGetOriginalURL_ReturnsWrappedError_WhenQueryFails(t *testing.T) {
	repo, mock := newMockRepository(t)
	dbErr := errors.New("connection refused")
	mock.ExpectQuery(getOriginalURLQuery).WithArgs("A7L4mBG1").WillReturnError(dbErr)

	longURL, err := repo.GetOriginalURL(context.Background(), "A7L4mBG1")

	assert.ErrorIs(t, err, dbErr)
	assert.NotErrorIs(t, err, domain.ErrURLNotFound)
	assert.EqualError(t, err, `reading long url for "A7L4mBG1": connection refused`)
	assert.Empty(t, longURL)
}

func TestGetOriginalURL_ReturnsWrappedError_WhenScanFails(t *testing.T) {
	repo, mock := newMockRepository(t)
	mock.ExpectQuery(getOriginalURLQuery).WithArgs("A7L4mBG1").
		WillReturnRows(sqlmock.NewRows([]string{"long_url"}).AddRow(nil))

	longURL, err := repo.GetOriginalURL(context.Background(), "A7L4mBG1")

	require.Error(t, err)
	assert.NotErrorIs(t, err, domain.ErrURLNotFound)
	assert.Empty(t, longURL)
}
