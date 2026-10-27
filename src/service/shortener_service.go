package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"urlshortener/model/request"
	"urlshortener/model/response"
	"urlshortener/src/domain"
	"urlshortener/src/logging"
	"urlshortener/src/repository"
	"urlshortener/src/util"
)

//go:generate go tool mockgen -source=shortener_service.go -destination=../../mocks/shortener_service_mock.go -package=mocks

type ShortenService interface {
	// Shorten validates req.URL and returns its short link and code, creating them
	// unless this URL was shortened before. Invalid URLs return a *domain.InvalidURLError.
	Shorten(ctx context.Context, req request.ShortenURLRequest) (response.ShortenURLResponse, error)
	// ResolveShortURL records a click on code and returns its long URL, or
	// domain.ErrURLNotFound if the code is malformed or unknown.
	ResolveShortURL(ctx context.Context, code string) (string, error)
	// LookupShortURL returns the long URL for code without counting a click, or
	// domain.ErrURLNotFound if the code is malformed or unknown.
	LookupShortURL(ctx context.Context, code string) (string, error)
}

const (
	// maxCodeLength matches the SHORT_URL varchar(20) column.
	maxCodeLength = 20
	// maxCodeAttempts bounds retries after hash collisions. With 48-bit codes even one
	// collision is very unlikely, so needing all attempts means something is wrong.
	maxCodeAttempts = 5
)

type shortenService struct {
	host    string
	baseURL *url.URL
	repo    repository.ShortenerRepository
	util    util.Util
}

// NewShortenerService builds the service. host is the public base URL that short
// codes are appended to; it is also used to reject links back to the shortener.
func NewShortenerService(host string, repo repository.ShortenerRepository, util util.Util) ShortenService {
	baseURL, err := url.Parse(host)
	if err != nil {
		baseURL = nil
	}
	return &shortenService{host: host, baseURL: baseURL, repo: repo, util: util}
}

func (ss *shortenService) Shorten(ctx context.Context, req request.ShortenURLRequest) (response.ShortenURLResponse, error) {
	longURL, err := validateLongURL(req.URL, ss.baseURL)
	if err != nil {
		return response.ShortenURLResponse{}, err
	}

	for attempt := range maxCodeAttempts {
		code := ss.util.GenerateShortURL(longURL, attempt)
		// the stored code, not our candidate: a URL shortened before (or concurrently,
		// or after a collision) keeps the code it already has
		storedCode, err := ss.repo.SaveShortURL(ctx, longURL, code)
		if errors.Is(err, repository.ErrShortURLTaken) {
			logging.FromContext(ctx).Warn("short code collides with another url, retrying",
				"code", code, "attempt", attempt+1, "max_attempts", maxCodeAttempts)
			continue
		}
		if err != nil {
			return response.ShortenURLResponse{}, fmt.Errorf("saving short url: %w", err)
		}
		return response.ShortenURLResponse{ShortURL: ss.host + storedCode, Code: storedCode}, nil
	}
	return response.ShortenURLResponse{}, fmt.Errorf("no free short code after %d attempts", maxCodeAttempts)
}

func (ss *shortenService) ResolveShortURL(ctx context.Context, code string) (string, error) {
	return resolve(ctx, code, ss.repo.RecordClick)
}

func (ss *shortenService) LookupShortURL(ctx context.Context, code string) (string, error) {
	return resolve(ctx, code, ss.repo.GetLongURL)
}

func resolve(ctx context.Context, code string, find func(context.Context, string) (string, error)) (string, error) {
	// malformed codes can't exist in the database, so skip the query
	if !isValidCode(code) {
		return "", domain.ErrURLNotFound
	}
	longURL, err := find(ctx, code)
	if errors.Is(err, domain.ErrURLNotFound) {
		return "", err
	}
	if err != nil {
		return "", fmt.Errorf("resolving short url: %w", err)
	}
	return longURL, nil
}

// isValidCode reports whether code could be a generated short code: 1 to
// maxCodeLength characters of the base64url alphabet.
func isValidCode(code string) bool {
	if code == "" || len(code) > maxCodeLength {
		return false
	}
	for _, r := range code {
		if !isLabelChar(r) && r != '_' {
			return false
		}
	}
	return true
}
