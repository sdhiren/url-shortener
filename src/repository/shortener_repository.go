package repository

import (
	"context"
)

//go:generate mockgen -source=shortener_repository.go -destination=mocks/shortener_repository_mock.go -package=mocks

type ShortenerRepository interface {
	SaveShotenedURL(context context.Context, url, shortenedURL string) bool
	GetOriginalURL(context context.Context, shortenedURL string) string 
}

type shortenerRepository struct {
	Cache map[string]string

}

 func NewShortenerRepository() ShortenerRepository {
	cache := make(map[string]string)
	return &shortenerRepository{Cache: cache}
 }

 func (sr *shortenerRepository) SaveShotenedURL(context context.Context, url, shortenedURL string) bool{
	if _, ok := sr.Cache[shortenedURL]; !ok {
		sr.Cache[shortenedURL] = url
		return true
	}
	return false
 }
 
 func (sr *shortenerRepository) GetOriginalURL(context context.Context, shortenedURL string) string {
	if val, ok := sr.Cache[shortenedURL]; ok {
		return val
	}
	return ""
 }