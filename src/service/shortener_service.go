package service

import (
	"context"
	"errors"
	"fmt"
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
	host string
	repo repository.ShortenerRepository
	util util.Util
}

func NewShortenerService(host string, repo repository.ShortenerRepository, util util.Util) ShortenService {
	return &shortenService{host: host, repo: repo, util: util}
}

func (ss *shortenService) Shorten(context context.Context, req model.ShortenURLRequest) (string, error) {
	shortUrl := ss.util.GenerateShortURL(req.URL)

	// check if long url already exists in the db
	dbShortUrl, err := ss.repo.IfURLAlreadyExists(context, req.URL)
	if err != nil {
		return "", fmt.Errorf("checking if url exists: %w", err)
	}
	if dbShortUrl != "" {
		return ss.host + dbShortUrl, nil
	}

	// save url in the database
	if err := ss.repo.SaveShotenedURL(context, req.URL, shortUrl); err != nil {
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
