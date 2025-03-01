package middlewares

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/jzx17/gofetch/core"
)

var _ ConfigurableMiddleware = (*retryMiddleware)(nil)

type RetryError struct {
	Attempts int
	LastErr  error
}

func (e *RetryError) Error() string {
	return fmt.Sprintf("request failed after %d attempts: %v", e.Attempts, e.LastErr)
}

func (e *RetryError) Unwrap() error {
	return e.LastErr
}

type TimeoutError struct {
	Err error
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("request timed out: %v", e.Err)
}

func (e *TimeoutError) Unwrap() error {
	return e.Err
}

var bodyPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 4096))
	},
}

// RetryMiddleware returns a middleware that retries a request based on the given RetryStrategy
type retryMiddleware struct {
	BaseMiddleware
	strategy RetryStrategy
}

// RetryMiddleware returns a middleware that retries a request according to the provided strategy
func RetryMiddleware(strategy RetryStrategy) ConfigurableMiddleware {
	mw := &retryMiddleware{
		strategy: strategy,
	}

	// Initialize the embedded BaseMiddleware fields.
	mw.BaseMiddleware = BaseMiddleware{
		Identifier: MiddlewareIdentifier{
			Name:    "retry middleware",
			Options: strategy,
		},
		Wrapper: mw.roundTrip,
	}
	return mw
}

// SimpleRetryMiddleware creates a retry middleware with constant delay strategy
func SimpleRetryMiddleware(attempts int, retryDelay time.Duration) ConfigurableMiddleware {
	strategy := NewConstantDelayStrategy(retryDelay, attempts)
	return RetryMiddleware(strategy)
}

// ExponentialRetryMiddleware creates a retry middleware with exponential backoff strategy
func ExponentialRetryMiddleware(
	maxAttempts int,
	initialDelay time.Duration,
	maxDelay time.Duration,
	factor float64,
) ConfigurableMiddleware {
	strategy := NewExponentialBackoffStrategy(initialDelay, maxDelay, factor, maxAttempts)
	return RetryMiddleware(strategy)
}

func (m *retryMiddleware) roundTrip(next core.RoundTripFunc) core.RoundTripFunc {
	return func(req *http.Request) (*http.Response, error) {
		var buf *bytes.Buffer
		if req.Body != nil {
			buf = bodyPool.Get().(*bytes.Buffer)
			defer bodyPool.Put(buf)
			buf.Reset()

			_, err := io.Copy(buf, req.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to copy request body: %w", err)
			}
			_ = req.Body.Close()
		}

		var resp *http.Response
		var err error
		var attempt int

		for {
			if ctxErr := req.Context().Err(); ctxErr != nil {
				return nil, ctxErr
			}

			// Clone the request for each attempt only if it exists
			if buf != nil {
				req.Body = io.NopCloser(bytes.NewReader(buf.Bytes()))
			} else {
				req.Body = nil
			}

			// Add retry attempt header for debugging
			if attempt > 0 {
				req.Header.Set("X-Retry-Attempt", strconv.Itoa(attempt))
			}

			resp, err = next(req)

			// Check if we should retry
			if !m.strategy.ShouldRetry(attempt, resp, err) {
				break
			}

			// We're going to retry, so close the response if it exists
			if resp != nil {
				DrainAndClose(resp)
			}

			// Increment attempt counter
			attempt++

			// Wait before retrying
			delay := m.strategy.NextDelay(attempt, resp, err)
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(delay):
				// Continue with retry
			}
		}

		// Check for network timeouts
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			err = &TimeoutError{Err: netErr}
		}

		// If we still have an error after all retries
		if err != nil {
			return nil, &RetryError{Attempts: attempt + 1, LastErr: err}
		}

		return resp, nil
	}
}

// DrainAndClose reads the remaining data from resp.Body and closes it.
func DrainAndClose(resp *http.Response) {
	if resp.Body != nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
}
