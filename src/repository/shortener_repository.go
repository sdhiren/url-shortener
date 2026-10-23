package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"urlshortener/src/domain"
)

//go:generate go tool mockgen -source=shortener_repository.go -destination=../../mocks/shortener_repository_mock.go -package=mocks

type ShortenerRepository interface {
	// SaveShortURL stores shortenedURL -> longURL unless shortenedURL is already
	// taken, and returns the long URL stored for shortenedURL afterwards: longURL
	// itself if it was inserted now or earlier, or a different URL on a collision.
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
	// ON CONFLICT makes concurrent requests for the same code safe: exactly one insert
	// wins, and the others fall through to reading what was stored
	result, err := sr.db.ExecContext(ctx,
		"INSERT INTO URLS (LONG_URL, SHORT_URL) VALUES ($1, $2) ON CONFLICT (SHORT_URL) DO NOTHING",
		longURL, shortenedURL,
	)
	if err != nil {
		return "", fmt.Errorf("inserting short url %q: %w", shortenedURL, err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return "", fmt.Errorf("inserting short url %q: %w", shortenedURL, err)
	}
	if inserted == 1 {
		return longURL, nil
	}

	// a separate statement gets a fresh snapshot, so it sees the row that won the conflict
	storedURL, err := sr.GetLongURL(ctx, shortenedURL)
	if errors.Is(err, domain.ErrURLNotFound) {
		// not a 404 for the caller: the row existed a moment ago and was removed concurrently
		return "", fmt.Errorf("short url %q was taken but no longer exists", shortenedURL)
	}
	if err != nil {
		return "", fmt.Errorf("reading taken short url: %w", err)
	}
	return storedURL, nil
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
