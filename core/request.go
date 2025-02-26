package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
)

// SizeConfig holds all size-related configuration parameters
type SizeConfig struct {
	MaxRequestBodySize  int64
	MaxResponseBodySize int64
	MaxStreamSize       int64
}

// DefaultSizeConfig returns a SizeConfig with default values
func DefaultSizeConfig() SizeConfig {
	return SizeConfig{
		MaxRequestBodySize:  10 * 1024 * 1024, // 10MB
		MaxResponseBodySize: 10 * 1024 * 1024, // 10MB
		MaxStreamSize:       10 * 1024 * 1024, // 10MB
	}
}

// WithRequestBodySize returns a new SizeConfig with updated request body size
// Panics if size is negative.
func (c SizeConfig) WithRequestBodySize(size int64) SizeConfig {
	if size < 0 {
		panic("RequestBodySize must be greater than or equal to 0")
	}
	c.MaxRequestBodySize = size
	return c
}

// WithResponseBodySize returns a new SizeConfig with updated response body size
// Panics if size is negative.
func (c SizeConfig) WithResponseBodySize(size int64) SizeConfig {
	if size < 0 {
		panic("ResponseBodySize must be greater than or equal to 0")
	}
	c.MaxResponseBodySize = size
	return c
}

// WithStreamSize returns a new SizeConfig with updated stream size
// Panics if size is negative.
func (c SizeConfig) WithStreamSize(size int64) SizeConfig {
	if size < 0 {
		panic("StreamSize must be greater than or equal to 0")
	}
	c.MaxStreamSize = size
	return c
}

// Request represents an API request with configurable headers, query parameters, and body.
type Request struct {
	method      string
	url         string
	headers     map[string]string
	queryParams url.Values
	body        io.Reader
	bodySize    int64
	// Indicates whether the body was built as multipart.
	isMultipart bool
	// Holds any error encountered during body building.
	buildErr error
}

var byteBufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 4096))
	},
}

func getBuffer() *bytes.Buffer {
	return byteBufferPool.Get().(*bytes.Buffer)
}

func putBuffer(buf *bytes.Buffer) {
	buf.Reset()
	byteBufferPool.Put(buf)
}

// NewRequest creates a new Request for the given method and URL.
func NewRequest(method, urlStr string) *Request {
	if _, err := url.ParseRequestURI(urlStr); err != nil {
		return &Request{
			buildErr: fmt.Errorf("invalid URL: %w", err),
		}
	}

	return &Request{
		method:      method,
		url:         urlStr,
		headers:     make(map[string]string),
		queryParams: url.Values{},
	}
}

// Clone creates a deep copy of the Request
func (r *Request) Clone() *Request {
	clone := &Request{
		method:      r.method,
		url:         r.url,
		headers:     make(map[string]string),
		queryParams: url.Values{},
		isMultipart: r.isMultipart,
		buildErr:    r.buildErr,
	}

	// Copy headers
	for k, v := range r.headers {
		clone.headers[k] = v
	}

	// Copy query params
	for k, values := range r.queryParams {
		for _, v := range values {
			clone.queryParams.Add(k, v)
		}
	}

	// Copy body if present
	if r.body != nil {
		// This handles only byte slices, not arbitrary readers
		// For full support, bodies should always be re-addable
		if buf, ok := r.body.(*bytes.Reader); ok {
			size := buf.Size()
			data := make([]byte, size)
			_, _ = buf.ReadAt(data, 0)
			_, _ = buf.Seek(0, io.SeekStart) // Reset original reader
			clone.body = bytes.NewReader(data)
			clone.bodySize = size
		}
	}

	return clone
}

// WithHeader adds a single header to the Request.
func (r *Request) WithHeader(key, value string) *Request {
	if r.headers == nil {
		r.headers = make(map[string]string)
	}

	r.headers[key] = value

	return r
}

// WithHeaders adds multiple headers to the Request.
func (r *Request) WithHeaders(headers map[string]string) *Request {
	if r.headers == nil {
		r.headers = make(map[string]string)
	}

	for k, v := range headers {
		r.headers[k] = v
	}

	return r
}

// WithQueryParam adds a query parameter to the Request.
func (r *Request) WithQueryParam(key, value string) *Request {
	r.queryParams.Add(key, value)

	return r
}

// WithQueryParams adds multiple query parameters to the Request.
func (r *Request) WithQueryParams(params map[string]string) *Request {
	for k, v := range params {
		r.queryParams.Add(k, v)
	}
	return r
}

// WithBody sets the request body from a byte slice.
func (r *Request) WithBody(body []byte) *Request {
	if r.method == http.MethodGet || r.method == http.MethodHead {
		r.buildErr = fmt.Errorf("http method %s does not allow a body", r.method)
		return r
	}

	r.body = bytes.NewReader(body)
	r.bodySize = int64(len(body))

	return r
}

// WithChunkedEncoding sets the Transfer-Encoding header to chunk.
func (r *Request) WithChunkedEncoding() *Request {
	r.headers["Transfer-Encoding"] = "chunked"

	return r
}

// WithJSONBody sets the request body to the JSON representation of the provided data
// and sets the Content-Type header to application/json.
func (r *Request) WithJSONBody(data interface{}) *Request {
	b, err := json.Marshal(data)
	if err != nil {
		r.buildErr = err
		return r
	}
	r.body = bytes.NewReader(b)
	r.bodySize = int64(len(b))
	r.WithHeader("Content-Type", "application/json")

	return r
}

// WithMultipartForm constructs a multipart/form-data body from formFields and fileFields.
func (r *Request) WithMultipartForm(formFields map[string]string, fileFields map[string]string) *Request {
	buf := getBuffer()

	writer := multipart.NewWriter(buf)

	for key, val := range formFields {
		if err := writer.WriteField(key, val); err != nil {
			r.buildErr = fmt.Errorf("failed to write form field %s: %w", key, err)
			putBuffer(buf)
			return r
		}
	}

	for field, filePath := range fileFields {
		if err := func() (retErr error) {
			file, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("failed to open file %s: %w", filePath, err)
			}
			defer func() {
				if closeErr := file.Close(); closeErr != nil && retErr == nil {
					retErr = fmt.Errorf("failed to close file %s: %w", filePath, closeErr)
				}
			}()

			part, err := writer.CreateFormFile(field, filepath.Base(filePath))
			if err != nil {
				return fmt.Errorf("failed to create form file for field %s: %w", field, err)
			}
			if _, err := io.Copy(part, file); err != nil {
				return fmt.Errorf("failed to copy file %s: %w", filePath, err)
			}
			return nil
		}(); err != nil {
			r.buildErr = err
			putBuffer(buf)
			return r
		}
	}

	if err := writer.Close(); err != nil {
		r.buildErr = err
		putBuffer(buf)
		return r
	}

	data := buf.Bytes()
	putBuffer(buf)

	r.body = bytes.NewReader(data)
	r.bodySize = int64(len(data))
	r.isMultipart = true
	r.WithHeader("Content-Type", writer.FormDataContentType())

	return r
}

// BuildHTTPRequest constructs an *http.Request from the Request.
func (r *Request) BuildHTTPRequest() (*http.Request, error) {
	if r.buildErr != nil {
		return nil, r.buildErr
	}
	if r.url == "" {
		return nil, fmt.Errorf("failed to build HTTP request: request URL is empty")
	}

	parsedURL, err := url.Parse(r.url)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	q := parsedURL.Query()
	for key, values := range r.queryParams {
		for _, v := range values {
			q.Add(key, v)
		}
	}
	parsedURL.RawQuery = q.Encode()

	httpReq, err := http.NewRequest(r.method, parsedURL.String(), r.body)
	if err != nil {
		return nil, fmt.Errorf("failed to create new HTTP request: %w", err)
	}

	for key, value := range r.headers {
		httpReq.Header.Set(key, value)
	}

	// Ensure chunked encoding is correctly applied
	if r.headers["Transfer-Encoding"] == "chunked" {
		httpReq.ContentLength = -1
	} else if r.bodySize > 0 {
		httpReq.ContentLength = r.bodySize
	}

	return httpReq, nil
}

// BuildHTTPRequestWithContext constructs an *http.Request with context from the Request.
func (r *Request) BuildHTTPRequestWithContext(ctx context.Context) (*http.Request, error) {
	req, err := r.BuildHTTPRequest()
	if err != nil {
		return nil, err
	}
	return req.WithContext(ctx), nil
}
