package repository

import (
	"context"
	"testing"
)

func Test_shortenerRepository_SaveShotenedURL(t *testing.T) {
	tests := []struct {
		name string
		cache map[string]string
		url   string
		shortenedUrl string
		want   bool
	}{
		{"key does not exist", map[string]string{"key1": "value1"}, "www.test.com", "WzadtdDb",  true},
		{"key exists", map[string]string{"Rgdt4Gt5": "www.test2.com"}, "www.test2.com", "Rgdt4Gt5",  false},
	}

	context := context.TODO()


	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := &shortenerRepository{
				Cache: tt.cache,
			}
			if got := sr.SaveShotenedURL(context, tt.url, tt.shortenedUrl); got != tt.want {
				t.Errorf("shortenerRepository.SaveShotenedURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_shortenerRepository_GetOriginalURL(t *testing.T) {
	context := context.TODO()
	tests := []struct {
		name string
		cache map[string]string
		shortenedUrl string
		want   string
	}{
		{"key does not exist", map[string]string{"key1": "value1"},  "WzadtdDb",  ""},
		{"key exists", map[string]string{"Rgdt4Gt5": "www.test2.com"}, "Rgdt4Gt5",  "www.test2.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := &shortenerRepository{
				Cache: tt.cache,
			}
			if got := sr.GetOriginalURL(context, tt.shortenedUrl); got != tt.want {
				t.Errorf("shortenerRepository.GetOriginalURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
