package repository

import (
	"context"
	"fmt"
)


type ShortenerRepository struct {
	Cache map[string]string

}
 func NewShortenerRepository() *ShortenerRepository {
	cache := make(map[string]string)
	return &ShortenerRepository{Cache: cache}
 }

 func (sr *ShortenerRepository) SaveShotenedURL(context context.Context, url, shortenedURL string) bool{
	if _, ok := sr.Cache[shortenedURL]; !ok {
		sr.Cache[shortenedURL] = url
		fmt.Println("cache data :", sr.Cache)
		return true
	}
	return false
 }
 
 func (sr *ShortenerRepository) GetOriginalURL(context context.Context, shortenedURL string) string {
	if val, ok := sr.Cache[shortenedURL]; ok {
		return val
	}
	return ""
 }