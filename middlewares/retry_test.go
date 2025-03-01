package middlewares_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/jzx17/gofetch/core"
	"github.com/jzx17/gofetch/middlewares"
	"github.com/jzx17/gofetch/utils/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// roundTripperWrapper is a local interface exposing the middleware Wrap method.
// It is expected that the middleware (via its embedded BaseMiddleware) implements this.
type roundTripperWrapper interface {
	Wrap(core.RoundTripFunc) core.RoundTripFunc
}

var _ = Describe("Retry Middleware", func() {
	var baseURL = "http://example.com"

	Context("when a network error occurs", func() {
		It("should retry and succeed on a subsequent attempt", func() {
			var callCount int32 = 0
			var capturedRetryAttempt string

			// Fake round-trip: first call returns a network error; second returns success.
			fakeRoundTrip := func(req *http.Request) (*http.Response, error) {
				capturedRetryAttempt = req.Header.Get("X-Retry-Attempt")
				atomic.AddInt32(&callCount, 1)
				if callCount == 1 {
					return nil, &test.FakeNetError{Msg: "simulated network error"}
				} else if callCount == 2 {
					// The middleware should add the X-Retry-Attempt header on retry.
					Expect(capturedRetryAttempt).To(Equal("1"))
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewBufferString("success")),
						Header:     make(http.Header),
					}, nil
				}
				return nil, errors.New("unexpected call")
			}

			// Create a constant delay strategy allowing one retry.
			strategy := middlewares.NewConstantDelayStrategy(1*time.Millisecond, 1)
			mw := middlewares.RetryMiddleware(strategy)
			// Get the wrapping function via the exported Wrap method.
			wrapped := mw.(roundTripperWrapper).Wrap(fakeRoundTrip)

			req, err := http.NewRequest("GET", baseURL, nil)
			Expect(err).NotTo(HaveOccurred())

			resp, err := wrapped(req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).NotTo(BeNil())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			bodyBytes, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(bodyBytes)).To(Equal("success"))
			Expect(callCount).To(Equal(int32(2)))
		})
	})

	Context("when maximum retry attempts are exceeded", func() {
		It("should return a RetryError with the correct attempt count", func() {
			var callCount int32 = 0

			// Fake round-trip that always returns a network error.
			fakeRoundTrip := func(req *http.Request) (*http.Response, error) {
				atomic.AddInt32(&callCount, 1)
				return nil, &test.FakeNetError{Msg: "persistent network error"}
			}

			// With maxAttempts=1, the middleware will try the initial attempt plus one retry.
			strategy := middlewares.NewConstantDelayStrategy(1*time.Millisecond, 1)
			mw := middlewares.RetryMiddleware(strategy)
			wrapped := mw.(roundTripperWrapper).Wrap(fakeRoundTrip)

			req, err := http.NewRequest("GET", baseURL, nil)
			Expect(err).NotTo(HaveOccurred())

			resp, err := wrapped(req)
			Expect(resp).To(BeNil())
			// The error should be a RetryError wrapping the original error.
			var retryErr *middlewares.RetryError
			Expect(errors.As(err, &retryErr)).To(BeTrue())
			// With maxAttempts=1, total attempts should be 2 (initial + one retry).
			Expect(retryErr.Attempts).To(Equal(2))
			Expect(err.Error()).To(ContainSubstring("persistent network error"))
			Expect(callCount).To(Equal(int32(2)))
		})
	})

	Context("when a non-retryable error occurs", func() {
		It("should not retry and return immediately", func() {
			var callCount int32 = 0

			// Fake round-trip returning a non-network error.
			fakeRoundTrip := func(req *http.Request) (*http.Response, error) {
				atomic.AddInt32(&callCount, 1)
				return nil, errors.New("non network error")
			}

			// With maxAttempts=1, non-network errors should not be retried.
			strategy := middlewares.NewConstantDelayStrategy(1*time.Millisecond, 1)
			mw := middlewares.RetryMiddleware(strategy)
			wrapped := mw.(roundTripperWrapper).Wrap(fakeRoundTrip)

			req, err := http.NewRequest("GET", baseURL, nil)
			Expect(err).NotTo(HaveOccurred())

			resp, err := wrapped(req)
			Expect(resp).To(BeNil())
			var retryErr *middlewares.RetryError
			Expect(errors.As(err, &retryErr)).To(BeTrue())
			// Only one attempt should have been made.
			Expect(retryErr.Attempts).To(Equal(1))
			Expect(err.Error()).To(ContainSubstring("non network error"))
			Expect(callCount).To(Equal(int32(1)))
		})
	})

	Context("when a retryable HTTP status is returned", func() {
		It("should retry on a 500 status and succeed subsequently", func() {
			var callCount int32 = 0
			var capturedRetryAttempt string

			fakeRoundTrip := func(req *http.Request) (*http.Response, error) {
				capturedRetryAttempt = req.Header.Get("X-Retry-Attempt")
				atomic.AddInt32(&callCount, 1)
				if callCount == 1 {
					// Return a 500 error response.
					return &http.Response{
						StatusCode: http.StatusInternalServerError,
						Body:       io.NopCloser(bytes.NewBufferString("server error")),
						Header:     make(http.Header),
					}, nil
				} else if callCount == 2 {
					Expect(capturedRetryAttempt).To(Equal("1"))
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewBufferString("ok")),
						Header:     make(http.Header),
					}, nil
				}
				return nil, errors.New("unexpected call")
			}

			strategy := middlewares.NewConstantDelayStrategy(1*time.Millisecond, 1)
			mw := middlewares.RetryMiddleware(strategy)
			wrapped := mw.(roundTripperWrapper).Wrap(fakeRoundTrip)

			req, err := http.NewRequest("GET", baseURL, nil)
			Expect(err).NotTo(HaveOccurred())

			resp, err := wrapped(req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).NotTo(BeNil())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(callCount).To(Equal(int32(2)))
		})
	})

	Context("when the context is cancelled", func() {
		It("should abort retries and return the context error", func() {
			// Fake round-trip always returns a network error.
			fakeRoundTrip := func(req *http.Request) (*http.Response, error) {
				return nil, &test.FakeNetError{Msg: "simulated network error"}
			}

			strategy := middlewares.NewConstantDelayStrategy(50*time.Millisecond, 3)
			mw := middlewares.RetryMiddleware(strategy)
			wrapped := mw.(roundTripperWrapper).Wrap(fakeRoundTrip)

			ctx, cancel := context.WithCancel(context.Background())
			// Cancel the context immediately.
			cancel()
			req, err := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
			Expect(err).NotTo(HaveOccurred())

			resp, err := wrapped(req)
			Expect(resp).To(BeNil())
			Expect(err).To(MatchError(context.Canceled))
		})
	})

	Context("when a request body exists", func() {
		It("should preserve the body across retries", func() {
			var callCount int32 = 0
			bodyContent := "test body"
			originalBody := bytes.NewBufferString(bodyContent)

			fakeRoundTrip := func(req *http.Request) (*http.Response, error) {
				// Read the request body and ensure it matches the original.
				data, err := io.ReadAll(req.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data)).To(Equal(bodyContent))
				atomic.AddInt32(&callCount, 1)
				if callCount == 1 {
					return nil, &test.FakeNetError{Msg: "simulated network error"}
				} else if callCount == 2 {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewBufferString("success")),
						Header:     make(http.Header),
					}, nil
				}
				return nil, errors.New("unexpected call")
			}

			strategy := middlewares.NewConstantDelayStrategy(1*time.Millisecond, 1)
			mw := middlewares.RetryMiddleware(strategy)
			wrapped := mw.(roundTripperWrapper).Wrap(fakeRoundTrip)

			req, err := http.NewRequest("POST", baseURL, originalBody)
			Expect(err).NotTo(HaveOccurred())

			resp, err := wrapped(req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).NotTo(BeNil())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(callCount).To(Equal(int32(2)))
		})
	})

	// Test SimpleRetryMiddleware convenience function
	Context("when using SimpleRetryMiddleware", func() {
		It("should create a middleware with constant delay strategy", func() {
			var callCount int32 = 0

			fakeRoundTrip := func(req *http.Request) (*http.Response, error) {
				atomic.AddInt32(&callCount, 1)
				if callCount == 1 {
					return nil, &test.FakeNetError{Msg: "simulated network error"}
				} else if callCount == 2 {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewBufferString("success")),
						Header:     make(http.Header),
					}, nil
				}
				return nil, errors.New("unexpected call")
			}

			// Use SimpleRetryMiddleware instead of directly creating strategy
			mw := middlewares.SimpleRetryMiddleware(1, 1*time.Millisecond)
			wrapped := mw.(roundTripperWrapper).Wrap(fakeRoundTrip)

			req, err := http.NewRequest("GET", baseURL, nil)
			Expect(err).NotTo(HaveOccurred())

			resp, err := wrapped(req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).NotTo(BeNil())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(callCount).To(Equal(int32(2)))
		})
	})

	// Test ExponentialRetryMiddleware convenience function
	Context("when using ExponentialRetryMiddleware", func() {
		It("should create a middleware with exponential backoff strategy", func() {
			var callCount int32 = 0
			var delays []time.Duration

			// Record time between attempts
			var lastCallTime time.Time
			fakeRoundTrip := func(req *http.Request) (*http.Response, error) {
				currentTime := time.Now()
				if !lastCallTime.IsZero() {
					delays = append(delays, currentTime.Sub(lastCallTime))
				}
				lastCallTime = currentTime

				count := atomic.AddInt32(&callCount, 1)
				if count <= 3 {
					return nil, &test.FakeNetError{Msg: "simulated network error"}
				} else {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewBufferString("success")),
						Header:     make(http.Header),
					}, nil
				}
			}

			// Use ExponentialRetryMiddleware with short delays for testing
			initialDelay := 5 * time.Millisecond
			maxDelay := 50 * time.Millisecond
			factor := 2.0
			maxAttempts := 3

			mw := middlewares.ExponentialRetryMiddleware(maxAttempts, initialDelay, maxDelay, factor)
			wrapped := mw.(roundTripperWrapper).Wrap(fakeRoundTrip)

			req, err := http.NewRequest("GET", baseURL, nil)
			Expect(err).NotTo(HaveOccurred())

			resp, err := wrapped(req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).NotTo(BeNil())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			// Should have made 4 attempts (initial + 3 retries)
			Expect(callCount).To(Equal(int32(4)))

			// Should have recorded 3 delays
			Expect(delays).To(HaveLen(3))

			// Each delay should be progressively longer (within reasonable margin for test execution timing)
			if len(delays) >= 2 {
				Expect(delays[1]).To(BeNumerically(">", delays[0]))
			}
			if len(delays) >= 3 {
				Expect(delays[2]).To(BeNumerically(">", delays[1]))
			}
		})
	})

	// Test for WithRetryableStatuses utility function
	Context("when using WithRetryableStatuses", func() {
		It("should extend the list of retryable status codes", func() {
			var callCount int32 = 0

			fakeRoundTrip := func(req *http.Request) (*http.Response, error) {
				count := atomic.AddInt32(&callCount, 1)
				if count == 1 {
					// Return a 418 (I'm a teapot) error response which is not retried by default
					return &http.Response{
						StatusCode: 418, // I'm a teapot
						Body:       io.NopCloser(bytes.NewBufferString("I'm a teapot")),
						Header:     make(http.Header),
					}, nil
				} else {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewBufferString("success")),
						Header:     make(http.Header),
					}, nil
				}
			}

			// Create strategy and add 418 to retryable status codes
			strategy := middlewares.NewConstantDelayStrategy(1*time.Millisecond, 1)
			// Use the strategy directly through the RetryMiddleware without reassigning
			strategyWithExtendedCodes := middlewares.WithRetryableStatuses(strategy, 418)

			mw := middlewares.RetryMiddleware(strategyWithExtendedCodes)
			wrapped := mw.(roundTripperWrapper).Wrap(fakeRoundTrip)

			req, err := http.NewRequest("GET", baseURL, nil)
			Expect(err).NotTo(HaveOccurred())

			resp, err := wrapped(req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp).NotTo(BeNil())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(callCount).To(Equal(int32(2)))
		})
	})
})
