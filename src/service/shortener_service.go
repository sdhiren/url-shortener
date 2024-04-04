package service

import (
	"context"
	model "urlshortener/model/request"
)

type ShortenService struct {

}

func NewShortenerService() *ShortenService {
	return &ShortenService{}
}

func (ss *ShortenService) Shorten(context context.Context, req model.ShortenURLRequest) string {
	return "shorten-url"
}