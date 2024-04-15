package service

import (
	"context"
	"fmt"
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

	// check if long url already exists in the db
	if db_short_url, err := ss.repo.IfURLAlreadyExists(context, req.URL); db_short_url != "" && err == nil {
		return ss.host + db_short_url
	}else if err != nil {
		fmt.Println("db error: ", err)
		return err.Error()
	}

	// save url in the database
	save_err := ss.repo.SaveShotenedURL(context, req.URL, shortUrl)
	if save_err != nil {
		fmt.Println("error while saving in db: ", save_err)
		return save_err.Error()
	}
	return ss.host + shortUrl
}

func (ss *ShortenService) GetOriginalURL(context context.Context, url string) string {
	long_url, err := ss.repo.GetOriginalURL(context, url)
	if err != nil {
		fmt.Println("db error: ", err)
		return ""
	}
	return long_url
}