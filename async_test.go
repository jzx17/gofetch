package gofetch_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jzx17/gofetch"
	"github.com/jzx17/gofetch/core"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// ContextTracker tracks the lifecycle of contexts
type ContextTracker struct {
	Count      *int32
	ResetCount func()
}

// NewContextTracker creates a new context tracker
func NewContextTracker() *ContextTracker {
	var count int32
	return &ContextTracker{
		Count: &count,
		ResetCount: func() {
			atomic.StoreInt32(&count, 0)
		},
	}
}

// ContextTrackerTransport injects context tracking into the transport layer
type ContextTrackerTransport struct {
	tracker       *ContextTracker
	nextTransport http.RoundTripper
}

func (t *ContextTrackerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Create a done channel we can use to ensure cleanup happens exactly once
	done := make(chan struct{})
	cleanup := sync.Once{}

	// Increment counter when request starts
	atomic.AddInt32(t.tracker.Count, 1)

	// Ensure cleanup happens on any exit path
	defer func() {
		cleanup.Do(func() {
			atomic.AddInt32(t.tracker.Count, -1)
			close(done)
		})
	}()

	// Check if context is already done
	select {
	case <-req.Context().Done():
		return nil, req.Context().Err()
	default:
		// Continue with request
	}

	// Start a goroutine to watch for context cancellation
	go func() {
		select {
		case <-req.Context().Done():
			// Context was canceled, make sure we clean up
			cleanup.Do(func() {
				atomic.AddInt32(t.tracker.Count, -1)
				close(done)
			})
		case <-done:
			// Regular completion, nothing to do
		}
	}()

	// Only proceed with the actual request if context is still active
	resp, err := t.nextTransport.RoundTrip(req)

	// If the request succeeded but returned an error, ensure cleanup
	if err != nil {
		cleanup.Do(func() {
			atomic.AddInt32(t.tracker.Count, -1)
			close(done)
		})
		return nil, err
	}

	// If we got a valid response, wrap its body to track closing
	if resp != nil && resp.Body != nil {
		originalBody := resp.Body
		resp.Body = &contextTrackerReadCloser{
			ReadCloser: originalBody,
			cleanup: func() {
				cleanup.Do(func() {
					atomic.AddInt32(t.tracker.Count, -1)
					close(done)
				})
			},
		}
	}

	return resp, nil
}

// contextTrackerReadCloser wraps response bodies to track when they're closed
type contextTrackerReadCloser struct {
	io.ReadCloser
	cleanup  func()
	closedAt time.Time
}

func (rc *contextTrackerReadCloser) Close() error {
	if rc.closedAt.IsZero() {
		rc.closedAt = time.Now()
		rc.cleanup()
	}
	return rc.ReadCloser.Close()
}

// TimeoutTransport injects artificial timeouts for testing
type TimeoutTransport struct {
	nextTransport http.RoundTripper
	delayedPaths  map[string]time.Duration
	defaultDelay  time.Duration
}

func NewTimeoutTransport(next http.RoundTripper) *TimeoutTransport {
	return &TimeoutTransport{
		nextTransport: next,
		delayedPaths:  make(map[string]time.Duration),
		defaultDelay:  50 * time.Millisecond,
	}
}

func (t *TimeoutTransport) WithPathDelay(path string, delay time.Duration) *TimeoutTransport {
	t.delayedPaths[path] = delay
	return t
}

func (t *TimeoutTransport) WithDefaultDelay(delay time.Duration) *TimeoutTransport {
	t.defaultDelay = delay
	return t
}

func (t *TimeoutTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	path := req.URL.Path
	delay := t.defaultDelay

	// Check if this path has a special delay
	if customDelay, exists := t.delayedPaths[path]; exists {
		delay = customDelay
	}

	// Create a timer channel for the delay
	timer := time.NewTimer(delay)
	defer timer.Stop()

	// Check for cancellation or delay completion
	select {
	case <-req.Context().Done():
		return nil, req.Context().Err()
	case <-timer.C:
		// Continue with request after delay
		return t.nextTransport.RoundTrip(req)
	}
}

var _ = Describe("Async Operations", func() {
	var (
		testServer *httptest.Server
	)

	BeforeEach(func() {
		testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/delay":
				// Add a small delay to simulate network latency
				time.Sleep(50 * time.Millisecond)
				_, _ = fmt.Fprint(w, "delayed response")
			case "/long-delay":
				// Longer delay to test timeouts
				time.Sleep(200 * time.Millisecond)
				_, _ = fmt.Fprint(w, "long delayed response")
			case "/1":
				_, _ = fmt.Fprint(w, "response 1")
			case "/2":
				_, _ = fmt.Fprint(w, "response 2")
			case "/3":
				_, _ = fmt.Fprint(w, "response 3")
			case "/panic":
				// This endpoint will trigger a handler that panics
				panic("test panic in handler")
			default:
				_, _ = fmt.Fprint(w, "default response")
			}
		}))
	})

	AfterEach(func() {
		testServer.Close()
	})

	It("should handle asynchronous DoAsync calls", func() {
		client := gofetch.NewClient()
		req := core.NewRequest("GET", testServer.URL+"/delay")

		// Start time to measure async execution
		startTime := time.Now()

		// Start async request
		asyncChan := client.DoAsync(context.Background(), req)

		// Check that the request was started asynchronously
		Expect(time.Since(startTime)).To(BeNumerically("<", 50*time.Millisecond))

		// Now get the result
		result := <-asyncChan
		Expect(result.Error).NotTo(HaveOccurred())
		Expect(result.Response).NotTo(BeNil())

		body, err := io.ReadAll(result.Response.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).To(Equal("delayed response"))
	})

	It("should handle context cancellation in async operations", func() {
		// Create a context that will be cancelled shortly
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		client := gofetch.NewClient()
		req := core.NewRequest("GET", testServer.URL+"/long-delay")
		asyncChan := client.DoAsync(ctx, req)

		// Get the result - should be an error due to context cancellation
		result := <-asyncChan
		Expect(result.Error).To(HaveOccurred())
		Expect(result.Error.Error()).To(ContainSubstring("context deadline exceeded"))
	})

	It("should recover from panics in async operations", func() {
		// We'll create a client with custom transport that panics
		transport := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
			panic("test panic in transport")
		})

		client := gofetch.NewClient(gofetch.WithTransport(transport))
		req := core.NewRequest("GET", "http://example.com")

		asyncChan := client.DoAsync(context.Background(), req)
		result := <-asyncChan

		Expect(result.Error).To(HaveOccurred())
		Expect(result.Error.Error()).To(ContainSubstring("panic occurred"))
	})

	It("should handle DoGroupAsync with a slice of requests", func() {
		client := gofetch.NewClient()
		requests := []*core.Request{
			core.NewRequest("GET", testServer.URL+"/1"),
			core.NewRequest("GET", testServer.URL+"/2"),
			core.NewRequest("GET", testServer.URL+"/3"),
		}

		groupChan := client.DoGroupAsync(context.Background(), requests)
		results := <-groupChan

		Expect(len(results)).To(Equal(3))

		for i, result := range results {
			Expect(result.Error).NotTo(HaveOccurred())
			body, err := io.ReadAll(result.Response.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(body)).To(Equal(fmt.Sprintf("response %d", i+1)))
		}
	})

	It("should respect context cancellation in JoinAsyncResponses", func() {
		ctx, cancel := context.WithCancel(context.Background())

		// Create channels that will never receive a response
		ch1 := make(chan core.AsyncResponse)
		ch2 := make(chan core.AsyncResponse)

		// Start the join operation
		client := gofetch.NewClient()
		joinChan := client.JoinAsyncResponses(ctx, ch1, ch2)

		// Cancel the context
		cancel()

		// Should receive a result with context cancelled errors
		results := <-joinChan
		Expect(len(results)).To(Equal(2))
		Expect(results[0].Error).To(Equal(context.Canceled))
		Expect(results[1].Error).To(Equal(context.Canceled))
	})

	It("should apply individual timeouts in DoGroupAsyncWithOptions", func() {
		client := gofetch.NewClient()

		// Create a mix of fast and slow endpoints
		requests := []*core.Request{
			core.NewRequest("GET", testServer.URL+"/1"),          // Fast
			core.NewRequest("GET", testServer.URL+"/long-delay"), // Slow
		}

		// Set a timeout that will allow the fast request but timeout the slow one
		opts := gofetch.GroupOptions{
			IndividualTimeout: 30 * time.Millisecond, // Very short timeout for the test
		}

		// Use a background context with no timeout
		groupChan := client.DoGroupAsyncWithOptions(context.Background(), requests, opts)
		results := <-groupChan

		Expect(len(results)).To(Equal(2))
		Expect(results[0].Error).NotTo(HaveOccurred()) // Fast request should succeed
		Expect(results[1].Error).To(HaveOccurred())    // Slow request should time out
		Expect(results[1].Error.Error()).To(ContainSubstring("context deadline exceeded"))
	})

	It("should use customizable buffer sizes for high throughput", func() {
		client := gofetch.NewClient()

		// Create many requests
		var requests []*core.Request
		for i := 0; i < 10; i++ { // Reduced from 100 to 10 for faster tests
			requests = append(requests, core.NewRequest("GET", testServer.URL+"/1"))
		}

		// Use a large buffer to handle all results at once
		opts := gofetch.GroupOptions{
			BufferSize: 10,
		}

		groupChan := client.DoGroupAsyncWithOptions(context.Background(), requests, opts)
		results := <-groupChan

		Expect(len(results)).To(Equal(10))
	})

	It("should handle ExecuteGroupAsyncWithOptions", func() {
		client := gofetch.NewClient()

		requests := []*core.Request{
			core.NewRequest("GET", testServer.URL+"/1"),
			core.NewRequest("GET", testServer.URL+"/2"),
		}

		groupOpts := gofetch.GroupOptions{
			BufferSize: 2,
		}

		// Use ExecuteGroupAsyncWithOptions with both group options and execution options
		groupChan := client.ExecuteGroupAsyncWithOptions(
			context.Background(),
			requests,
			groupOpts,
			gofetch.WithStreamProcessing(), // Execution option
		)

		results := <-groupChan

		Expect(len(results)).To(Equal(2))
		for i, result := range results {
			Expect(result.Error).NotTo(HaveOccurred())
			body, err := io.ReadAll(result.Response.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(body)).To(Equal(fmt.Sprintf("response %d", i+1)))
		}
	})

	It("should clean up resources properly with individual timeouts", func() {
		// Create a context tracker to monitor request lifecycles
		tracker := NewContextTracker()

		// Create a transport chain that adds artificial delays and tracks contexts
		baseTransport := http.DefaultTransport
		timeoutTransport := NewTimeoutTransport(baseTransport).
			WithPathDelay("/long-delay", 300*time.Millisecond) // Longer than our timeout

		trackerTransport := &ContextTrackerTransport{
			tracker:       tracker,
			nextTransport: timeoutTransport,
		}

		// Create a client with our instrumented transport
		client := gofetch.NewClient(gofetch.WithTransport(trackerTransport))

		// Create test requests
		requests := []*core.Request{
			core.NewRequest("GET", testServer.URL+"/long-delay"),
			core.NewRequest("GET", testServer.URL+"/long-delay"),
		}

		// Set a timeout shorter than the artificial delay
		opts := gofetch.GroupOptions{
			IndividualTimeout: 30 * time.Millisecond,
		}

		// Execute the requests with timeouts
		groupChan := client.DoGroupAsyncWithOptions(context.Background(), requests, opts)
		results := <-groupChan

		// Verify that the requests timed out
		for _, result := range results {
			Expect(result.Error).To(HaveOccurred())
			Expect(result.Error.Error()).To(ContainSubstring("context deadline exceeded"))
		}

		// The activeContexts should be reset to 0 once all the contexts are cleaned up
		Eventually(func() int32 {
			return atomic.LoadInt32(tracker.Count)
		}, "2s", "100ms").Should(Equal(int32(0)), "Context count should be zero after all requests complete")
	})
})
