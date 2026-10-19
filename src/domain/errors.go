package domain

import "errors"

// ErrURLNotFound is returned when no long URL is stored for a short code.
var ErrURLNotFound = errors.New("short url not found")
