package middlewares

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/jzx17/gofetch/core"
)

// RateLimitExceededError is returned when the rate limit is exceeded
type RateLimitExceededError struct {
	Limit      float64
	RetryAfter time.Duration
}

func (e *RateLimitExceededError) Error() string {
	return fmt.Sprintf("rate limit exceeded: %v requests per second, retry after %v", e.Limit, e.RetryAfter)
}

// RateLimitOptions configures the rate limiting behavior
type RateLimitOptions struct {
	// RequestsPerSecond is the maximum number of requests per second
	RequestsPerSecond float64
	// Burst is the maximum number of requests allowed to exceed the rate
	Burst int
	// WaitOnLimit determines if the middleware should wait when limit is reached
	WaitOnLimit bool
	// MaxWaitTime is the maximum time to wait when limit is reached
	MaxWaitTime time.Duration
}

// DefaultRateLimitOptions returns default rate limit options
func DefaultRateLimitOptions() RateLimitOptions {
	return RateLimitOptions{
		RequestsPerSecond: 10,
		Burst:             1,
		WaitOnLimit:       true,
		MaxWaitTime:       5 * time.Second,
	}
}

// rateLimitMiddleware implements client-side rate limiting
type rateLimitMiddleware struct {
	BaseMiddleware
	options RateLimitOptions

	mu            sync.Mutex
	tokens        float64
	lastTimestamp time.Time
}

// RateLimitMiddleware creates a middleware that implements client-side rate limiting
func RateLimitMiddleware(options RateLimitOptions) ConfigurableMiddleware {
	if options.RequestsPerSecond <= 0 {
		options.RequestsPerSecond = DefaultRateLimitOptions().RequestsPerSecond
	}
	if options.Burst < 0 {
		options.Burst = DefaultRateLimitOptions().Burst
	}
	if options.MaxWaitTime < 0 {
		options.MaxWaitTime = DefaultRateLimitOptions().MaxWaitTime
	}

	mw := &rateLimitMiddleware{
		options:       options,
		tokens:        float64(options.Burst),
		lastTimestamp: time.Now(),
	}

	mw.BaseMiddleware = BaseMiddleware{
		Identifier: MiddlewareIdentifier{
			Name:    "rate-limit",
			Options: options,
		},
		Wrapper: mw.roundTrip,
	}

	return mw
}

func (m *rateLimitMiddleware) roundTrip(next core.RoundTripFunc) core.RoundTripFunc {
	return func(req *http.Request) (*http.Response, error) {
		m.mu.Lock()

		// Update tokens based on time elapsed
		now := time.Now()
		elapsed := now.Sub(m.lastTimestamp).Seconds()
		m.lastTimestamp = now

		// Add tokens for time elapsed (up to burst limit)
		m.tokens += elapsed * m.options.RequestsPerSecond
		maxTokens := float64(m.options.Burst)
		if maxTokens < 1 {
			maxTokens = 1
		}
		if m.tokens > maxTokens {
			m.tokens = maxTokens
		}

		// Check if we have enough tokens
		if m.tokens < 1.0 {
			// Calculate wait time to get a token
			waitTime := time.Duration((1.0 - m.tokens) * float64(time.Second) / m.options.RequestsPerSecond)

			if !m.options.WaitOnLimit || waitTime > m.options.MaxWaitTime {
				// Return error if we're not waiting or wait time exceeds max
				m.mu.Unlock()
				return nil, &RateLimitExceededError{
					Limit:      m.options.RequestsPerSecond,
					RetryAfter: waitTime,
				}
			}

			// Wait for token to be available
			timer := time.NewTimer(waitTime)
			defer timer.Stop()

			// Release lock while waiting
			m.mu.Unlock()
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-timer.C:
				// Reacquire lock after waiting
				m.mu.Lock()
			}

			// Update timestamp and token count after waiting
			m.lastTimestamp = time.Now()
			m.tokens = 0
		}

		// Consume token
		m.tokens--
		m.mu.Unlock()

		// Execute the request
		return next(req)
	}
}

// WithRequestsPerSecond sets the maximum number of requests per second
func WithRequestsPerSecond(rps float64) func(*RateLimitOptions) {
	return func(o *RateLimitOptions) {
		o.RequestsPerSecond = rps
	}
}

// WithBurst sets the maximum burst size
func WithBurst(burst int) func(*RateLimitOptions) {
	return func(o *RateLimitOptions) {
		o.Burst = burst
	}
}

// WithWaitOnLimit configures whether to wait when limit is exceeded
func WithWaitOnLimit(wait bool) func(*RateLimitOptions) {
	return func(o *RateLimitOptions) {
		o.WaitOnLimit = wait
	}
}

// WithMaxWaitTime sets the maximum time to wait when limit is exceeded
func WithMaxWaitTime(duration time.Duration) func(*RateLimitOptions) {
	return func(o *RateLimitOptions) {
		o.MaxWaitTime = duration
	}
}

// NewRateLimitMiddleware creates a rate limit middleware with custom options
func NewRateLimitMiddleware(optFuncs ...func(*RateLimitOptions)) ConfigurableMiddleware {
	options := DefaultRateLimitOptions()
	for _, fn := range optFuncs {
		fn(&options)
	}
	return RateLimitMiddleware(options)
}
