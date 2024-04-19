package service

import (
	"context"
	"fmt"
	model "urlshortener/model/request"
	"urlshortener/src/repository"
	"urlshortener/src/util"
)

//go:generate mockgen -source=shortener_service.go -destination=mocks/shortener_service_mock.go -package=mocks

type ShortenService interface {
	Shorten(context context.Context, req model.ShortenURLRequest) string
	GetOriginalURL(context context.Context, url string) string 
}

type shortenService struct {
	host string
	repo repository.ShortenerRepository
	util util.Util
}

func NewShortenerService(host string, repo repository.ShortenerRepository, util util.Util) ShortenService {
	return &shortenService{host: host, repo: repo, util: util}
}

func (ss *shortenService) Shorten(context context.Context, req model.ShortenURLRequest) string {
	shortUrl :=  ss.util.GenerateShortURL(req.URL)

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

func (ss *shortenService) GetOriginalURL(context context.Context, url string) string {
	long_url, err := ss.repo.GetOriginalURL(context, url)
	if err != nil {
		fmt.Println("db error: ", err)
		return ""
	}
	return long_url
}