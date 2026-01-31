package utils

import (
	"context"
	"fmt"
	"math"
	"time"
)

type RetryConfig struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	RetryableErrors []error
}

func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
	}
}

func Retry(attempts int, sleep time.Duration, fn func() error) error {
	var lastErr error
	for i := 0; i < attempts; i++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(sleep)
	}
	return fmt.Errorf("after %d attempts, last error: %v", attempts, lastErr)
}

func RetryWithContext(ctx context.Context, attempts int, sleep time.Duration, fn func() error) error {
	var lastErr error
	for i := 0; i < attempts; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := fn(); err == nil {
				return nil
			} else {
				lastErr = err
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(sleep):
			}
		}
	}
	return fmt.Errorf("after %d attempts, last error: %v", attempts, lastErr)
}

func RetryWithBackoff(ctx context.Context, config *RetryConfig, fn func() error) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := fn(); err == nil {
				return nil
			} else {
				lastErr = err
			}

			if attempt < config.MaxAttempts-1 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(delay):
				}

				delay = time.Duration(float64(delay) * config.BackoffFactor)
				if delay > config.MaxDelay {
					delay = config.MaxDelay
				}
			}
		}
	}

	return fmt.Errorf("after %d attempts, last error: %v", config.MaxAttempts, lastErr)
}

func RetryWithExponentialBackoff(ctx context.Context, maxAttempts int, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := fn(); err == nil {
				return nil
			} else {
				lastErr = err
			}

			if attempt < maxAttempts-1 {
				delay := time.Duration(math.Pow(2, float64(attempt))) * time.Second
				if delay > 60*time.Second {
					delay = 60 * time.Second
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(delay):
				}
			}
		}
	}

	return fmt.Errorf("after %d attempts, last error: %v", maxAttempts, lastErr)
}

type RetryableFunc func(ctx context.Context) (interface{}, error)

func RetryWithResult(ctx context.Context, config *RetryConfig, fn RetryableFunc) (interface{}, error) {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			result, err := fn(ctx)
			if err == nil {
				return result, nil
			}
			lastErr = err

			if attempt < config.MaxAttempts-1 {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(delay):
				}

				delay = time.Duration(float64(delay) * config.BackoffFactor)
				if delay > config.MaxDelay {
					delay = config.MaxDelay
				}
			}
		}
	}

	return nil, fmt.Errorf("after %d attempts, last error: %v", config.MaxAttempts, lastErr)
}

func IsRetryable(err error, retryableErrors []error) bool {
	if len(retryableErrors) == 0 {
		return true
	}

	for _, retryableErr := range retryableErrors {
		if err == retryableErr {
			return true
		}
	}

	return false
}
