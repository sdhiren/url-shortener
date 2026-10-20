package domain

import "errors"

// ErrURLNotFound is returned when no long URL is stored for a short code.
var ErrURLNotFound = errors.New("short url not found")

// InvalidURLError is returned when a URL submitted for shortening is rejected.
// Reason is safe to show to API clients.
type InvalidURLError struct {
	Reason string
}

func (e *InvalidURLError) Error() string {
	return "invalid url: " + e.Reason
}
