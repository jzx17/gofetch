package core

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Response wraps a http.Response to provide helper methods.
type Response struct {
	*http.Response
	BytesRead int64
}

// CloseBody closes the response body.
func (r *Response) CloseBody() error {
	if r.Response == nil || r.Body == nil {
		return nil
	}
	return r.Body.Close()
}

// JSON decodes the JSON response into the provided variable.
func (r *Response) JSON(v interface{}) (err error) {
	defer func() {
		if closeErr := r.CloseBody(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close response body: %w", closeErr)
		}
	}()

	return json.NewDecoder(r.Body).Decode(v)
}

// XML decodes the XML response into the provided variable.
func (r *Response) XML(v interface{}) (err error) {
	defer func() {
		if closeErr := r.CloseBody(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close response body: %w", closeErr)
		}
	}()

	return xml.NewDecoder(r.Body).Decode(v)
}

// Bytes reads the full response body into a byte slice.
func (r *Response) Bytes() (body []byte, err error) {
	defer func() {
		if closeErr := r.CloseBody(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close response body: %w", closeErr)
		}
	}()

	return io.ReadAll(r.Body)
}

// String reads the full response body and returns it as a string.
func (r *Response) String() (body string, err error) {
	bytes, err := r.Bytes()
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// SaveToFile writes the response body to a file at the given path.
func (r *Response) SaveToFile(filePath string) (err error) {
	defer func() {
		if closeErr := r.CloseBody(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close response body: %w", closeErr)
		}
	}()

	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close file %s: %w", filePath, closeErr)
		}
	}()

	_, err = io.Copy(f, r.Body)
	if err != nil {
		return fmt.Errorf("failed to save response to file %s: %w", filePath, err)
	}

	return nil
}

// Process executes the given function on the response body and handles closing
func (r *Response) Process(fn func(io.Reader) error) error {
	if r.Response == nil {
		return fmt.Errorf("nil response")
	}

	defer r.CloseBody()
	return fn(r.Body)
}

// IsSuccess returns true if the status code is 2xx
func (r *Response) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// IsRedirect returns true if the status code is 3xx
func (r *Response) IsRedirect() bool {
	return r.StatusCode >= 300 && r.StatusCode < 400
}

// IsClientError returns true if the status code is 4xx
func (r *Response) IsClientError() bool {
	return r.StatusCode >= 400 && r.StatusCode < 500
}

// IsServerError returns true if the status code is 5xx
func (r *Response) IsServerError() bool {
	return r.StatusCode >= 500 && r.StatusCode < 600
}

// IsError returns true if the status code is 4xx or 5xx
func (r *Response) IsError() bool {
	return r.StatusCode >= 400
}

// MustSuccess returns the response if it's successful, otherwise returns an error
func (r *Response) MustSuccess() (*Response, error) {
	if !r.IsSuccess() {
		body, _ := r.String()
		return nil, fmt.Errorf("request failed with status %s: %s", r.Status, body)
	}
	return r, nil
}

type StreamOption func(*streamConfig)

type streamConfig struct {
	bufferSize int
}

func WithBufferSize(size int) StreamOption {
	return func(c *streamConfig) {
		if size > 0 {
			c.bufferSize = size
		}
	}
}

// StreamChunks reads the response body in chunks and passes each chunk to the callback.
func (r *Response) StreamChunks(callback func(chunk []byte), opts ...StreamOption) error {
	config := streamConfig{
		bufferSize: 4096,
	}

	for _, opt := range opts {
		opt(&config)
	}

	buf := make([]byte, config.bufferSize)
	for {
		n, err := r.Body.Read(buf)
		if n > 0 {
			r.BytesRead += int64(n)
			callback(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error while streaming chunks: %w", err)
		}
	}

	return nil
}

// StreamChunksWithContext reads the response body in chunks and respects context cancellation.
func (r *Response) StreamChunksWithContext(ctx context.Context, callback func(chunk []byte), opts ...StreamOption) error {
	config := streamConfig{
		bufferSize: 4096,
	}

	for _, opt := range opts {
		opt(&config)
	}

	buf := make([]byte, config.bufferSize)
	readChan := make(chan readResult, 1)

	for {
		go func() {
			n, err := r.Body.Read(buf)
			readChan <- readResult{n: n, err: err}
		}()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case result := <-readChan:
			if result.n > 0 {
				r.BytesRead += int64(result.n)
				callback(buf[:result.n])
			}
			if result.err == io.EOF {
				return nil
			}
			if result.err != nil {
				return fmt.Errorf("error while streaming chunks: %w", result.err)
			}
		}
	}
}

type readResult struct {
	n   int
	err error
}

// AsyncResponse represents the eventual outcome of an asynchronous HTTP call.
type AsyncResponse struct {
	Response *Response
	Error    error
}
