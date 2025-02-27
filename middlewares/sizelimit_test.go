package middlewares_test

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/jzx17/gofetch/core"
	"github.com/jzx17/gofetch/middlewares"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SizeValidationMiddleware", func() {
	var config core.SizeConfig

	Describe("Request Body Limits", func() {
		BeforeEach(func() {
			// Set a small limit for request bodies.
			config = core.SizeConfig{
				MaxRequestBodySize: 10, // 10 bytes limit
			}
		})

		It("should return an error if the request body exceeds the limit", func() {
			// Create a request with a body that is 11 bytes long.
			body := "12345678901" // 11 bytes
			req, err := http.NewRequest("POST", "http://example.com", strings.NewReader(body))
			Expect(err).NotTo(HaveOccurred())

			// Dummy RoundTrip that reads the entire body.
			dummy := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
				_, err := io.ReadAll(req.Body)
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader("dummy response")),
				}, err
			})

			mw := middlewares.SizeValidationMiddleware(config)
			wrapped := mw.Wrap(dummy)
			_, err = wrapped(req)
			Expect(err).To(HaveOccurred())
			var sizeErr *middlewares.SizeError
			Expect(errors.As(err, &sizeErr)).To(BeTrue())
			Expect(sizeErr.Type).To(Equal("request"))
		})

		It("should allow the request body if within the limit", func() {
			body := "123456789" // 9 bytes
			req, err := http.NewRequest("POST", "http://example.com", strings.NewReader(body))
			Expect(err).NotTo(HaveOccurred())

			dummy := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
				data, err := io.ReadAll(req.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data)).To(Equal(body))
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader("dummy response")),
				}, nil
			})

			mw := middlewares.SizeValidationMiddleware(config)
			wrapped := mw.Wrap(dummy)
			resp, err := wrapped(req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))
		})

		It("should immediately error if ContentLength exceeds the limit", func() {
			// Even if the actual body is short, an artificially high ContentLength should trigger an error.
			body := "short" // 5 bytes actual
			req, err := http.NewRequest("POST", "http://example.com", strings.NewReader(body))
			Expect(err).NotTo(HaveOccurred())
			req.ContentLength = 20 // Exceeds our 10-byte limit

			dummy := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
				// Should not be reached.
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader("dummy response")),
				}, nil
			})

			mw := middlewares.SizeValidationMiddleware(config)
			wrapped := mw.Wrap(dummy)
			_, err = wrapped(req)
			Expect(err).To(HaveOccurred())
			var sizeErr *middlewares.SizeError
			Expect(errors.As(err, &sizeErr)).To(BeTrue())
			Expect(sizeErr.Type).To(Equal("request"))
			Expect(sizeErr.Max).To(Equal(int64(10)))
			Expect(sizeErr.Current).To(Equal(int64(20)))
		})

		It("should allow a request body exactly at the limit", func() {
			// Body is exactly 10 bytes.
			body := "1234567890"
			req, err := http.NewRequest("POST", "http://example.com", strings.NewReader(body))
			Expect(err).NotTo(HaveOccurred())

			dummy := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
				data, err := io.ReadAll(req.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data)).To(Equal(body))
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader("ok")),
				}, nil
			})
			mw := middlewares.SizeValidationMiddleware(config)
			wrapped := mw.Wrap(dummy)
			_, err = wrapped(req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not enforce any limit if RequestBodySize is zero", func() {
			config.MaxRequestBodySize = 0
			body := strings.Repeat("a", 100)
			req, err := http.NewRequest("POST", "http://example.com", strings.NewReader(body))
			Expect(err).NotTo(HaveOccurred())

			dummy := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
				data, err := io.ReadAll(req.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data)).To(Equal(body))
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader("ok")),
				}, nil
			})
			mw := middlewares.SizeValidationMiddleware(config)
			wrapped := mw.Wrap(dummy)
			_, err = wrapped(req)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Response Body Limits", func() {
		BeforeEach(func() {
			// Set a small limit for response bodies.
			config = core.SizeConfig{
				MaxResponseBodySize: 10, // 10 bytes limit
			}
		})

		It("should return an error if the response body exceeds the limit (by reading)", func() {
			req, err := http.NewRequest("GET", "http://example.com", nil)
			Expect(err).NotTo(HaveOccurred())

			// Dummy RoundTrip returns a response body of 11 bytes.
			responseBody := "12345678901" // 11 bytes
			dummy := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader(responseBody)),
				}, nil
			})

			mw := middlewares.SizeValidationMiddleware(config)
			wrapped := mw.Wrap(dummy)
			resp, err := wrapped(req)
			Expect(err).NotTo(HaveOccurred())

			// Reading the response should trigger a SizeError.
			_, err = io.ReadAll(resp.Body)
			Expect(err).To(HaveOccurred())
			var sizeErr *middlewares.SizeError
			Expect(errors.As(err, &sizeErr)).To(BeTrue())
			Expect(sizeErr.Type).To(Equal("response"))
		})

		It("should allow the response body if within the limit", func() {
			req, err := http.NewRequest("GET", "http://example.com", nil)
			Expect(err).NotTo(HaveOccurred())

			responseBody := "123456789" // 9 bytes
			dummy := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader(responseBody)),
				}, nil
			})

			mw := middlewares.SizeValidationMiddleware(config)
			wrapped := mw.Wrap(dummy)
			resp, err := wrapped(req)
			Expect(err).NotTo(HaveOccurred())
			data, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal(responseBody))
		})

		It("should immediately error if response ContentLength exceeds the limit", func() {
			req, err := http.NewRequest("GET", "http://example.com", nil)
			Expect(err).NotTo(HaveOccurred())

			// Even though the actual body is short, an excessive ContentLength should trigger error.
			responseBody := "short"
			dummy := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode:    200,
					Body:          io.NopCloser(strings.NewReader(responseBody)),
					ContentLength: 20, // Exceeds limit of 10 bytes.
				}, nil
			})
			mw := middlewares.SizeValidationMiddleware(config)
			wrapped := mw.Wrap(dummy)
			resp, err := wrapped(req)
			Expect(err).To(HaveOccurred())
			Expect(resp).To(BeNil())

			var sizeErr *middlewares.SizeError
			Expect(errors.As(err, &sizeErr)).To(BeTrue())
			Expect(sizeErr.Type).To(Equal("response"))
			Expect(sizeErr.Max).To(Equal(int64(10)))
			Expect(sizeErr.Current).To(Equal(int64(20)))
		})

		It("should use StreamSize if ResponseBodySize is zero", func() {
			// When ResponseBodySize is zero, StreamSize is used as the limit.
			config.MaxResponseBodySize = 0
			config.MaxStreamSize = 15
			req, err := http.NewRequest("GET", "http://example.com", nil)
			Expect(err).NotTo(HaveOccurred())

			responseBody := "123456789012345" // exactly 15 bytes.
			dummy := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode:    200,
					Body:          io.NopCloser(strings.NewReader(responseBody)),
					ContentLength: -1, // Unknown length; middleware will enforce limit while reading.
				}, nil
			})
			mw := middlewares.SizeValidationMiddleware(config)
			wrapped := mw.Wrap(dummy)
			resp, err := wrapped(req)
			Expect(err).NotTo(HaveOccurred())
			data, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal(responseBody))
		})

		It("should not enforce any limit if both ResponseBodySize and StreamSize are zero", func() {
			config.MaxResponseBodySize = 0
			config.MaxStreamSize = 0
			req, err := http.NewRequest("GET", "http://example.com", nil)
			Expect(err).NotTo(HaveOccurred())

			longResp := strings.Repeat("b", 1000)
			dummy := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode:    200,
					Body:          io.NopCloser(strings.NewReader(longResp)),
					ContentLength: int64(len(longResp)),
				}, nil
			})
			mw := middlewares.SizeValidationMiddleware(config)
			wrapped := mw.Wrap(dummy)
			resp, err := wrapped(req)
			Expect(err).NotTo(HaveOccurred())
			data, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal(longResp))
		})
	})

	Describe("Edge cases for nil body", func() {

		It("should pass through if request body is nil", func() {
			req, err := http.NewRequest("GET", "http://example.com", nil)
			Expect(err).NotTo(HaveOccurred())

			dummy := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 200,
					Body:       nil,
				}, nil
			})
			mw := middlewares.SizeValidationMiddleware(config)
			wrapped := mw.Wrap(dummy)
			resp, err := wrapped(req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))
		})

		It("should pass through if response body is nil", func() {
			req, err := http.NewRequest("GET", "http://example.com", nil)
			Expect(err).NotTo(HaveOccurred())

			dummy := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 200,
					Body:       nil,
				}, nil
			})
			mw := middlewares.SizeValidationMiddleware(config)
			wrapped := mw.Wrap(dummy)
			resp, err := wrapped(req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(200))
		})
	})
})
