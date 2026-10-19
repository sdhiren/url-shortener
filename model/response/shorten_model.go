package model

type ShortenURLResponse struct {
	ShortURL string `json:"short_url"`
}

type OriginalURLResponse struct {
	LongURL string `json:"long_url"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
