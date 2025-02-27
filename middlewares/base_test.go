package middlewares_test

import (
	"io"
	"net/http"
	"strings"

	"github.com/jzx17/gofetch/core"
	"github.com/jzx17/gofetch/middlewares"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Middleware", func() {

	Describe("ChainMiddlewares", func() {
		It("should chain middleware in the correct order", func() {
			// Create two middleware that append letters to a header "X-Test".
			mw1 := middlewares.CreateMiddleware(
				"test-mw-1",
				nil,
				func(next core.RoundTripFunc) core.RoundTripFunc {
					return func(req *http.Request) (*http.Response, error) {
						prev := req.Header.Get("X-Test")
						req.Header.Set("X-Test", prev+"A")
						return next(req)
					}
				},
			)
			mw2 := middlewares.CreateMiddleware(
				"test-mw-2",
				nil,
				func(next core.RoundTripFunc) core.RoundTripFunc {
					return func(req *http.Request) (*http.Response, error) {
						prev := req.Header.Get("X-Test")
						req.Header.Set("X-Test", prev+"B")
						return next(req)
					}
				},
			)
			// Final function returns a response with header "X-Test" copied from the request.
			final := core.RoundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 200,
					Header:     http.Header{"X-Test": {req.Header.Get("X-Test")}},
					Body:       io.NopCloser(strings.NewReader("ok")),
				}, nil
			})

			chained := middlewares.ChainMiddlewares(final, mw1, mw2)

			req, err := http.NewRequest("GET", "http://example.com", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("X-Test", "")

			resp, err := chained(req)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Header.Get("X-Test")).To(Equal("AB"))
		})
	})
})
