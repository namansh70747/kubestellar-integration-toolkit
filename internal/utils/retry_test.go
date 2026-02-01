package utils

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetry(t *testing.T) {
	attempts := 0
	err := Retry(3, time.Millisecond, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary error")
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 3, attempts)
}
