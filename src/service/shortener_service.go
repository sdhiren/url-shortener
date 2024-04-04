package service

import (
	"context"
	model "urlshortener/model/request"
	"urlshortener/src/util"
)

type ShortenService struct {
	host string
}

func NewShortenerService(host string) *ShortenService {
	return &ShortenService{host: host}
}

func (ss *ShortenService) Shorten(context context.Context, req model.ShortenURLRequest) string {
	return ss.host + util.GenerateShortURL(req.URL)
}