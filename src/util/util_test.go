package util

import (
	"regexp"
	"testing"

	assert "github.com/stretchr/testify/assert"
)

func TestGenerateShortURL(t *testing.T) {

	testCases := []struct {
		longURL string
		wantURL string
	}{
		{"https://www.example.com/1", "xiGD94yi"},
		{"https://www.example.com/2", "A7L4mBG1"},
		{"https://www.example.com/3", "Z9cJpkVn"},
		{"https://www.google.co.in", "PYyy3Gja"},
	}

	util := NewUtil()

	for _, test := range testCases {
		expURL := util.GenerateShortURL(test.longURL, 0)
		assert.Equal(t, expURL, test.wantURL)
	}
}

func TestGenerateShortURL_SaltedAttemptsGiveStableDifferentCodes(t *testing.T) {
	util := NewUtil()
	codeChars := regexp.MustCompile(`^[A-Za-z0-9_-]{8}$`)

	// pinned so a change to the salting can't silently re-code stored links
	assert.Equal(t, "zVnUWGMf", util.GenerateShortURL("https://www.example.com/2", 1))
	assert.Equal(t, "KVLoLHOQ", util.GenerateShortURL("https://www.example.com/2", 4))

	seen := map[string]bool{}
	for attempt := range 5 {
		code := util.GenerateShortURL("https://www.example.com/2", attempt)
		assert.Regexp(t, codeChars, code)
		assert.Equal(t, code, util.GenerateShortURL("https://www.example.com/2", attempt))
		seen[code] = true
	}
	assert.Len(t, seen, 5)
}

func TestSaltedURL(t *testing.T) {
	assert.Equal(t, "https://example.com", saltedURL("https://example.com", 0))
	assert.Equal(t, "https://example.com\x001", saltedURL("https://example.com", 1))
	assert.Equal(t, "https://example.com\x004", saltedURL("https://example.com", 4))
}
