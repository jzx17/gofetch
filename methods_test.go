package gofetch_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/jzx17/gofetch"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Convenience Methods", func() {
	var (
		testServer *httptest.Server
		client     *gofetch.Client
		ctx        context.Context
	)

	BeforeEach(func() {
		testServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check for header passing
			if headerVal := r.Header.Get("X-Test-Header"); headerVal != "" {
				w.Header().Set("X-Test-Response", headerVal)
			}

			switch r.Method {
			case http.MethodGet:
				_, _ = fmt.Fprint(w, "GET response")
			case http.MethodPost:
				body, _ := io.ReadAll(r.Body)
				_, _ = fmt.Fprintf(w, "POST response: %s", string(body))
			case http.MethodPut:
				body, _ := io.ReadAll(r.Body)
				_, _ = fmt.Fprintf(w, "PUT response: %s", string(body))
			case http.MethodDelete:
				_, _ = fmt.Fprint(w, "DELETE response")
			case http.MethodPatch:
				body, _ := io.ReadAll(r.Body)
				_, _ = fmt.Fprintf(w, "PATCH response: %s", string(body))
			case http.MethodHead:
				// HEAD should have empty body
				w.Header().Set("X-Test", "HEAD test")
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
			}

		}))

		client = gofetch.NewClient()
		ctx = context.Background()
	})

	AfterEach(func() {
		testServer.Close()
	})

	It("should perform GET convenience method", func() {
		headers := map[string]string{"X-Test-Header": "test-value"}
		resp, err := client.Get(ctx, testServer.URL, headers)

		Expect(err).NotTo(HaveOccurred())
		Expect(resp).NotTo(BeNil())

		// Check header was passed
		Expect(resp.Header.Get("X-Test-Response")).To(Equal("test-value"))

		// Check body
		body, err := resp.Bytes()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).To(Equal("GET response"))
	})

	It("should perform POST convenience method", func() {
		headers := map[string]string{"X-Test-Header": "test-value"}
		body := []byte("hello world")
		resp, err := client.Post(ctx, testServer.URL, body, headers)

		Expect(err).NotTo(HaveOccurred())
		Expect(resp).NotTo(BeNil())

		// Check header was passed
		Expect(resp.Header.Get("X-Test-Response")).To(Equal("test-value"))

		// Check body
		responseBody, err := resp.Bytes()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(responseBody)).To(Equal("POST response: hello world"))
	})

	It("should perform PUT convenience method", func() {
		headers := map[string]string{"X-Test-Header": "test-value"}
		body := []byte("update data")
		resp, err := client.Put(ctx, testServer.URL, body, headers)

		Expect(err).NotTo(HaveOccurred())
		Expect(resp).NotTo(BeNil())

		// Check header was passed
		Expect(resp.Header.Get("X-Test-Response")).To(Equal("test-value"))

		// Check body
		responseBody, err := resp.Bytes()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(responseBody)).To(Equal("PUT response: update data"))
	})

	It("should perform DELETE convenience method", func() {
		headers := map[string]string{"X-Test-Header": "test-value"}
		resp, err := client.Delete(ctx, testServer.URL, headers)

		Expect(err).NotTo(HaveOccurred())
		Expect(resp).NotTo(BeNil())

		// Check header was passed
		Expect(resp.Header.Get("X-Test-Response")).To(Equal("test-value"))

		// Check body
		responseBody, err := resp.Bytes()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(responseBody)).To(Equal("DELETE response"))
	})

	It("should perform PATCH convenience method", func() {
		headers := map[string]string{"X-Test-Header": "test-value"}
		body := []byte("patch data")
		resp, err := client.Patch(ctx, testServer.URL, body, headers)

		Expect(err).NotTo(HaveOccurred())
		Expect(resp).NotTo(BeNil())

		// Check header was passed
		Expect(resp.Header.Get("X-Test-Response")).To(Equal("test-value"))

		// Check body
		responseBody, err := resp.Bytes()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(responseBody)).To(Equal("PATCH response: patch data"))
	})

	It("should perform HEAD convenience method", func() {
		headers := map[string]string{"X-Test-Header": "test-value"}
		resp, err := client.Head(ctx, testServer.URL, headers)

		Expect(err).NotTo(HaveOccurred())
		Expect(resp).NotTo(BeNil())

		// Check headers were set correctly
		Expect(resp.Header.Get("X-Test-Response")).To(Equal("test-value"))
		Expect(resp.Header.Get("X-Test")).To(Equal("HEAD test"))

		// HEAD should have empty body
		responseBody, err := resp.Bytes()
		Expect(err).NotTo(HaveOccurred())
		Expect(responseBody).To(BeEmpty())
	})

	It("should perform PostJSON convenience method", func() {
		headers := map[string]string{"X-Test-Header": "test-value"}
		data := map[string]interface{}{
			"message": "json data",
			"value":   42,
		}
		resp, err := client.PostJSON(ctx, testServer.URL, data, headers)

		Expect(err).NotTo(HaveOccurred())
		Expect(resp).NotTo(BeNil())

		// Check header was passed
		Expect(resp.Header.Get("X-Test-Response")).To(Equal("test-value"))

		// Check response contains the JSON data
		responseBody, err := resp.Bytes()
		Expect(err).NotTo(HaveOccurred())
		// Response will contain the JSON as a string in the body
		Expect(string(responseBody)).To(ContainSubstring("json data"))
		Expect(string(responseBody)).To(ContainSubstring("42"))
	})

	It("should perform PutJSON convenience method", func() {
		headers := map[string]string{"X-Test-Header": "test-value"}
		data := map[string]interface{}{
			"message": "json update",
			"value":   100,
		}
		resp, err := client.PutJSON(ctx, testServer.URL, data, headers)

		Expect(err).NotTo(HaveOccurred())
		Expect(resp).NotTo(BeNil())

		// Check header was passed
		Expect(resp.Header.Get("X-Test-Response")).To(Equal("test-value"))

		// Check response contains the JSON data
		responseBody, err := resp.Bytes()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(responseBody)).To(ContainSubstring("json update"))
		Expect(string(responseBody)).To(ContainSubstring("100"))
	})

	It("should perform PatchJSON convenience method", func() {
		headers := map[string]string{"X-Test-Header": "test-value"}
		data := map[string]interface{}{
			"message": "json patch",
			"value":   50,
		}
		resp, err := client.PatchJSON(ctx, testServer.URL, data, headers)

		Expect(err).NotTo(HaveOccurred())
		Expect(resp).NotTo(BeNil())

		// Check header was passed
		Expect(resp.Header.Get("X-Test-Response")).To(Equal("test-value"))

		// Check response contains the JSON data
		responseBody, err := resp.Bytes()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(responseBody)).To(ContainSubstring("json patch"))
		Expect(string(responseBody)).To(ContainSubstring("50"))
	})
})
