package gofetch_test

import (
	"context"
	"errors"
	"github.com/jzx17/gofetch"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/jzx17/gofetch/core"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client Options", func() {
	It("should respect WithHTTPClient option", func() {
		// Create a custom HTTP client with a mock transport
		mockTransport := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"X-Custom-Client": {"true"}},
				Body:       io.NopCloser(strings.NewReader("custom client response")),
			}, nil
		})
		customClient := &http.Client{Transport: mockTransport}

		// Create client with custom HTTP client
		client := gofetch.NewClient(gofetch.WithHTTPClient(customClient))

		// Make a request
		req := core.NewRequest("GET", "http://example.com")
		resp, err := client.Do(context.Background(), req)

		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Header.Get("X-Custom-Client")).To(Equal("true"))

		body, err := io.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).To(Equal("custom client response"))
	})

	It("should respect WithTransport option", func() {
		// Create a custom transport
		mockTransport := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"X-Custom-Transport": {"true"}},
				Body:       io.NopCloser(strings.NewReader("custom transport response")),
			}, nil
		})

		// Create client with custom transport
		client := gofetch.NewClient(gofetch.WithTransport(mockTransport))

		// Make a request
		req := core.NewRequest("GET", "http://example.com")
		resp, err := client.Do(context.Background(), req)

		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Header.Get("X-Custom-Transport")).To(Equal("true"))

		body, err := io.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).To(Equal("custom transport response"))
	})

	It("should respect WithTimeout option", func() {
		// Create a client with a very short timeout
		client := gofetch.NewClient(gofetch.WithTimeout(50 * time.Millisecond))

		// Create a mock server that delays response
		delayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			io.WriteString(w, "delayed response")
		}))
		defer delayServer.Close()

		// Make a request that should time out
		req := core.NewRequest("GET", delayServer.URL)
		_, err := client.Do(context.Background(), req)

		// Expect a timeout error
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Client.Timeout"))
	})

	It("should respect WithMiddlewares option", func() {
		// Create test middleware that adds a header
		testMiddleware := gofetch.CreateMiddleware(
			"test-middleware",
			nil,
			func(next gofetch.RoundTripFunc) gofetch.RoundTripFunc {
				return func(req *http.Request) (*http.Response, error) {
					req.Header.Set("X-Test-Middleware", "applied")
					return next(req)
				}
			},
		)

		// Create a mock transport that verifies the header
		var headerApplied bool
		mockTransport := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
			headerApplied = req.Header.Get("X-Test-Middleware") == "applied"
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("response")),
			}, nil
		})

		// Create client with middleware and transport
		client := gofetch.NewClient(
			gofetch.WithMiddlewares(testMiddleware),
			gofetch.WithTransport(mockTransport),
		)

		// Make a request
		req := core.NewRequest("GET", "http://example.com")
		_, err := client.Do(context.Background(), req)

		Expect(err).NotTo(HaveOccurred())
		Expect(headerApplied).To(BeTrue())
	})

	It("should respect WithAutoBufferResponse option", func() {
		// Create a mock transport
		mockTransport := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("test body")),
			}, nil
		})

		// Create client with auto-buffer disabled
		client := gofetch.NewClient(
			gofetch.WithTransport(mockTransport),
			gofetch.WithAutoBufferResponse(false),
		)

		// Make a request
		req := core.NewRequest("GET", "http://example.com")
		resp, err := client.Do(context.Background(), req)

		Expect(err).NotTo(HaveOccurred())

		// With auto-buffer disabled, we need to read and close the body manually
		body, err := io.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).To(Equal("test body"))

		err = resp.Body.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	It("should respect WithSizeConfig option", func() {
		// Create size config with small limits
		sizeConfig := gofetch.DefaultSizeConfig().
			WithRequestBodySize(10). // Only 10 bytes allowed
			WithResponseBodySize(10) // Only 10 bytes allowed

		// Create a mock transport that would succeed if reached
		mockTransport := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Body != nil {
				// make sure the body is read, otherwise the size validation middleware will be skipped
				_, err := io.ReadAll(req.Body)
				if err != nil {
					return nil, err
				}
			}

			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("response")),
			}, nil
		})

		// Create client with size config
		client := gofetch.NewClient(
			gofetch.WithSizeConfig(sizeConfig),
			gofetch.WithTransport(mockTransport),
		)

		// Verify the size middleware was added
		middlewares := client.GetMiddlewares()
		var hasSizeMiddleware bool
		for _, mw := range middlewares {
			if mw.GetIdentifier().Name == "size-validation" {
				hasSizeMiddleware = true
				break
			}
		}
		Expect(hasSizeMiddleware).To(BeTrue())

		// Test with request that exceeds limit
		largeBody := make([]byte, 20) // 20 bytes, exceeds 10 byte limit
		for i := range largeBody {
			largeBody[i] = 'A'
		}

		req := core.NewRequest("POST", "http://example.com").WithBody(largeBody)

		// This should fail due to size limit
		_, err := client.Do(context.Background(), req)
		Expect(err).To(HaveOccurred())

		// Verify it's a size error
		var sizeErr *gofetch.SizeError
		Expect(errors.As(err, &sizeErr)).To(BeTrue())
		Expect(sizeErr.Type).To(Equal("request"))
		Expect(sizeErr.Current).To(BeNumerically(">", sizeErr.Max))
	})

	It("should panic with negative values in WithSizeConfig", func() {
		invalidConfig := gofetch.SizeConfig{
			MaxRequestBodySize: -1, // Negative, should cause panic
		}

		Expect(func() {
			gofetch.NewClient(gofetch.WithSizeConfig(invalidConfig))
		}).To(Panic())
	})

	It("should respect WithConnectionPool option for http.Transport", func() {
		// This is hard to test directly, but we can verify the function doesn't panic
		transport := &http.Transport{}
		Expect(func() {
			gofetch.NewClient(
				gofetch.WithTransport(transport),
				gofetch.WithConnectionPool(100, 10),
			)
		}).NotTo(Panic())
	})
})
