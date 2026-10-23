package util

import (
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
		expURL := util.GenerateShortURL(test.longURL)
		assert.Equal(t, expURL, test.wantURL)
	}
}
