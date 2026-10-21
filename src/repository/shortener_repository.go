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
	SaveShotenedURL(ctx context.Context, url, shortenedURL string) error
	// RecordClick atomically increments the click count for shortenedURL and returns
	// its long URL, or domain.ErrURLNotFound if the short URL doesn't exist.
	RecordClick(ctx context.Context, shortenedURL string) (string, error)
	// GetLongURL returns the long URL for shortenedURL without counting a click,
	// or domain.ErrURLNotFound if the short URL doesn't exist.
	GetLongURL(ctx context.Context, shortenedURL string) (string, error)
	IfURLAlreadyExists(ctx context.Context, longURL string) (string, error)
}

type shortenerRepository struct {
	db *sql.DB
}

func NewShortenerRepository(db *sql.DB) ShortenerRepository {
	return &shortenerRepository{db: db}
}

func (sr *shortenerRepository) SaveShotenedURL(ctx context.Context, url, shortenedURL string) error {
	_, err := sr.db.ExecContext(ctx, "INSERT INTO URLS (LONG_URL, SHORT_URL) VALUES ($1, $2)", url, shortenedURL)
	if err != nil {
		return fmt.Errorf("inserting short url %q: %w", shortenedURL, err)
	}
	return nil
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

// IfURLAlreadyExists returns the short URL already stored for longURL, or "" if there is none.
func (sr *shortenerRepository) IfURLAlreadyExists(ctx context.Context, longURL string) (string, error) {
	var shortURL string
	err := sr.db.QueryRowContext(ctx, "SELECT SHORT_URL FROM URLS WHERE LONG_URL = $1", longURL).Scan(&shortURL)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("looking up existing short url: %w", err)
	}
	return shortURL, nil
}
