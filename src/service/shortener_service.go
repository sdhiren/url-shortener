package service

import (
	"context"
	model "urlshortener/model/request"
	"urlshortener/src/repository"
	"urlshortener/src/util"
)

type ShortenService struct {
	host string
	repo repository.ShortenerRepository
}

func NewShortenerService(host string, repo repository.ShortenerRepository) *ShortenService {
	return &ShortenService{host: host, repo: repo}
}

func (ss *ShortenService) Shorten(context context.Context, req model.ShortenURLRequest) string {
	shortUrl :=  util.GenerateShortURL(req.URL)
	ss.repo.SaveShotenedURL(context, req.URL, shortUrl)
	return ss.host + shortUrl
}

func (ss *ShortenService) GetOriginalURL(context context.Context, url string) string {
	return ss.repo.GetOriginalURL(context, url)
}