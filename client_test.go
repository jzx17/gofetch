package gofetch_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"github.com/jzx17/gofetch"
	"github.com/jzx17/gofetch/core"
	"github.com/jzx17/gofetch/utils/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client", func() {
	var (
		testServer *httptest.Server
	)

	BeforeEach(func() {
		testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/json":
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, `{"message": "ok"}`)
			case "/text":
				_, _ = fmt.Fprint(w, "Hello, World!")
			case "/stream":
				// Simulate streaming by sending several chunks.
				for i := 0; i < 3; i++ {
					_, _ = fmt.Fprintf(w, "chunk%d\n", i)
					time.Sleep(10 * time.Millisecond)
				}
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
	})

	AfterEach(func() {
		testServer.Close()
	})

	It("should create a new client with default settings", func() {
		client := gofetch.NewClient()
		Expect(client).NotTo(BeNil())
	})

	It("should respect custom timeout", func() {
		// Create a client with a very short timeout.
		client := gofetch.NewClient(gofetch.WithTimeout(50 * time.Millisecond))
		// Create a server that sleeps longer than the timeout.
		slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			_, _ = fmt.Fprint(w, "slow")
		}))
		defer slowServer.Close()

		req := core.NewRequest("GET", slowServer.URL)
		_, err := client.Do(context.Background(), req)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Client.Timeout"))
	})

	It("should perform a GET request and read the full response", func() {
		client := gofetch.NewClient()
		req := core.NewRequest("GET", testServer.URL+"/text")
		resp, err := client.Do(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())

		// Use the Bytes() helper to read the auto-read, auto-closed response.
		body, err := resp.Bytes()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).To(Equal("Hello, World!"))
	})

	It("should perform a streaming request and allow manual reading", func() {
		client := gofetch.NewClient()
		req := core.NewRequest("GET", testServer.URL+"/stream")
		resp, err := client.DoStream(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		// Caller is responsible for closing the response.
		defer resp.CloseBody()

		var chunks []string
		err = resp.StreamChunks(func(chunk []byte) {
			chunks = append(chunks, string(chunk))
		})
		Expect(err).NotTo(HaveOccurred())

		// Combine all chunks (whether one or several) into one string.
		combined := ""
		for _, s := range chunks {
			combined += s
		}
		// The test server writes: "chunk0\nchunk1\nchunk2\n"
		Expect(combined).To(Equal("chunk0\nchunk1\nchunk2\n"))
	})

	It("should return an error if request building fails", func() {
		client := gofetch.NewClient()
		// NewRequest with an empty URL should trigger a build error.
		req := core.NewRequest("GET", "")
		_, err := client.Do(context.Background(), req)
		Expect(err).To(HaveOccurred())
	})

	It("should use a custom HTTP client provided via WithHTTPClient", func() {
		// Create a dummy RoundTrip function.
		rt := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": {"text/custom"}},
				Body:       io.NopCloser(strings.NewReader("custom client response")),
			}, nil
		})
		customClient := &http.Client{Transport: rt}
		client := gofetch.NewClient(gofetch.WithHTTPClient(customClient))
		req := core.NewRequest("GET", "http://dummy")
		resp, err := client.Do(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		body, err := io.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).To(Equal("custom client response"))
	})

	It("should use a custom transport provided via WithTransport", func() {
		// Create a dummy RoundTrip function that returns a 201 status.
		rt := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 201,
				Header:     http.Header{"Content-Type": {"text/transport"}},
				Body:       io.NopCloser(strings.NewReader("custom transport response")),
			}, nil
		})
		client := gofetch.NewClient(gofetch.WithTransport(rt))
		req := core.NewRequest("GET", "http://dummy")
		resp, err := client.Do(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(201))
		body, err := io.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).To(Equal("custom transport response"))
	})

	It("should invoke middleware in proper order using WithMiddlewares", func() {
		// Create two middleware functions that append letters to a header.
		mwA := test.CreateTestMiddleware("middleware-a", func(req *http.Request) {
			req.Header.Set("X-Mw", req.Header.Get("X-Mw")+"A")
		})

		mwB := test.CreateTestMiddleware("middleware-b", func(req *http.Request) {
			req.Header.Set("X-Mw", req.Header.Get("X-Mw")+"B")
		})
		// Capture the header value inside the dummy round trip.
		var headerValue string
		dummy := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
			// Capture the header value after middleware have run.
			headerValue = req.Header.Get("X-Mw")
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"X-Mw": {headerValue}},
				Body:       io.NopCloser(strings.NewReader("mw test response")),
			}, nil
		})
		// Disable auto-buffering to see the header change directly.
		client := gofetch.NewClient(
			gofetch.WithMiddlewares(mwA, mwB),
			gofetch.WithTransport(dummy),
			gofetch.WithAutoBufferResponse(false),
		)
		// Reassign in case WithHeader returns a new instance.
		req := core.NewRequest("GET", "http://dummy")
		req = req.WithHeader("X-MW", "")
		resp, err := client.Do(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		// Verify the header captured inside the dummy is "AB"
		Expect(headerValue).To(Equal("AB"))
		// Also verify that the response header carries the expected value.
		Expect(resp.Response.Header.Get("X-Mw")).To(Equal("AB"))
	})

	It("should return the raw response when autoBuffer is disabled", func() {
		// Create a dummy round trip that returns a fixed response.
		rt := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": {"text/plain"}},
				Body:       io.NopCloser(strings.NewReader("raw response data")),
			}, nil
		})
		client := gofetch.NewClient(gofetch.WithTransport(rt), gofetch.WithAutoBufferResponse(false))
		req := core.NewRequest("GET", "http://dummy")
		resp, err := client.Do(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		// Read the body and verify its content.
		body, err := io.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).To(Equal("raw response data"))
	})

	It("should add middleware using AddMiddlewares", func() {
		// Create a dummy transport to capture the request header after middleware processing.
		var headerValue string
		dummyTransport := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
			headerValue = req.Header.Get("X-Mw")
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": {"text/plain"}},
				Body:       io.NopCloser(strings.NewReader("test response")),
			}, nil
		})
		// Initialize client with the dummy transport and disable auto-buffering.
		client := gofetch.NewClient(gofetch.WithTransport(dummyTransport), gofetch.WithAutoBufferResponse(false))
		// Initially, there should be no middlewares.
		Expect(client.GetMiddlewares()).To(HaveLen(0))

		// Define a middleware that appends "X" to the "X-Mw" header.
		mw := test.CreateTestMiddleware("test-middleware", func(req *http.Request) {
			req.Header.Set("X-Mw", req.Header.Get("X-Mw")+"X")
		})
		// Add the middleware.
		client.Use(mw)
		Expect(client.GetMiddlewares()).To(HaveLen(1))

		// Build and execute a request.
		req := core.NewRequest("GET", "http://dummy")
		req = req.WithHeader("X-Mw", "")
		resp, err := client.Do(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())

		Expect(headerValue).To(Equal("X"))

		body, err := io.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).To(Equal("test response"))
	})

	It("should return a defensive copy from GetMiddlewares", func() {
		// Define a dummy middleware.
		mw := gofetch.CreateMiddleware(
			"passthrough-middleware", // Name for identification
			nil,                      // No special options needed
			func(next gofetch.RoundTripFunc) gofetch.RoundTripFunc {
				return next
			},
		)
		client := gofetch.NewClient()
		client.Use(mw)
		// Get the middlewares slice.
		mwsCopy := client.GetMiddlewares()
		Expect(mwsCopy).To(HaveLen(1))

		// Modify the returned slice.
		mwsCopy = mwsCopy[:0]
		// Get the middlewares again; it should remain unaffected.
		mwsAgain := client.GetMiddlewares()
		Expect(mwsAgain).To(HaveLen(1))
	})

	It("should be safe to add middlewares concurrently", func() {
		client := gofetch.NewClient()
		const numGoroutines = 100
		var wg sync.WaitGroup
		wg.Add(numGoroutines)
		// Define a no-op middleware.
		mw := gofetch.CreateMiddleware(
			"pass through middleware", // Name for identification
			nil,                       // No special options needed
			func(next gofetch.RoundTripFunc) gofetch.RoundTripFunc {
				return next
			},
		)
		// Concurrently add middleware.
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				client.Use(mw)
			}()
		}
		wg.Wait()
		// Ensure the total count of middlewares is as expected.
		Expect(client.GetMiddlewares()).To(HaveLen(numGoroutines))
	})

	// Test UpdateMiddleware method
	It("should update existing middleware with the same name", func() {
		// Create a client
		client := gofetch.NewClient()

		// Add initial middleware
		mw1 := test.CreateTestMiddleware("test-mw", func(req *http.Request) {
			req.Header.Set("X-Test", "value1")
		})
		client.UpdateMiddleware(mw1)

		// Update with new middleware with same name
		mw2 := test.CreateTestMiddleware("test-mw", func(req *http.Request) {
			req.Header.Set("X-Test", "value2")
		})
		client.UpdateMiddleware(mw2)

		// Should only have one middleware
		middlewares := client.GetMiddlewares()
		Expect(middlewares).To(HaveLen(1))

		// Verify the middleware was updated by checking effect on request
		var receivedHeader string
		testTransport := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
			receivedHeader = req.Header.Get("X-Test")
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("OK")),
			}, nil
		})

		// Set custom transport and make a request
		client = gofetch.NewClient(gofetch.WithTransport(testTransport))
		client.UpdateMiddleware(mw2)

		req := core.NewRequest("GET", "http://example.com")
		_, err := client.Do(context.Background(), req)
		Expect(err).NotTo(HaveOccurred())
		Expect(receivedHeader).To(Equal("value2"))
	})

	// Test RemoveMiddleware method
	It("should remove middleware by name", func() {
		client := gofetch.NewClient()

		// Add two middlewares
		mw1 := test.CreateTestMiddleware("mw1", func(req *http.Request) {
			req.Header.Set("X-Mw1", "yes")
		})
		mw2 := test.CreateTestMiddleware("mw2", func(req *http.Request) {
			req.Header.Set("X-Mw2", "yes")
		})

		client.Use(mw1).Use(mw2)
		Expect(client.GetMiddlewares()).To(HaveLen(2))

		// Remove one middleware
		client.RemoveMiddleware("mw1")

		// Should only have one middleware left
		middlewares := client.GetMiddlewares()
		Expect(middlewares).To(HaveLen(1))
		Expect(middlewares[0].GetIdentifier().Name).To(Equal("mw2"))
	})

	// Test Execute method with options
	It("should respect execute options", func() {
		client := gofetch.NewClient()
		req := core.NewRequest("GET", testServer.URL+"/text")

		// Use Execute with streaming option
		resp, err := client.Execute(context.Background(), req, gofetch.WithStreamProcessing())
		Expect(err).NotTo(HaveOccurred())
		defer resp.CloseBody()

		// Use Execute with timeout option
		shortCtx := context.Background()
		_, err = client.Execute(shortCtx, req, gofetch.WithRequestTimeout(1*time.Nanosecond))
		// This should fail with timeout error
		Expect(err).To(HaveOccurred())
	})

	// Test DoWithTimeout
	It("should respect DoWithTimeout", func() {
		client := gofetch.NewClient()
		// Create a server that sleeps longer than the timeout.
		slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			_, _ = fmt.Fprint(w, "slow")
		}))
		defer slowServer.Close()

		req := core.NewRequest("GET", slowServer.URL)
		_, err := client.DoWithTimeout(context.Background(), req, 10*time.Millisecond)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("context deadline exceeded"))
	})
})
