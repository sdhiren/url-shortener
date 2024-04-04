package repository

import "context"


type ShortenerRepository struct {
	Cache map[string]string

}
 func NewShortenerRepository() *ShortenerRepository {
	return &ShortenerRepository{}
 }

 func (sr *ShortenerRepository) SaveShotenedURL(context context.Context, url, shortenedURL string) bool{
	if val, ok := sr.Cache[shortenedURL]; !ok {
		sr.Cache[shortenedURL] = val
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