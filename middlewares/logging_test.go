package middlewares_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jzx17/gofetch/core"
	"github.com/jzx17/gofetch/middlewares"
)

var _ = Describe("Logging Middleware", func() {
	var (
		request        *http.Request
		responseWriter *httptest.ResponseRecorder
		logBuffer      *bytes.Buffer
		options        middlewares.LoggingOptions
		middleware     middlewares.ConfigurableMiddleware
		nextCalled     bool
		mockResponse   *http.Response
		mockError      error
	)

	// Helper to create a simple round trip function
	createMockRoundTripper := func(resp *http.Response, err error) core.RoundTripFunc {
		return func(req *http.Request) (*http.Response, error) {
			nextCalled = true
			return resp, err
		}
	}

	BeforeEach(func() {
		// Reset test variables
		logBuffer = new(bytes.Buffer)
		nextCalled = false
		mockError = nil

		// Create a test request
		var err error
		request, err = http.NewRequest("GET", "https://example.com/test", nil)
		Expect(err).NotTo(HaveOccurred())
		request.Header.Add("User-Agent", "test-agent")
		request.Header.Add("Authorization", "Bearer secret-token")

		// Create a test response
		responseWriter = httptest.NewRecorder()
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.WriteHeader(http.StatusOK)
		_, err = responseWriter.WriteString(`{"status":"ok"}`)
		Expect(err).NotTo(HaveOccurred())

		mockResponse = &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(bytes.NewBufferString(`{"status":"ok"}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}

		// Default options
		options = middlewares.DefaultLoggingOptions()
		options.Writer = logBuffer
	})

	Describe("With default settings", func() {
		BeforeEach(func() {
			middleware = middlewares.LoggingMiddleware(options)
		})

		It("should call the next middleware", func() {
			wrappedFunc := middleware.Wrap(createMockRoundTripper(mockResponse, nil))
			resp, err := wrappedFunc(request)

			Expect(nextCalled).To(BeTrue())
			Expect(err).To(BeNil())
			Expect(resp).To(Equal(mockResponse))
		})

		It("should log basic request and response info at INFO level", func() {
			options.Level = middlewares.LogLevelInfo
			middleware = middlewares.LoggingMiddleware(options)

			wrappedFunc := middleware.Wrap(createMockRoundTripper(mockResponse, nil))
			_, _ = wrappedFunc(request)

			logOutput := logBuffer.String()
			Expect(logOutput).To(ContainSubstring("→ Request: GET https://example.com/test"))
			Expect(logOutput).To(ContainSubstring("← Response: GET https://example.com/test → 200 200 OK"))

			// Should not log headers or body at INFO level
			Expect(logOutput).NotTo(ContainSubstring("Headers:"))
			Expect(logOutput).NotTo(ContainSubstring("Body:"))
		})
	})

	Describe("With different log levels", func() {
		It("should not log anything at NONE level", func() {
			options.Level = middlewares.LogLevelNone
			middleware = middlewares.LoggingMiddleware(options)

			wrappedFunc := middleware.Wrap(createMockRoundTripper(mockResponse, nil))
			_, _ = wrappedFunc(request)

			Expect(logBuffer.String()).To(BeEmpty())
		})

		It("should log only errors at ERROR level", func() {
			options.Level = middlewares.LogLevelError
			middleware = middlewares.LoggingMiddleware(options)
			mockError = errors.New("test error")

			wrappedFunc := middleware.Wrap(createMockRoundTripper(nil, mockError))
			_, err := wrappedFunc(request)

			Expect(err).To(Equal(mockError))
			logOutput := logBuffer.String()
			Expect(logOutput).To(ContainSubstring("✗ Error: GET https://example.com/test → test error"))

			// Reset and try with successful response
			logBuffer.Reset()
			wrappedFunc = middleware.Wrap(createMockRoundTripper(mockResponse, nil))
			_, _ = wrappedFunc(request)

			// Should not log successful requests at ERROR level
			Expect(logBuffer.String()).To(BeEmpty())
		})

		It("should log detailed information at DEBUG level", func() {
			options.Level = middlewares.LogLevelDebug
			middleware = middlewares.LoggingMiddleware(options)

			wrappedFunc := middleware.Wrap(createMockRoundTripper(mockResponse, nil))
			_, _ = wrappedFunc(request)

			logOutput := logBuffer.String()
			Expect(logOutput).To(ContainSubstring("Headers:"))
			Expect(logOutput).To(ContainSubstring("User-Agent: test-agent"))

			// Authorization header should be redacted
			Expect(logOutput).To(ContainSubstring("Authorization: [REDACTED]"))
			Expect(logOutput).NotTo(ContainSubstring("Bearer secret-token"))
		})
	})

	Describe("With body logging enabled", func() {
		BeforeEach(func() {
			options.Level = middlewares.LogLevelDebug
			options.RequestBodyMaxLen = 1024
			options.ResponseBodyMaxLen = 1024
			middleware = middlewares.LoggingMiddleware(options)
		})

		It("should log request body", func() {
			bodyContent := `{"test":"value"}`
			request, _ = http.NewRequest("POST", "https://example.com/test",
				strings.NewReader(bodyContent))
			request.GetBody = func() (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader(bodyContent)), nil
			}

			wrappedFunc := middleware.Wrap(createMockRoundTripper(mockResponse, nil))
			_, _ = wrappedFunc(request)

			logOutput := logBuffer.String()
			Expect(logOutput).To(ContainSubstring("Body:"))
			Expect(logOutput).To(ContainSubstring(`{"test":"value"}`))
		})

		It("should log response body", func() {
			wrappedFunc := middleware.Wrap(createMockRoundTripper(mockResponse, nil))
			_, _ = wrappedFunc(request)

			logOutput := logBuffer.String()
			Expect(logOutput).To(ContainSubstring(`{"status":"ok"}`))
		})

		It("should truncate large bodies", func() {
			// Create a large body
			largeBody := strings.Repeat("X", 2000)

			// Set a small max length
			options.ResponseBodyMaxLen = 100
			middleware = middlewares.LoggingMiddleware(options)

			mockResponse.Body = io.NopCloser(strings.NewReader(largeBody))

			wrappedFunc := middleware.Wrap(createMockRoundTripper(mockResponse, nil))
			_, _ = wrappedFunc(request)

			logOutput := logBuffer.String()
			Expect(logOutput).To(ContainSubstring("[truncated]"))
			Expect(logOutput).NotTo(ContainSubstring(largeBody))
			Expect(strings.Count(logOutput, "X")).To(Equal(100))
		})
	})

	Describe("JSON format logging", func() {
		BeforeEach(func() {
			options.Level = middlewares.LogLevelDebug
			options.LogFormat = middlewares.LogFormatJSON
			options.RequestBodyMaxLen = 1024
			options.ResponseBodyMaxLen = 1024
			middleware = middlewares.LoggingMiddleware(options)
		})

		It("should output valid JSON logs", func() {
			wrappedFunc := middleware.Wrap(createMockRoundTripper(mockResponse, nil))
			_, _ = wrappedFunc(request)

			lines := strings.Split(strings.TrimSpace(logBuffer.String()), "\n")
			Expect(len(lines)).To(Equal(2)) // Request + Response log entries

			// Verify both lines are valid JSON
			for _, line := range lines {
				var jsonData map[string]interface{}
				err := json.Unmarshal([]byte(line), &jsonData)
				Expect(err).NotTo(HaveOccurred())

				// Check common fields
				Expect(jsonData).To(HaveKey("timestamp"))
				Expect(jsonData).To(HaveKey("type"))
				Expect(jsonData).To(HaveKey("url"))
				Expect(jsonData).To(HaveKey("method"))

				// Check type-specific fields
				switch jsonData["type"] {
				case "request":
					Expect(jsonData).To(HaveKey("headers"))
					headers := jsonData["headers"].(map[string]interface{})
					Expect(headers).To(HaveKey("User-Agent"))
					Expect(headers).To(HaveKey("Authorization"))
					Expect(headers["Authorization"]).To(Equal("[REDACTED]"))

				case "response":
					Expect(jsonData).To(HaveKey("status"))
					Expect(jsonData).To(HaveKey("status_code"))
					Expect(jsonData).To(HaveKey("duration_ms"))
					Expect(jsonData).To(HaveKey("headers"))
					Expect(jsonData).To(HaveKey("body"))
				}
			}
		})

		It("should handle JSON body parsing", func() {
			mockResponse.Body = io.NopCloser(bytes.NewBufferString(`{"key":"value"}`))

			wrappedFunc := middleware.Wrap(createMockRoundTripper(mockResponse, nil))
			_, _ = wrappedFunc(request)

			lines := strings.Split(strings.TrimSpace(logBuffer.String()), "\n")

			// Get response line
			var responseLine map[string]interface{}
			err := json.Unmarshal([]byte(lines[1]), &responseLine)
			Expect(err).NotTo(HaveOccurred())

			// Verify body is parsed as JSON object, not string
			body := responseLine["body"].(map[string]interface{})
			Expect(body).To(HaveKey("key"))
			Expect(body["key"]).To(Equal("value"))
		})

		It("should log errors in JSON format", func() {
			mockError = errors.New("test error")

			wrappedFunc := middleware.Wrap(createMockRoundTripper(nil, mockError))
			_, _ = wrappedFunc(request)

			// Split the buffer into lines since we might have multiple JSON objects
			lines := strings.Split(strings.TrimSpace(logBuffer.String()), "\n")

			// Find the error line - it should be the only line or the last line
			var errorLine string
			for _, line := range lines {
				if strings.Contains(line, "test error") {
					errorLine = line
					break
				}
			}

			Expect(errorLine).NotTo(BeEmpty(), "Error log line not found")

			var errorLog map[string]interface{}
			err := json.Unmarshal([]byte(errorLine), &errorLog)
			Expect(err).NotTo(HaveOccurred(), "Failed to parse error JSON: "+errorLine)

			Expect(errorLog["type"]).To(Equal("error"))
			Expect(errorLog["error"]).To(Equal("test error"))
			Expect(errorLog["method"]).To(Equal("GET"))
			Expect(errorLog["url"]).To(Equal("https://example.com/test"))
		})
	})

	Describe("Using helper functions", func() {
		It("should create middleware with NewLoggingMiddleware", func() {
			middleware = middlewares.NewLoggingMiddleware(middlewares.LogLevelDebug)
			identifier := middleware.GetIdentifier()
			Expect(identifier.Name).To(Equal("logging"))

			options, ok := identifier.Options.(middlewares.LoggingOptions)
			Expect(ok).To(BeTrue())
			Expect(options.Level).To(Equal(middlewares.LogLevelDebug))
		})

		It("should create middleware with functional options", func() {
			customHeaders := []string{"X-Custom", "X-Token"}

			middleware = middlewares.ConfigureLoggingMiddleware(
				middlewares.WithLogFormat(middlewares.LogFormatJSON),
				middlewares.WithLogWriter(logBuffer),
				middlewares.WithHeadersToRedact(customHeaders...),
				middlewares.WithBodyLogging(512, 512),
			)

			identifier := middleware.GetIdentifier()
			options, ok := identifier.Options.(middlewares.LoggingOptions)
			Expect(ok).To(BeTrue())

			Expect(options.LogFormat).To(Equal(middlewares.LogFormatJSON))
			Expect(options.Writer).To(Equal(logBuffer))
			Expect(options.HeadersToRedact).To(Equal(customHeaders))
			Expect(options.RequestBodyMaxLen).To(Equal(512))
			Expect(options.ResponseBodyMaxLen).To(Equal(512))
		})
	})

	Describe("Error handling", func() {
		It("should handle request with nil body", func() {
			request.Body = nil
			request.GetBody = nil

			wrappedFunc := middleware.Wrap(createMockRoundTripper(mockResponse, nil))
			_, err := wrappedFunc(request)

			Expect(err).NotTo(HaveOccurred())
			// Should not panic
		})

		It("should handle response with nil body", func() {
			options.ResponseBodyMaxLen = 1024
			middleware = middlewares.LoggingMiddleware(options)

			mockResponse.Body = nil

			wrappedFunc := middleware.Wrap(createMockRoundTripper(mockResponse, nil))
			_, err := wrappedFunc(request)

			Expect(err).NotTo(HaveOccurred())
			// Should not panic
		})

		It("should handle case where GetBody returns error", func() {
			request.GetBody = func() (io.ReadCloser, error) {
				return nil, errors.New("cannot get body")
			}

			wrappedFunc := middleware.Wrap(createMockRoundTripper(mockResponse, nil))
			_, err := wrappedFunc(request)

			Expect(err).NotTo(HaveOccurred())
			// Should continue and not panic
		})
	})
})
