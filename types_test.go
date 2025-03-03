package gofetch_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/jzx17/gofetch"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Types and Constructors", func() {
	It("should create requests with proper HTTP method constants", func() {
		// Verify method constants work correctly
		methods := []struct {
			constant gofetch.RequestMethod
			expected string
		}{
			{gofetch.MethodGet, "GET"},
			{gofetch.MethodPost, "POST"},
			{gofetch.MethodPut, "PUT"},
			{gofetch.MethodDelete, "DELETE"},
			{gofetch.MethodHead, "HEAD"},
			{gofetch.MethodOptions, "OPTIONS"},
			{gofetch.MethodPatch, "PATCH"},
			{gofetch.MethodTrace, "TRACE"},
		}

		for _, m := range methods {
			Expect(string(m.constant)).To(Equal(m.expected))
		}
	})

	It("should support functional request options", func() {
		// Create request with options
		req := gofetch.NewRequestWithOptions("GET", "http://example.com",
			gofetch.WithHeader("X-Test", "value"),
			gofetch.WithQueryParam("q", "search"),
		)

		// Build HTTP request to verify options were applied
		httpReq, err := req.BuildHTTPRequest()
		Expect(err).NotTo(HaveOccurred())

		Expect(httpReq.Header.Get("X-Test")).To(Equal("value"))
		Expect(httpReq.URL.Query().Get("q")).To(Equal("search"))
	})

	It("should support method-specific request constructors", func() {
		constructors := []struct {
			name     string
			factory  func(string, ...gofetch.RequestOption) *gofetch.Request
			expected string
		}{
			{"NewGetRequest", gofetch.NewGetRequest, "GET"},
			{"NewPostRequest", gofetch.NewPostRequest, "POST"},
			{"NewPutRequest", gofetch.NewPutRequest, "PUT"},
			{"NewDeleteRequest", gofetch.NewDeleteRequest, "DELETE"},
			{"NewPatchRequest", gofetch.NewPatchRequest, "PATCH"},
		}

		for _, c := range constructors {
			req := c.factory("http://example.com")
			httpReq, err := req.BuildHTTPRequest()
			Expect(err).NotTo(HaveOccurred())
			Expect(httpReq.Method).To(Equal(c.expected))
		}
	})

	It("should support JSON request constructor", func() {
		data := map[string]interface{}{
			"name":  "test",
			"value": 42,
		}

		req := gofetch.NewJSONRequest("POST", "http://example.com", data)

		// Build HTTP request to verify
		httpReq, err := req.BuildHTTPRequest()
		Expect(err).NotTo(HaveOccurred())

		// Verify method and Content-Type
		Expect(httpReq.Method).To(Equal("POST"))
		Expect(httpReq.Header.Get("Content-Type")).To(Equal("application/json"))

		// Verify body contains JSON data
		body, err := io.ReadAll(httpReq.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).To(ContainSubstring(`"name":"test"`))
		Expect(string(body)).To(ContainSubstring(`"value":42`))
	})

	It("should support webhook request constructor", func() {
		payload := map[string]interface{}{
			"event": "update",
			"data":  "test data",
		}
		signature := "sha256=abcdef1234567890"

		req := gofetch.NewWebhookRequest("http://example.com/webhook", payload, signature)

		// Build HTTP request to verify
		httpReq, err := req.BuildHTTPRequest()
		Expect(err).NotTo(HaveOccurred())

		// Verify method, headers, and Content-Type
		Expect(httpReq.Method).To(Equal("POST"))
		Expect(httpReq.Header.Get("Content-Type")).To(Equal("application/json"))
		Expect(httpReq.Header.Get("X-Webhook-Signature")).To(Equal(signature))
		Expect(httpReq.Header.Get("User-Agent")).To(Equal("go-requests-webhook-client/1.0"))

		// Verify body contains JSON data
		body, err := io.ReadAll(httpReq.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).To(ContainSubstring(`"event":"update"`))
		Expect(string(body)).To(ContainSubstring(`"data":"test data"`))
	})

	It("should support various request options", func() {
		// Test server
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}))
		defer ts.Close()

		// Test WithHeaders
		headers := map[string]string{
			"X-Test-1": "value1",
			"X-Test-2": "value2",
		}
		req := gofetch.NewRequestWithOptions("GET", ts.URL,
			gofetch.WithHeaders(headers))

		httpReq, err := req.BuildHTTPRequest()
		Expect(err).NotTo(HaveOccurred())
		Expect(httpReq.Header.Get("X-Test-1")).To(Equal("value1"))
		Expect(httpReq.Header.Get("X-Test-2")).To(Equal("value2"))

		// Test WithBody
		body := []byte("test body data")
		req = gofetch.NewRequestWithOptions("POST", ts.URL,
			gofetch.WithBody(body))

		httpReq, err = req.BuildHTTPRequest()
		Expect(err).NotTo(HaveOccurred())
		bodyBytes, err := io.ReadAll(httpReq.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(bodyBytes).To(Equal(body))

		// Test WithChunkedEncoding
		req = gofetch.NewRequestWithOptions("POST", ts.URL,
			gofetch.WithBody([]byte("test")),
			gofetch.WithChunkedEncoding())

		httpReq, err = req.BuildHTTPRequest()
		Expect(err).NotTo(HaveOccurred())
		Expect(httpReq.Header.Get("Transfer-Encoding")).To(Equal("chunked"))
		Expect(httpReq.ContentLength).To(Equal(int64(-1))) // Indicates chunked encoding
	})

	It("should implement retry strategies", func() {
		// Test ConstantRetryStrategy
		constStrategy := &gofetch.ConstantRetryStrategy{
			Delay: 100 * time.Millisecond,
		}

		delay := constStrategy.NextDelay(2, nil)
		Expect(delay).To(Equal(100 * time.Millisecond))

		// Test ExponentialRetryStrategy
		expStrategy := &gofetch.ExponentialRetryStrategy{
			InitialDelay: 100 * time.Millisecond,
			MaxDelay:     5 * time.Second,
			Factor:       2.0,
		}

		delay = expStrategy.NextDelay(1, nil)
		Expect(delay).To(Equal(200 * time.Millisecond)) // 100ms * 2.0

		// Test max delay cap
		hugeStrategy := &gofetch.ExponentialRetryStrategy{
			InitialDelay: 1 * time.Second,
			MaxDelay:     2 * time.Second,
			Factor:       10.0, // Would result in 10s without cap
		}

		delay = hugeStrategy.NextDelay(1, nil)
		Expect(delay).To(Equal(2 * time.Second)) // Capped at MaxDelay
	})
})
