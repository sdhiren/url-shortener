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
	GetOriginalURL(context context.Context, url string) (string, error)
}

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

// GetOriginalURL returns domain.ErrURLNotFound when the short code is unknown.
func (ss *shortenService) GetOriginalURL(context context.Context, url string) (string, error) {
	longUrl, err := ss.repo.GetOriginalURL(context, url)
	if errors.Is(err, domain.ErrURLNotFound) {
		return "", err
	}
	if err != nil {
		return "", fmt.Errorf("getting original url: %w", err)
	}
	return longUrl, nil
}
