package gofetch_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/jzx17/gofetch"
	"github.com/jzx17/gofetch/core"
	"io"
	"net/http"
	"net/url"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Error Handling", func() {
	It("should create and use RequestError", func() {
		// Create a RequestError
		originalErr := fmt.Errorf("original error")
		reqErr := gofetch.NewRequestError("test operation", originalErr)

		// Check error message format
		Expect(reqErr.Error()).To(ContainSubstring("request"))
		Expect(reqErr.Error()).To(ContainSubstring("test operation"))
		Expect(reqErr.Error()).To(ContainSubstring("original error"))

		// Check error unwrapping
		Expect(errors.Unwrap(reqErr)).To(Equal(originalErr))
	})

	It("should create and use TransportError", func() {
		// Create a TransportError
		originalErr := fmt.Errorf("connection refused")
		transportErr := gofetch.NewTransportError("send request", originalErr)

		// Check error message format
		Expect(transportErr.Error()).To(ContainSubstring("transport"))
		Expect(transportErr.Error()).To(ContainSubstring("send request"))
		Expect(transportErr.Error()).To(ContainSubstring("connection refused"))

		// Check error unwrapping
		Expect(errors.Unwrap(transportErr)).To(Equal(originalErr))
	})

	It("should create and use ResponseError", func() {
		// Create a ResponseError
		originalErr := fmt.Errorf("read EOF")
		respErr := gofetch.NewResponseError("read body", originalErr)

		// Check error message format
		Expect(respErr.Error()).To(ContainSubstring("response"))
		Expect(respErr.Error()).To(ContainSubstring("read body"))
		Expect(respErr.Error()).To(ContainSubstring("read EOF"))

		// Check error unwrapping
		Expect(errors.Unwrap(respErr)).To(Equal(originalErr))
	})

	It("should create and use StatusError", func() {
		// Create a Response with a non-2xx status code
		resp := &gofetch.Response{
			Response: &http.Response{
				StatusCode: 404,
				Status:     "404 Not Found",
				Request: &http.Request{
					URL: &url.URL{
						Scheme: "https",
						Host:   "example.com",
						Path:   "/test",
					},
				},
			},
		}

		// Create StatusError
		statusErr := gofetch.NewStatusError(resp)

		// Check error message format
		Expect(statusErr.Error()).To(ContainSubstring("404"))
		Expect(statusErr.Error()).To(ContainSubstring("Not Found"))
		Expect(statusErr.Error()).To(ContainSubstring("https://example.com/test"))

		// Check error fields
		Expect(statusErr.StatusCode).To(Equal(404))
		Expect(statusErr.Status).To(Equal("404 Not Found"))
		Expect(statusErr.URL).To(Equal("https://example.com/test"))
	})

	It("should create and use NetworkErrorWrapper", func() {
		// Create a NetworkErrorWrapper
		originalErr := fmt.Errorf("DNS lookup failed")
		netErr := gofetch.NewNetworkError("DNS resolution", "https://example.com", originalErr)

		// Check error message format
		Expect(netErr.Error()).To(ContainSubstring("network error"))
		Expect(netErr.Error()).To(ContainSubstring("DNS resolution"))
		Expect(netErr.Error()).To(ContainSubstring("https://example.com"))
		Expect(netErr.Error()).To(ContainSubstring("DNS lookup failed"))

		// Check error unwrapping
		Expect(errors.Unwrap(netErr)).To(Equal(originalErr))
	})

	It("should use IsStatusError to check status code", func() {
		// Create a StatusError with code 404
		statusErr := &gofetch.StatusError{
			StatusCode: 404,
			Status:     "404 Not Found",
			URL:        "https://example.com/test",
		}

		// Check with IsStatusError
		matchesCorrectCode := gofetch.IsStatusError(statusErr, 404)
		matchesWrongCode := gofetch.IsStatusError(statusErr, 500)

		Expect(matchesCorrectCode).To(BeTrue())
		Expect(matchesWrongCode).To(BeFalse())

		// Check with nil error
		matchesNilError := gofetch.IsStatusError(nil, 404)
		Expect(matchesNilError).To(BeFalse())
	})

	It("should properly handle error from Do method", func() {
		// Create a transport that returns an error
		errorTransport := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("connection timeout")
		})

		client := gofetch.NewClient(gofetch.WithTransport(errorTransport))

		// Make a request that will fail
		req := core.NewRequest("GET", "http://example.com")
		_, err := client.Do(context.Background(), req)

		// Verify error is wrapped correctly
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("transport"))
		Expect(err.Error()).To(ContainSubstring("connection timeout"))
	})

	It("should properly handle error from invalid URL", func() {
		client := gofetch.NewClient()

		// Make a request with an invalid URL
		req := core.NewRequest("GET", "")
		_, err := client.Do(context.Background(), req)

		// Verify error is wrapped correctly
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("request"))
		Expect(err.Error()).To(ContainSubstring("URL"))
	})

	It("should properly handle error during response body read", func() {
		// Create a transport that returns a response with a body that errors
		errorBodyTransport := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Status:     "200 OK",
				Body:       io.NopCloser(&errorReader{err: fmt.Errorf("read failed")}),
			}, nil
		})

		client := gofetch.NewClient(
			gofetch.WithTransport(errorBodyTransport),
			gofetch.WithAutoBufferResponse(true), // Enable auto-buffering to trigger the read
		)

		// Make a request
		req := core.NewRequest("GET", "http://example.com")
		_, err := client.Do(context.Background(), req)

		// Verify error is wrapped correctly
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("response"))
		Expect(err.Error()).To(ContainSubstring("read failed"))
	})
})

// errorReader is a helper that returns an error when read
type errorReader struct {
	err error
}

func (r *errorReader) Read(p []byte) (int, error) {
	return 0, r.err
}
