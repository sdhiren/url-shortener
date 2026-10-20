package domain

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInvalidURLError_Error(t *testing.T) {
	err := &InvalidURLError{Reason: "url is required"}

	assert.Equal(t, "invalid url: url is required", err.Error())
}

func TestInvalidURLError_CanBeUnwrapped(t *testing.T) {
	err := fmt.Errorf("shortening: %w", &InvalidURLError{Reason: "url is malformed"})

	var invalidURLErr *InvalidURLError
	assert.True(t, errors.As(err, &invalidURLErr))
	assert.Equal(t, "url is malformed", invalidURLErr.Reason)
	assert.False(t, errors.Is(err, ErrURLNotFound))
}

func TestErrURLNotFound_Message(t *testing.T) {
	assert.EqualError(t, ErrURLNotFound, "short url not found")
}
