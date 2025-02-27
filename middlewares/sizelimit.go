package middlewares

import (
	"fmt"
	"github.com/jzx17/gofetch/core"
	"io"
	"net/http"
)

var _ ConfigurableMiddleware = (*sizeValidationMiddleware)(nil)

type SizeError struct {
	Current int64
	Max     int64
	Type    string // "request", "response", or "stream"
}

func (e *SizeError) Error() string {
	return fmt.Sprintf("%s size %d exceeds the maximum size of %d", e.Type, e.Current, e.Max)
}

type sizeValidationMiddleware struct {
	BaseMiddleware
	config core.SizeConfig
}

func (m *sizeValidationMiddleware) Identity() MiddlewareIdentifier {
	return MiddlewareIdentifier{
		Name:    "size-validation",
		Options: m.config,
	}
}

// sizeLimitedReader tracks the number of bytes read and enforces size limits
type sizeLimitedReader struct {
	reader    io.Reader
	maxSize   int64
	bytesRead int64
	sizeType  string
}

func newSizeLimitedReader(reader io.Reader, maxSize int64, sizeType string) *sizeLimitedReader {
	return &sizeLimitedReader{
		reader:   reader,
		maxSize:  maxSize,
		sizeType: sizeType,
	}
}

func (r *sizeLimitedReader) Read(p []byte) (n int, err error) {
	if r.maxSize <= 0 {
		return r.reader.Read(p)
	}

	n, err = r.reader.Read(p)
	r.bytesRead += int64(n)
	if r.bytesRead > r.maxSize {
		return n, &SizeError{
			Current: r.bytesRead,
			Max:     r.maxSize,
			Type:    r.sizeType,
		}
	}
	return n, err
}

// SizeValidationMiddleware creates a middleware that validates request and response sizes
func SizeValidationMiddleware(config core.SizeConfig) ConfigurableMiddleware {
	wrapper := func(next core.RoundTripFunc) core.RoundTripFunc {
		return func(req *http.Request) (*http.Response, error) {
			// Check request body size if limit is set
			if req.Body != nil && config.MaxRequestBodySize > 0 {
				// Check content length directly if available
				if req.ContentLength > 0 && req.ContentLength > config.MaxRequestBodySize {
					return nil, &SizeError{
						Type:    "request",
						Max:     config.MaxRequestBodySize,
						Current: req.ContentLength,
					}
				}
				req.Body = io.NopCloser(newSizeLimitedReader(req.Body, config.MaxRequestBodySize, "request"))
			}

			// Make the actual request
			resp, err := next(req)
			if err != nil {
				return nil, err
			}

			// Check response body size if limit is set
			if resp.Body != nil && (config.MaxResponseBodySize > 0 || config.MaxStreamSize > 0) {
				maxSize := config.MaxResponseBodySize
				if maxSize == 0 {
					maxSize = config.MaxStreamSize
				}

				// Check content length directly if available
				if resp.ContentLength > 0 && resp.ContentLength > maxSize {
					_ = resp.Body.Close() // Prevent resource leak
					return nil, &SizeError{
						Type:    "response",
						Max:     maxSize,
						Current: resp.ContentLength,
					}
				}

				resp.Body = io.NopCloser(newSizeLimitedReader(resp.Body, maxSize, "response"))
			}

			return resp, nil
		}
	}

	return &sizeValidationMiddleware{
		BaseMiddleware: BaseMiddleware{
			Identifier: MiddlewareIdentifier{
				Name:    "size-validation",
				Options: config,
			},
			Wrapper: wrapper,
		},
		config: config,
	}
}
