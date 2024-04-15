package repository

import (
	"context"
	"database/sql"
)

//go:generate mockgen -source=shortener_repository.go -destination=mocks/shortener_repository_mock.go -package=mocks

type ShortenerRepository interface {
	SaveShotenedURL(context context.Context, url, shortenedURL string) bool
	GetOriginalURL(context context.Context, shortenedURL string) string 
}

type shortenerRepository struct {
	Cache map[string]string
	db *sql.DB

}

 func NewShortenerRepository(db *sql.DB) ShortenerRepository {
	cache := make(map[string]string)
	return &shortenerRepository{Cache: cache, db: db}
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