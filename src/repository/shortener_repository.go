package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"
	"github.com/lib/pq/pqerror"

	"urlshortener/src/domain"
)

//go:generate go tool mockgen -source=shortener_repository.go -destination=../../mocks/shortener_repository_mock.go -package=mocks

// shortURLConstraint is the unique constraint on SHORT_URL, added in migration 003.
const shortURLConstraint = "urls_short_url_key"

var (
	// ErrShortURLTaken is returned by SaveShortURL when the code already belongs to a
	// different long URL (a hash collision), so the caller should try another code.
	ErrShortURLTaken = errors.New("short url is taken by another url")
	// ErrLongURLHashCollision is returned by SaveShortURL when a different long URL with
	// the same md5 is stored. md5 collisions don't happen by accident, so this means a
	// crafted URL; it's reported instead of handing out the other URL's code.
	ErrLongURLHashCollision = errors.New("a different long url with the same md5 is already stored")
)

type ShortenerRepository interface {
	// SaveShortURL stores longURL under shortenedURL unless longURL is already stored,
	// and returns the code longURL is stored under: shortenedURL if it was inserted now,
	// or the existing code otherwise. It runs a single statement, and returns
	// ErrShortURLTaken if shortenedURL belongs to a different URL.
	SaveShortURL(ctx context.Context, longURL, shortenedURL string) (string, error)
	// RecordClick atomically increments the click count for shortenedURL and returns
	// its long URL, or domain.ErrURLNotFound if the short URL doesn't exist.
	RecordClick(ctx context.Context, shortenedURL string) (string, error)
	// GetLongURL returns the long URL for shortenedURL without counting a click,
	// or domain.ErrURLNotFound if the short URL doesn't exist.
	GetLongURL(ctx context.Context, shortenedURL string) (string, error)
}

type shortenerRepository struct {
	db *sql.DB
}

func NewShortenerRepository(db *sql.DB) ShortenerRepository {
	return &shortenerRepository{db: db}
}

func (sr *shortenerRepository) SaveShortURL(ctx context.Context, longURL, shortenedURL string) (string, error) {
	var storedCode, storedURL string
	// The conflict target is the unique index on md5(LONG_URL) (migration 004), so a URL
	// shortened before, even by a concurrent request, gets its existing row back. The
	// no-op DO UPDATE is there because DO NOTHING returns no row on a conflict.
	err := sr.db.QueryRowContext(ctx,
		"INSERT INTO URLS (LONG_URL, SHORT_URL) VALUES ($1, $2) "+
			"ON CONFLICT ((md5(LONG_URL))) DO UPDATE SET LONG_URL = URLS.LONG_URL "+
			"RETURNING SHORT_URL, LONG_URL",
		longURL, shortenedURL,
	).Scan(&storedCode, &storedURL)
	if pqErr := pq.As(err, pqerror.UniqueViolation); pqErr != nil && pqErr.Constraint == shortURLConstraint {
		// longURL is new, but its code is stored for a different URL
		return "", fmt.Errorf("saving short url %q: %w", shortenedURL, ErrShortURLTaken)
	}
	if err != nil {
		return "", fmt.Errorf("saving short url %q: %w", shortenedURL, err)
	}
	if storedURL != longURL {
		return "", fmt.Errorf("saving short url %q: %w", shortenedURL, ErrLongURLHashCollision)
	}
	return storedCode, nil
}

func (sr *shortenerRepository) RecordClick(ctx context.Context, shortenedURL string) (string, error) {
	var longURL string
	// a single UPDATE ... RETURNING is atomic, so concurrent clicks are never lost
	err := sr.db.QueryRowContext(ctx,
		"UPDATE URLS SET CLICK_COUNT = CLICK_COUNT + 1 WHERE SHORT_URL = $1 RETURNING LONG_URL",
		shortenedURL,
	).Scan(&longURL)
	if errors.Is(err, sql.ErrNoRows) {
		return "", domain.ErrURLNotFound
	}
	if err != nil {
		return "", fmt.Errorf("recording click for %q: %w", shortenedURL, err)
	}
	return longURL, nil
}

func (sr *shortenerRepository) GetLongURL(ctx context.Context, shortenedURL string) (string, error) {
	var longURL string
	err := sr.db.QueryRowContext(ctx, "SELECT LONG_URL FROM URLS WHERE SHORT_URL = $1", shortenedURL).Scan(&longURL)
	if errors.Is(err, sql.ErrNoRows) {
		return "", domain.ErrURLNotFound
	}
	if err != nil {
		return "", fmt.Errorf("reading long url for %q: %w", shortenedURL, err)
	}
	return longURL, nil
}
