package repository

import (
	"context"
	"database/sql"
	"fmt"
)

type URL struct {
	long_url string `db:"LONG_URL"`
	short_url string `db:"SHORT_URL"`
}

//go:generate mockgen -source=shortener_repository.go -destination=mocks/shortener_repository_mock.go -package=mocks

type ShortenerRepository interface {
	SaveShotenedURL(context context.Context, url, shortenedURL string) error
	GetOriginalURL(context context.Context, shortenedURL string) (string, error)
	IfURLAlreadyExists(context context.Context, long_url string) (string, error)
}

type shortenerRepository struct {
	Cache map[string]string
	db *sql.DB

}

 func NewShortenerRepository(db *sql.DB) ShortenerRepository {
	cache := make(map[string]string)
	return &shortenerRepository{Cache: cache, db: db}
 }

 func (sr *shortenerRepository) SaveShotenedURL(context context.Context, url, shortenedURL string) error{

	// prepare insert statement
	stmt, err := sr.db.Prepare("INSERT INTO URLS (LONG_URL, SHORT_URL, COUNTER_VALUE) VALUES ($1, $2, $3)")
	if err != nil {
		fmt.Println("Error preparing insert statement:", err)
		return err
	}

	defer stmt.Close()

	// Execute the insert statement
	_, err = stmt.Exec(url, shortenedURL, 1)
	if err != nil {
		fmt.Println("Error executing insert statement:", err)
		return err
	}

	return nil
 }
 
 func (sr *shortenerRepository) GetOriginalURL(context context.Context, shortenedURL string) (string, error) {

	url := URL{}
	result := sr.db.QueryRow("SELECT LONG_URL FROM URLS WHERE SHORT_URL = $1", shortenedURL)

	err := result.Scan(&url.long_url)
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("no record present")
			return "", err
		}else {
			fmt.Println("error reading the response")
			return "", err
		}
	}

	return url.long_url, nil
 }
 
 func (sr *shortenerRepository) IfURLAlreadyExists(context context.Context, long_url string) (string, error) {

	url := URL{}
	result := sr.db.QueryRow("SELECT SHORT_URL FROM URLS WHERE LONG_URL = $1", long_url)
	
	err := result.Scan(&url.short_url)
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("no record present")
			return  "", nil
		}else {
			fmt.Println("error reading the response")
			return "", err
		}
	}

	return url.short_url, nil
 }