package util

import (
	"crypto/sha256"
	"encoding/base64"
)

//go:generate mockgen -source=util.go -destination=mocks/util_mock.go -package=mocks

type Util interface {
	GenerateShortURL(longURL string) string
}

type util struct {

}

func NewUtil() Util {
	return &util{}
}

func (u *util) GenerateShortURL(longURL string) string {
	// Compute the SHA-256 hash of the long URL
	hash := sha256.Sum256([]byte(longURL))

	// Encode the hash bytes using Base64 encoding
	shortURL := base64.URLEncoding.EncodeToString(hash[:])

	// Take only the first 8 characters of the Base64 encoded string
	return shortURL[:8]
}