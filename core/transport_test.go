// transport_test.go
package core_test

import (
	"crypto/tls"
	"fmt"
	"github.com/jzx17/gofetch/core"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TLSTransport", func() {
	var (
		tlsTrans *core.TLSTransport
		server   *httptest.Server
	)

	BeforeEach(func() {
		// Create a test server with TLS.
		server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("hello https"))
		}))
		// Create a TLSTransport with the server's TLS config.
		var err error
		tlsTrans, err = core.NewTLSTransport(
			true,
			server.Client().Transport.(*http.Transport).TLSClientConfig,
			5*time.Second,
			10,
			30*time.Second,
		)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		server.Close()
	})

	It("should successfully perform an HTTPS request using the TLSTransport", func() {
		req, err := http.NewRequest("GET", server.URL, nil)
		Expect(err).NotTo(HaveOccurred())
		resp, err := tlsTrans.RoundTrip(req)
		Expect(err).NotTo(HaveOccurred())
		body, err := io.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(body)).To(Equal("hello https"))
	})

	It("should set default TLS min version when tlsConfig is nil", func() {
		tlsTrans, err := core.NewTLSTransport(false, nil, 5*time.Second, 10, 30*time.Second)
		Expect(err).NotTo(HaveOccurred())
		Expect(tlsTrans.Transport.TLSClientConfig).NotTo(BeNil())
		Expect(tlsTrans.Transport.TLSClientConfig.MinVersion).To(Equal(uint16(tls.VersionTLS13)))
	})

	It("should set default min version if provided tlsConfig has zero MinVersion", func() {
		customTLS := &tls.Config{} // MinVersion is zero.
		tlsTrans, err := core.NewTLSTransport(false, customTLS, 5*time.Second, 10, 30*time.Second)
		Expect(err).NotTo(HaveOccurred())
		Expect(tlsTrans.Transport.TLSClientConfig.MinVersion).To(Equal(uint16(tls.VersionTLS13)))
	})

	It("should preserve provided TLS min version if set", func() {
		customTLS := &tls.Config{
			MinVersion: tls.VersionTLS13,
		}
		tlsTrans, err := core.NewTLSTransport(false, customTLS, 5*time.Second, 10, 30*time.Second)
		Expect(err).NotTo(HaveOccurred())
		Expect(tlsTrans.Transport.TLSClientConfig.MinVersion).To(Equal(uint16(tls.VersionTLS13)))
	})

	Describe("RoundTripFunc", func() {
		It("should call the provided function and return its response", func() {
			rt := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader("round trip")),
				}, nil
			})
			req, err := http.NewRequest("GET", "http://example.com", nil)
			Expect(err).NotTo(HaveOccurred())
			resp, err := rt.RoundTrip(req)
			Expect(err).NotTo(HaveOccurred())
			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(body)).To(Equal("round trip"))
		})

		It("should return error if the provided function returns an error", func() {
			expectedErr := fmt.Errorf("test error")
			rt := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return nil, expectedErr
			})
			req, err := http.NewRequest("GET", "http://example.com", nil)
			Expect(err).NotTo(HaveOccurred())
			_, err = rt.RoundTrip(req)
			Expect(err).To(Equal(expectedErr))
		})
	})

	It("should return error if underlying transport fails due to bad proxy", func() {
		// Setup a bad proxy that returns a URL with an unreachable port.
		badProxy := func(req *http.Request) (*url.URL, error) {
			return url.Parse("http://127.0.0.1:0")
		}
		customTransport := &http.Transport{
			Proxy:           badProxy,
			TLSClientConfig: (&tls.Config{InsecureSkipVerify: true}),
		}
		tlsTrans := &core.TLSTransport{Transport: customTransport}
		// Use a known valid URL; the proxy should force an error.
		req, err := http.NewRequest("GET", "https://example.com", nil)
		Expect(err).NotTo(HaveOccurred())
		_, err = tlsTrans.RoundTrip(req)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("connect"))
	})
})
