package middlewares

import (
	"errors"
	"net"
	"net/http"
	"time"
)

// RetryStrategy defines how retry attempts are spaced
type RetryStrategy interface {
	// NextDelay returns the delay to wait for the next retry attempt
	NextDelay(attempt int, resp *http.Response, err error) time.Duration

	// ShouldRetry determines whether a retry should be attempted
	ShouldRetry(attempt int, resp *http.Response, err error) bool
}

// ConstantDelayStrategy implements a constant delay between retries
type ConstantDelayStrategy struct {
	Delay             time.Duration
	MaxAttempts       int
	RetryableStatuses []int
}

// NewConstantDelayStrategy creates a retry strategy with constant delay
func NewConstantDelayStrategy(delay time.Duration, maxAttempts int) *ConstantDelayStrategy {
	return &ConstantDelayStrategy{
		Delay:             delay,
		MaxAttempts:       maxAttempts,
		RetryableStatuses: RetryableStatusCodes(),
	}
}

// NextDelay returns a constant delay regardless of attempt, response or error
func (s *ConstantDelayStrategy) NextDelay(_ int, _ *http.Response, _ error) time.Duration {
	return s.Delay
}

func (s *ConstantDelayStrategy) ShouldRetry(attempt int, resp *http.Response, err error) bool {
	// Don't retry if we've reached max attempts
	if attempt >= s.MaxAttempts {
		return false
	}

	// Retry on network errors
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) {
			return true
		}
		return false
	}

	// Retry on certain status codes
	if resp != nil {
		for _, status := range s.RetryableStatuses {
			if resp.StatusCode == status {
				return true
			}
		}
	}

	return false
}

// ExponentialBackoffStrategy implements exponential backoff
type ExponentialBackoffStrategy struct {
	InitialDelay      time.Duration
	MaxDelay          time.Duration
	Factor            float64
	MaxAttempts       int
	RetryableStatuses []int
}

// NewExponentialBackoffStrategy creates a retry strategy with exponential backoff
func NewExponentialBackoffStrategy(initialDelay, maxDelay time.Duration, factor float64, maxAttempts int) *ExponentialBackoffStrategy {
	return &ExponentialBackoffStrategy{
		InitialDelay:      initialDelay,
		MaxDelay:          maxDelay,
		Factor:            factor,
		MaxAttempts:       maxAttempts,
		RetryableStatuses: RetryableStatusCodes(),
	}
}

// NextDelay calculates the next delay using exponential backoff
func (s *ExponentialBackoffStrategy) NextDelay(attempt int, _ *http.Response, _ error) time.Duration {
	// Calculate exponential delay
	delay := s.InitialDelay * time.Duration(float64(int64(1)<<uint(attempt))*s.Factor)

	// Cap at max delay
	if delay > s.MaxDelay {
		delay = s.MaxDelay
	}

	return delay
}

func (s *ExponentialBackoffStrategy) ShouldRetry(attempt int, resp *http.Response, err error) bool {
	// Don't retry if we've reached max attempts
	if attempt >= s.MaxAttempts {
		return false
	}

	// Retry on network errors
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) {
			return true
		}
		return false
	}

	// Retry on certain status codes
	if resp != nil {
		for _, status := range s.RetryableStatuses {
			if resp.StatusCode == status {
				return true
			}
		}

		// Special handling for 429 (Too Many Requests)
		if resp.StatusCode == 429 {
			// Check for Retry-After header
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				return true
			}
		}
	}

	return false
}

// RetryableStatusCodes returns the default list of status codes to retry
func RetryableStatusCodes() []int {
	return []int{408, 429, 500, 502, 503, 504}
}

// WithRetryableStatuses adds additional status codes to retry on
// It returns the modified strategy for fluent chaining
func WithRetryableStatuses(strategy RetryStrategy, codes ...int) RetryStrategy {
	switch s := strategy.(type) {
	case *ConstantDelayStrategy:
		s.RetryableStatuses = append(s.RetryableStatuses, codes...)
		return s
	case *ExponentialBackoffStrategy:
		s.RetryableStatuses = append(s.RetryableStatuses, codes...)
		return s
	default:
		return strategy
	}
}
