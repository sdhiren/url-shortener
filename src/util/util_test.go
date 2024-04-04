package util

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
)

func TestGenerateShortURL(t *testing.T) {
	
	testCases := []struct {
		longURL  string
		wantUrl string
	}{
		{"https://www.example.com/1", "xiGD94yi"},
		{"https://www.example.com/2", "A7L4mBG1"},
		{"https://www.example.com/3", "Z9cJpkVn"},
		{"https://www.google.co.in", "PYyy3Gja"},
	}

	for _, test := range testCases {
		expUrl := GenerateShortURL(test.longURL)
		assert.Equal(t, expUrl, test.wantUrl)
	}	 
}