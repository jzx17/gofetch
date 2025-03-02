package middlewares_test

import (
	"context"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jzx17/gofetch/core"
	"github.com/jzx17/gofetch/middlewares"
)

var _ = Describe("RateLimit Middleware", func() {
	var (
		options          middlewares.RateLimitOptions
		middleware       middlewares.ConfigurableMiddleware
		request          *http.Request
		nextCalled       int
		mockRoundTripper core.RoundTripFunc
	)

	BeforeEach(func() {
		// Reset test variables
		nextCalled = 0

		// Default test request
		var err error
		request, err = http.NewRequest("GET", "https://example.com/test", nil)
		Expect(err).NotTo(HaveOccurred())

		// Create a mock round tripper that counts calls
		mockRoundTripper = func(req *http.Request) (*http.Response, error) {
			nextCalled++
			return &http.Response{StatusCode: 200}, nil
		}

		// Default options
		options = middlewares.DefaultRateLimitOptions()
	})

	Describe("Basic functionality", func() {
		It("should allow requests within rate limit", func() {
			// Configure a high rate limit
			options.RequestsPerSecond = 100
			options.Burst = 10
			middleware = middlewares.RateLimitMiddleware(options)

			// Execute a few requests
			wrappedFunc := middleware.Wrap(mockRoundTripper)
			for i := 0; i < 5; i++ {
				resp, err := wrappedFunc(request)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(200))
			}

			Expect(nextCalled).To(Equal(5))
		})

		It("should block requests exceeding rate limit", func() {
			// Configure a very low rate limit with no waiting
			options.RequestsPerSecond = 1
			options.Burst = 1
			options.WaitOnLimit = false
			middleware = middlewares.RateLimitMiddleware(options)

			wrappedFunc := middleware.Wrap(mockRoundTripper)

			// First request should succeed
			resp, err := wrappedFunc(request)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))

			// Second request should be rate limited
			resp, err = wrappedFunc(request)
			Expect(err).To(HaveOccurred())

			// Check error type
			rateLimitErr, ok := err.(*middlewares.RateLimitExceededError)
			Expect(ok).To(BeTrue(), "Expected RateLimitExceededError")
			Expect(rateLimitErr.Limit).To(Equal(1.0))
			Expect(rateLimitErr.RetryAfter).To(BeNumerically(">", 0))

			// Only the first request should have reached the next handler
			Expect(nextCalled).To(Equal(1))
		})
	})

	Describe("Token replenishment", func() {
		It("should replenish tokens over time", func() {
			// 2 requests per second, 1 burst
			options.RequestsPerSecond = 2
			options.Burst = 1
			options.WaitOnLimit = false
			middleware = middlewares.RateLimitMiddleware(options)

			wrappedFunc := middleware.Wrap(mockRoundTripper)

			// First request should succeed
			_, err := wrappedFunc(request)
			Expect(err).NotTo(HaveOccurred())

			// Second request should fail (exceeds burst)
			_, err = wrappedFunc(request)
			Expect(err).To(HaveOccurred())

			// Wait for token to replenish (0.5s for 1 token at 2 RPS)
			time.Sleep(600 * time.Millisecond)

			// Now we should be able to make another request
			_, err = wrappedFunc(request)
			Expect(err).NotTo(HaveOccurred())

			// And the next one should fail again
			_, err = wrappedFunc(request)
			Expect(err).To(HaveOccurred())

			Expect(nextCalled).To(Equal(2))
		})

		It("should cap tokens at burst limit", func() {
			// 10 requests per second, 2 burst
			options.RequestsPerSecond = 10
			options.Burst = 2
			options.WaitOnLimit = false
			middleware = middlewares.RateLimitMiddleware(options)

			wrappedFunc := middleware.Wrap(mockRoundTripper)

			// Wait for tokens to accumulate (should be capped at burst)
			time.Sleep(1 * time.Second)

			// First 3 requests: 2 should succeed, 1 should fail
			_, err1 := wrappedFunc(request)
			_, err2 := wrappedFunc(request)
			_, err3 := wrappedFunc(request)

			Expect(err1).NotTo(HaveOccurred())
			Expect(err2).NotTo(HaveOccurred())
			Expect(err3).To(HaveOccurred())

			Expect(nextCalled).To(Equal(2))
		})
	})

	Describe("Waiting behavior", func() {
		It("should wait for tokens when configured to do so", func() {
			// 5 requests per second, 1 burst, with waiting
			options.RequestsPerSecond = 5
			options.Burst = 1
			options.WaitOnLimit = true
			options.MaxWaitTime = 1 * time.Second
			middleware = middlewares.RateLimitMiddleware(options)

			wrappedFunc := middleware.Wrap(mockRoundTripper)

			start := time.Now()

			// Make 3 requests rapidly - this should trigger waiting
			_, err1 := wrappedFunc(request)
			_, err2 := wrappedFunc(request)
			_, err3 := wrappedFunc(request)

			elapsed := time.Since(start)

			// All requests should succeed
			Expect(err1).NotTo(HaveOccurred())
			Expect(err2).NotTo(HaveOccurred())
			Expect(err3).NotTo(HaveOccurred())

			// Should have waited at least 400ms (200ms per token at 5 RPS)
			Expect(elapsed).To(BeNumerically(">=", 400*time.Millisecond))

			// All requests should have reached the handler
			Expect(nextCalled).To(Equal(3))
		})

		It("should respect max wait time", func() {
			// 0.1 requests per second (very slow), 1 burst, with waiting
			options.RequestsPerSecond = 0.1 // 1 request per 10 seconds
			options.Burst = 1
			options.WaitOnLimit = true
			options.MaxWaitTime = 500 * time.Millisecond // But only wait 500ms max
			middleware = middlewares.RateLimitMiddleware(options)

			wrappedFunc := middleware.Wrap(mockRoundTripper)

			// First request should succeed
			_, err1 := wrappedFunc(request)
			Expect(err1).NotTo(HaveOccurred())

			// Second request should exceed max wait time
			start := time.Now()
			_, err2 := wrappedFunc(request)
			elapsed := time.Since(start)

			// Should fail with rate limit error
			Expect(err2).To(HaveOccurred())

			// Should not have waited more than max wait time
			Expect(elapsed).To(BeNumerically("<", 600*time.Millisecond))

			// Only first request should have reached handler
			Expect(nextCalled).To(Equal(1))
		})

		It("should respect context cancellation", func() {
			// 0.5 requests per second, 1 burst, with waiting
			options.RequestsPerSecond = 0.5 // 1 request per 2 seconds
			options.Burst = 1
			options.WaitOnLimit = true
			options.MaxWaitTime = 5 * time.Second
			middleware = middlewares.RateLimitMiddleware(options)

			wrappedFunc := middleware.Wrap(mockRoundTripper)

			// First request should succeed
			_, err := wrappedFunc(request)
			Expect(err).NotTo(HaveOccurred())

			// Create a request with a context that will cancel quickly
			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			defer cancel()

			requestWithCtx, err := http.NewRequestWithContext(ctx, "GET", "https://example.com/test", nil)
			Expect(err).NotTo(HaveOccurred())

			// This request should get cancelled while waiting
			start := time.Now()
			_, err = wrappedFunc(requestWithCtx)
			elapsed := time.Since(start)

			// Should fail with context deadline exceeded
			Expect(err).To(MatchError(context.DeadlineExceeded))

			// Should have waited close to the context timeout
			Expect(elapsed).To(BeNumerically(">=", 250*time.Millisecond))
			Expect(elapsed).To(BeNumerically("<", 500*time.Millisecond))

			// Only first request should have reached handler
			Expect(nextCalled).To(Equal(1))
		})
	})

	Describe("Configuration options", func() {
		It("should use default options for invalid values", func() {
			// Invalid options
			invalidOptions := middlewares.RateLimitOptions{
				RequestsPerSecond: -1,
				Burst:             -5,
				MaxWaitTime:       -100 * time.Millisecond,
			}

			middleware = middlewares.RateLimitMiddleware(invalidOptions)

			// Get the options from the middleware identifier
			identifier := middleware.GetIdentifier()
			actualOptions, ok := identifier.Options.(middlewares.RateLimitOptions)
			Expect(ok).To(BeTrue())

			// Should have been normalized to defaults
			defaults := middlewares.DefaultRateLimitOptions()
			Expect(actualOptions.RequestsPerSecond).To(Equal(defaults.RequestsPerSecond))
			Expect(actualOptions.Burst).To(Equal(defaults.Burst))
			Expect(actualOptions.MaxWaitTime).To(Equal(defaults.MaxWaitTime))
		})

		It("should properly configure via functional options", func() {
			middleware = middlewares.NewRateLimitMiddleware(
				middlewares.WithRequestsPerSecond(42),
				middlewares.WithBurst(7),
				middlewares.WithWaitOnLimit(false),
				middlewares.WithMaxWaitTime(10*time.Second),
			)

			identifier := middleware.GetIdentifier()
			options, ok := identifier.Options.(middlewares.RateLimitOptions)
			Expect(ok).To(BeTrue())

			Expect(options.RequestsPerSecond).To(Equal(42.0))
			Expect(options.Burst).To(Equal(7))
			Expect(options.WaitOnLimit).To(BeFalse())
			Expect(options.MaxWaitTime).To(Equal(10 * time.Second))
		})
	})

	Describe("Error details", func() {
		It("should provide useful error information", func() {
			options.RequestsPerSecond = 1
			options.Burst = 1
			options.WaitOnLimit = false
			middleware = middlewares.RateLimitMiddleware(options)

			wrappedFunc := middleware.Wrap(mockRoundTripper)

			// Use up the burst capacity
			_, _ = wrappedFunc(request)

			// This should fail
			_, err := wrappedFunc(request)
			Expect(err).To(HaveOccurred())

			rateLimitErr, ok := err.(*middlewares.RateLimitExceededError)
			Expect(ok).To(BeTrue())

			// Check error details
			Expect(rateLimitErr.Limit).To(Equal(1.0))
			Expect(rateLimitErr.RetryAfter).To(BeNumerically(">", 0))

			// Error string should contain limit and retry info
			errString := rateLimitErr.Error()
			Expect(errString).To(ContainSubstring("rate limit exceeded"))
			Expect(errString).To(ContainSubstring("1 requests per second"))
			Expect(errString).To(ContainSubstring("retry after"))
		})
	})

	Describe("Edge cases", func() {
		It("should handle very high rate limits", func() {
			options.RequestsPerSecond = 100000 // 100k RPS
			options.Burst = 1000
			middleware = middlewares.RateLimitMiddleware(options)

			wrappedFunc := middleware.Wrap(mockRoundTripper)

			// Should be able to make many requests without hitting rate limit
			for i := 0; i < 100; i++ {
				_, err := wrappedFunc(request)
				Expect(err).NotTo(HaveOccurred())
			}

			Expect(nextCalled).To(Equal(100))
		})

		It("should handle zero burst", func() {
			// Configure rate limit with zero burst
			options.RequestsPerSecond = 10 // 10 RPS = 1 token every 100ms
			options.Burst = 0
			options.WaitOnLimit = false
			middleware = middlewares.RateLimitMiddleware(options)

			wrappedFunc := middleware.Wrap(mockRoundTripper)

			// First request should fail since burst=0 means no initial tokens
			_, err1 := wrappedFunc(request)
			Expect(err1).To(HaveOccurred())
			_, ok := err1.(*middlewares.RateLimitExceededError)
			Expect(ok).To(BeTrue(), "Expected RateLimitExceededError")

			// Wait long enough for a token to be available (150ms > 100ms needed)
			time.Sleep(150 * time.Millisecond)

			// After waiting, we should be able to make a request
			_, err2 := wrappedFunc(request)
			Expect(err2).NotTo(HaveOccurred())

			// Only one request should have succeeded
			Expect(nextCalled).To(Equal(1))
		})

	})
})
