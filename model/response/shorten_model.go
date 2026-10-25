package response

type ShortenURLResponse struct {
	ShortURL string `json:"short_url"`
	Code     string `json:"code"`
}

type LookupResponse struct {
	Code    string `json:"code"`
	LongURL string `json:"long_url"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
