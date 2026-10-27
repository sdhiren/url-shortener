package util

import (
	"crypto/sha256"
	"encoding/base64"
	"strconv"
)

//go:generate go tool mockgen -source=util.go -destination=../../mocks/util_mock.go -package=mocks

type Util interface {
	// GenerateShortURL returns the 8-character code for longURL at the given attempt.
	// Attempt 0 is the unsalted hash, so existing codes never change; later attempts
	// salt the hash to get a different code after a collision. The same URL and
	// attempt always give the same code.
	GenerateShortURL(longURL string, attempt int) string
}

type util struct {
}

func NewUtil() Util {
	return &util{}
}

func (u *util) GenerateShortURL(longURL string, attempt int) string {
	// Compute the SHA-256 hash of the (salted) long URL
	hash := sha256.Sum256([]byte(saltedURL(longURL, attempt)))

	// Encode the hash bytes using Base64 encoding
	shortURL := base64.URLEncoding.EncodeToString(hash[:])

	// Take only the first 8 characters of the Base64 encoded string
	return shortURL[:8]
}

// saltedURL returns the hash input for an attempt: the URL itself for attempt 0, and
// the URL plus a NUL-separated attempt number after that.
func saltedURL(longURL string, attempt int) string {
	if attempt == 0 {
		return longURL
	}
	return longURL + "\x00" + strconv.Itoa(attempt)
}
