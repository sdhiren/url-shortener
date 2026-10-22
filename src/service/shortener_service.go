package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	model "urlshortener/model/request"
	"urlshortener/src/domain"
	"urlshortener/src/repository"
	"urlshortener/src/util"
)

//go:generate go tool mockgen -source=shortener_service.go -destination=../../mocks/shortener_service_mock.go -package=mocks

type ShortenService interface {
	Shorten(context context.Context, req model.ShortenURLRequest) (string, error)
	// ResolveShortURL records a click on code and returns its long URL, or
	// domain.ErrURLNotFound if the code is malformed or unknown.
	ResolveShortURL(ctx context.Context, code string) (string, error)
	// LookupShortURL returns the long URL for code without counting a click, or
	// domain.ErrURLNotFound if the code is malformed or unknown.
	LookupShortURL(ctx context.Context, code string) (string, error)
}

// maxCodeLength matches the SHORT_URL varchar(20) column.
const maxCodeLength = 20

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

func (ss *shortenService) Shorten(context context.Context, req model.ShortenURLRequest) (string, error) {
	longUrl, err := validateLongURL(req.URL, ss.baseURL)
	if err != nil {
		return "", err
	}

	shortUrl := ss.util.GenerateShortURL(longUrl)

	// check if long url already exists in the db
	dbShortUrl, err := ss.repo.IfURLAlreadyExists(context, longUrl)
	if err != nil {
		return "", fmt.Errorf("checking if url exists: %w", err)
	}
	if dbShortUrl != "" {
		return ss.host + dbShortUrl, nil
	}

	// save url in the database
	if err := ss.repo.SaveShotenedURL(context, longUrl, shortUrl); err != nil {
		return "", fmt.Errorf("saving short url: %w", err)
	}
	return ss.host + shortUrl, nil
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
