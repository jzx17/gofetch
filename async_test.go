package gofetch_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/jzx17/gofetch"
	"github.com/jzx17/gofetch/core"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

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
			case "/1":
				_, _ = fmt.Fprint(w, "response 1")
			case "/2":
				_, _ = fmt.Fprint(w, "response 2")
			case "/3":
				_, _ = fmt.Fprint(w, "response 3")
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

	It("should handle asynchronous DoStreamAsync calls", func() {
		client := gofetch.NewClient()
		req := core.NewRequest("GET", testServer.URL+"/delay")

		// Start time to measure async execution
		startTime := time.Now()

		// Start async streaming request
		asyncChan := client.DoStreamAsync(context.Background(), req)

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

	It("should handle ExecuteAsync with various options", func() {
		client := gofetch.NewClient()
		req := core.NewRequest("GET", testServer.URL+"/delay")

		// Start time to measure async execution
		startTime := time.Now()

		// Start async request with stream processing
		asyncChan := client.ExecuteAsync(context.Background(), req, gofetch.WithStreamProcessing())

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

	It("should handle DoGroupAsync for multiple requests", func() {
		client := gofetch.NewClient()
		req1 := core.NewRequest("GET", testServer.URL+"/1")
		req2 := core.NewRequest("GET", testServer.URL+"/2")
		req3 := core.NewRequest("GET", testServer.URL+"/3")

		// Start time to measure group async execution
		startTime := time.Now()

		// Start group of async requests
		groupChan := client.DoGroupAsync(context.Background(), req1, req2, req3)

		// Check that the requests were started asynchronously
		Expect(time.Since(startTime)).To(BeNumerically("<", 30*time.Millisecond))

		// Now get all results
		results := <-groupChan
		Expect(len(results)).To(Equal(3))

		// Check each result
		for i, result := range results {
			Expect(result.Error).NotTo(HaveOccurred())
			Expect(result.Response).NotTo(BeNil())

			body, err := io.ReadAll(result.Response.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(body)).To(Equal(fmt.Sprintf("response %d", i+1)))
		}
	})

	It("should handle ExecuteGroupAsync for multiple requests with options", func() {
		client := gofetch.NewClient()
		req1 := core.NewRequest("GET", testServer.URL+"/1")
		req2 := core.NewRequest("GET", testServer.URL+"/2")

		// Start group of async requests with streaming option
		groupChan := client.ExecuteGroupAsync(context.Background(),
			[]*core.Request{req1, req2},
			gofetch.WithStreamProcessing())

		// Get all results
		results := <-groupChan
		Expect(len(results)).To(Equal(2))

		// Check each result
		for i, result := range results {
			Expect(result.Error).NotTo(HaveOccurred())
			Expect(result.Response).NotTo(BeNil())

			body, err := io.ReadAll(result.Response.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(body)).To(Equal(fmt.Sprintf("response %d", i+1)))
		}
	})

	It("should join async responses with JoinAsyncResponses", func() {
		// Create two channels that emit AsyncResponse
		ch1 := make(chan core.AsyncResponse, 1)
		ch2 := make(chan core.AsyncResponse, 1)

		// Put responses in channels
		ch1 <- core.AsyncResponse{
			Response: &core.Response{Response: &http.Response{
				StatusCode: 200,
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("join 1")),
			}},
			Error: nil,
		}
		ch2 <- core.AsyncResponse{
			Response: &core.Response{Response: &http.Response{
				StatusCode: 200,
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("join 2")),
			}},
			Error: nil,
		}
		close(ch1)
		close(ch2)

		// Join the channels
		client := gofetch.NewClient()
		joinChan := client.JoinAsyncResponses(ch1, ch2)
		results := <-joinChan

		// Check results
		Expect(len(results)).To(Equal(2))
		b1, err := io.ReadAll(results[0].Response.Body)
		Expect(err).NotTo(HaveOccurred())
		b2, err := io.ReadAll(results[1].Response.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(b1)).To(Equal("join 1"))
		Expect(string(b2)).To(Equal("join 2"))
	})

	It("should handle GetAsync convenience method", func() {
		client := gofetch.NewClient()
		asyncChan := client.GetAsync(context.Background(), testServer.URL+"/1", nil)
		result := <-asyncChan

		Expect(result.Error).NotTo(HaveOccurred())
		Expect(result.Response).NotTo(BeNil())

		body, err := io.ReadAll(result.Response.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).To(Equal("response 1"))
	})

	It("should handle PostAsync convenience method", func() {
		// Create a test server that echoes the POST body
		echoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			body, _ := io.ReadAll(r.Body)
			_, _ = w.Write(body)
		}))
		defer echoServer.Close()

		client := gofetch.NewClient()
		payload := []byte("test post data")
		asyncChan := client.PostAsync(context.Background(), echoServer.URL, payload, nil)
		result := <-asyncChan

		Expect(result.Error).NotTo(HaveOccurred())
		Expect(result.Response).NotTo(BeNil())

		body, err := io.ReadAll(result.Response.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).To(Equal("test post data"))
	})

	It("should handle context cancellation in async operations", func() {
		// Create server with long delay
		longDelayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(200 * time.Millisecond)
			_, _ = fmt.Fprint(w, "long delay response")
		}))
		defer longDelayServer.Close()

		// Create a context that will be cancelled shortly
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		client := gofetch.NewClient()
		req := core.NewRequest("GET", longDelayServer.URL)
		asyncChan := client.DoAsync(ctx, req)

		// Get the result - should be an error due to context cancellation
		result := <-asyncChan
		Expect(result.Error).To(HaveOccurred())
		Expect(result.Error.Error()).To(ContainSubstring("context deadline exceeded"))
	})
})
