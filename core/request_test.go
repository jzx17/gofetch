package core_test

import (
	"context"
	"encoding/json"
	"github.com/jzx17/gofetch/core"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"io"
	"os"
	"time"
)

var _ = Describe("Request", func() {
	It("should set headers correctly", func() {
		req := core.NewRequest("GET", "http://example.com")
		req.WithHeader("X-Test", "value")
		req.WithHeaders(map[string]string{"X-Test-2": "value2"})
		httpReq, err := req.BuildHTTPRequest()
		Expect(err).NotTo(HaveOccurred())
		Expect(httpReq.Header.Get("X-Test")).To(Equal("value"))
		Expect(httpReq.Header.Get("X-Test-2")).To(Equal("value2"))
	})

	It("should add query parameters", func() {
		req := core.NewRequest("GET", "http://example.com")
		req.WithQueryParam("foo", "bar")
		httpReq, err := req.BuildHTTPRequest()
		Expect(err).NotTo(HaveOccurred())
		Expect(httpReq.URL.RawQuery).To(ContainSubstring("foo=bar"))
	})

	It("should add multiple query parameters", func() {
		req := core.NewRequest("GET", "http://example.com")
		req.WithQueryParams(map[string]string{
			"foo": "bar",
			"baz": "qux",
		})
		httpReq, err := req.BuildHTTPRequest()
		Expect(err).NotTo(HaveOccurred())
		Expect(httpReq.URL.RawQuery).To(ContainSubstring("foo=bar"))
		Expect(httpReq.URL.RawQuery).To(ContainSubstring("baz=qux"))
	})

	It("should set the body correctly with WithBody", func() {
		data := []byte("hello")
		req := core.NewRequest("POST", "http://example.com")
		req.WithBody(data)
		httpReq, err := req.BuildHTTPRequest()
		Expect(err).NotTo(HaveOccurred())
		body, err := io.ReadAll(httpReq.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).To(Equal("hello"))
	})

	It("should set JSON body and content-type header", func() {
		payload := map[string]string{"key": "value"}
		req := core.NewRequest("POST", "http://example.com")
		req.WithJSONBody(payload)
		httpReq, err := req.BuildHTTPRequest()
		Expect(err).NotTo(HaveOccurred())
		Expect(httpReq.Header.Get("Content-Type")).To(Equal("application/json"))
		body, err := io.ReadAll(httpReq.Body)
		Expect(err).NotTo(HaveOccurred())
		var parsed map[string]string
		err = json.Unmarshal(body, &parsed)
		Expect(err).NotTo(HaveOccurred())
		Expect(parsed).To(Equal(payload))
	})

	It("should return an error for empty URL", func() {
		req := core.NewRequest("GET", "")
		_, err := req.BuildHTTPRequest()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("invalid URL: parse \"\": empty url"))
	})

	It("should return an error for an invalid URL", func() {
		req := core.NewRequest("GET", "http://%41:8080/")
		_, err := req.BuildHTTPRequest()
		Expect(err).To(HaveOccurred())
	})

	It("should error when setting a body for GET requests", func() {
		req := core.NewRequest("GET", "http://example.com")
		req.WithBody([]byte("data"))
		_, err := req.BuildHTTPRequest()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(MatchRegexp("http method \\w+ does not allow a body"))
	})

	It("should set Transfer-Encoding header to chunked with WithChunkedEncoding", func() {
		req := core.NewRequest("POST", "http://example.com")
		req.WithChunkedEncoding()
		httpReq, err := req.BuildHTTPRequest()
		Expect(err).NotTo(HaveOccurred())
		Expect(httpReq.Header.Get("Transfer-Encoding")).To(Equal("chunked"))
	})

	It("should error if JSON marshalling fails", func() {
		// channels cannot be marshalled to JSON.
		badData := make(chan int)
		req := core.NewRequest("POST", "http://example.com")
		req.WithJSONBody(badData)
		_, err := req.BuildHTTPRequest()
		Expect(err).To(HaveOccurred())
	})

	It("should build HTTP request with context", func() {
		req := core.NewRequest("GET", "http://example.com")
		ctx := context.Background()
		httpReq, err := req.BuildHTTPRequestWithContext(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(httpReq.Context()).To(Equal(ctx))
	})

	It("should clone a request correctly", func() {
		original := core.NewRequest("POST", "http://example.com")
		original.WithHeader("X-Test", "value")
		original.WithQueryParam("foo", "bar")
		original.WithBody([]byte("test body"))

		clone := original.Clone()

		// Build both requests
		originalReq, err := original.BuildHTTPRequest()
		Expect(err).NotTo(HaveOccurred())
		cloneReq, err := clone.BuildHTTPRequest()
		Expect(err).NotTo(HaveOccurred())

		// Headers should match
		Expect(cloneReq.Header.Get("X-Test")).To(Equal(originalReq.Header.Get("X-Test")))

		// Query params should match
		Expect(cloneReq.URL.RawQuery).To(Equal(originalReq.URL.RawQuery))

		// Read bodies to compare
		originalBody, err := io.ReadAll(originalReq.Body)
		Expect(err).NotTo(HaveOccurred())
		cloneBody, err := io.ReadAll(cloneReq.Body)
		Expect(err).NotTo(HaveOccurred())

		Expect(string(cloneBody)).To(Equal(string(originalBody)))
	})

	// Add tests for the new methods
	It("should set and use context correctly", func() {
		req := core.NewRequest("GET", "http://example.com")

		// Create a context with a timeout
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Set the context
		req = req.WithContext(ctx)

		// Build the request with the context
		httpReq, err := req.BuildHTTPRequest()
		Expect(err).NotTo(HaveOccurred())

		// The deadline from our context should be present in the request's context
		deadline, hasDeadline := httpReq.Context().Deadline()
		Expect(hasDeadline).To(BeTrue())
		Expect(deadline).To(BeTemporally("~", time.Now().Add(100*time.Millisecond), 50*time.Millisecond))
	})

	It("should handle nil context gracefully", func() {
		req := core.NewRequest("GET", "http://example.com")
		req = req.WithContext(nil)

		httpReq, err := req.BuildHTTPRequest()
		Expect(err).NotTo(HaveOccurred())
		Expect(httpReq.Context()).NotTo(BeNil())
	})

	It("should set and get metadata correctly", func() {
		req := core.NewRequest("GET", "http://example.com")

		// Test with different types of values
		req.SetMetadata("string", "test value")
		req.SetMetadata("int", 42)
		req.SetMetadata("bool", true)
		req.SetMetadata("struct", struct{ Name string }{"Test"})

		// Test retrieval
		Expect(req.GetMetadata("string")).To(Equal("test value"))
		Expect(req.GetMetadata("int")).To(Equal(42))
		Expect(req.GetMetadata("bool")).To(Equal(true))
		Expect(req.GetMetadata("struct")).To(Equal(struct{ Name string }{"Test"}))

		// Test non-existent key
		Expect(req.GetMetadata("nonexistent")).To(BeNil())
	})

	It("should preserve metadata when cloning", func() {
		original := core.NewRequest("GET", "http://example.com")
		original.SetMetadata("key", "value")

		clone := original.Clone()
		Expect(clone.GetMetadata("key")).To(Equal("value"))

		// Modifying clone's metadata should not affect original
		clone.SetMetadata("key", "new value")
		Expect(original.GetMetadata("key")).To(Equal("value"))
		Expect(clone.GetMetadata("key")).To(Equal("new value"))
	})

	It("should preserve context when cloning", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		original := core.NewRequest("GET", "http://example.com").WithContext(ctx)
		clone := original.Clone()

		originalReq, err := original.BuildHTTPRequest()
		Expect(err).NotTo(HaveOccurred())
		cloneReq, err := clone.BuildHTTPRequest()
		Expect(err).NotTo(HaveOccurred())

		// Both requests should have the same context
		Expect(cloneReq.Context()).To(Equal(originalReq.Context()))
	})

	Context("SizeConfig", func() {
		It("should set default size configurations", func() {
			config := core.DefaultSizeConfig()
			Expect(config.MaxRequestBodySize).To(Equal(int64(10 * 1024 * 1024)))  // 10MB
			Expect(config.MaxResponseBodySize).To(Equal(int64(10 * 1024 * 1024))) // 10MB
			Expect(config.MaxStreamSize).To(Equal(int64(10 * 1024 * 1024)))       // 10MB
		})

		It("should modify request body size", func() {
			config := core.DefaultSizeConfig()
			newConfig := config.WithRequestBodySize(20 * 1024 * 1024) // 20MB
			Expect(newConfig.MaxRequestBodySize).To(Equal(int64(20 * 1024 * 1024)))
			// Other values should remain unchanged
			Expect(newConfig.MaxResponseBodySize).To(Equal(config.MaxResponseBodySize))
			Expect(newConfig.MaxStreamSize).To(Equal(config.MaxStreamSize))
		})

		It("should modify response body size", func() {
			config := core.DefaultSizeConfig()
			newConfig := config.WithResponseBodySize(20 * 1024 * 1024) // 20MB
			Expect(newConfig.MaxResponseBodySize).To(Equal(int64(20 * 1024 * 1024)))
			// Other values should remain unchanged
			Expect(newConfig.MaxRequestBodySize).To(Equal(config.MaxRequestBodySize))
			Expect(newConfig.MaxStreamSize).To(Equal(config.MaxStreamSize))
		})

		It("should modify stream size", func() {
			config := core.DefaultSizeConfig()
			newConfig := config.WithStreamSize(20 * 1024 * 1024) // 20MB
			Expect(newConfig.MaxStreamSize).To(Equal(int64(20 * 1024 * 1024)))
			// Other values should remain unchanged
			Expect(newConfig.MaxRequestBodySize).To(Equal(config.MaxRequestBodySize))
			Expect(newConfig.MaxResponseBodySize).To(Equal(config.MaxResponseBodySize))
		})

		It("should panic when setting a negative request body size", func() {
			config := core.DefaultSizeConfig()
			Expect(func() {
				config.WithRequestBodySize(-1)
			}).To(Panic())
		})

		It("should panic when setting a negative response body size", func() {
			config := core.DefaultSizeConfig()
			Expect(func() {
				config.WithResponseBodySize(-1)
			}).To(Panic())
		})

		It("should panic when setting a negative stream size", func() {
			config := core.DefaultSizeConfig()
			Expect(func() {
				config.WithStreamSize(-1)
			}).To(Panic())
		})
	})

	Context("Multipart Form", func() {
		var (
			tmpFile  *os.File
			filePath string
		)

		BeforeEach(func() {
			var err error
			tmpFile, err = os.CreateTemp("", "testfile")
			Expect(err).NotTo(HaveOccurred())
			filePath = tmpFile.Name()
			_, err = tmpFile.WriteString("file content")
			Expect(err).NotTo(HaveOccurred())
			tmpFile.Close()
		})

		AfterEach(func() {
			os.Remove(filePath)
		})

		It("should build a multipart form request", func() {
			formFields := map[string]string{"field1": "value1"}
			fileFields := map[string]string{"file1": filePath}
			req := core.NewRequest("POST", "http://example.com")
			req.WithMultipartForm(formFields, fileFields)
			httpReq, err := req.BuildHTTPRequest()
			Expect(err).NotTo(HaveOccurred())
			// Check that Content-Type is multipart/form-data with a boundary.
			contentType := httpReq.Header.Get("Content-Type")
			Expect(contentType).To(ContainSubstring("multipart/form-data"))
			body, err := io.ReadAll(httpReq.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(body)).To(ContainSubstring("value1"))
			Expect(string(body)).To(ContainSubstring("file content"))
		})

		It("should error if a file does not exist", func() {
			formFields := map[string]string{"field1": "value1"}
			fileFields := map[string]string{"file1": "nonexistent_file.txt"}
			req := core.NewRequest("POST", "http://example.com")
			req.WithMultipartForm(formFields, fileFields)
			_, err := req.BuildHTTPRequest()
			Expect(err).To(HaveOccurred())
		})
	})
})
