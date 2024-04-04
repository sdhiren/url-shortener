package util

import (
	"crypto/sha256"
	"encoding/base64"
)

func GenerateShortURL(longURL string) string {
	// Compute the SHA-256 hash of the long URL
	hash := sha256.Sum256([]byte(longURL))

	// Encode the hash bytes using Base64 encoding
	shortURL := base64.URLEncoding.EncodeToString(hash[:])

	// Take only the first 8 characters of the Base64 encoded string
	return shortURL[:8]
}