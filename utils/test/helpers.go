package test

import (
	"errors"
	"github.com/jzx17/gofetch/core"
	"github.com/jzx17/gofetch/middlewares"
	"io"
	"net/http"
)

// MockReadCloser is a configurable implementation of io.ReadCloser for testing
// that can simulate different error behaviors.
type MockReadCloser struct {
	// Content is the data that will be returned by Read
	Content []byte
	// Position tracks where we are in the content
	Position int
	// ReadBehavior determines how Read() operates
	ReadBehavior ReadBehaviorType
	// ReadError is the error returned when ReadBehavior is set to FailAlways or FailAfterFirstRead
	ReadError error
	// CloseError is the error returned by Close() if not nil
	CloseError error
	// SuccessfulReads counts the number of successful reads before failing
	// Used when ReadBehavior is set to FailAfterFirstRead or FailAfterNReads
	SuccessfulReads int
	// MaxReads is the number of successful reads before failing
	// Used when ReadBehavior is set to FailAfterNReads
	MaxReads int
}

// ReadBehaviorType defines how the MockReadCloser will behave when Read() is called
type ReadBehaviorType int

const (
	// Success means Read will succeed normally
	Success ReadBehaviorType = iota
	// FailAlways means Read will always return an error
	FailAlways
	// FailAfterFirstRead means Read will succeed once, then fail
	FailAfterFirstRead
	// FailAfterNReads means Read will succeed N times, then fail
	FailAfterNReads
)

// Read implements io.Reader
func (m *MockReadCloser) Read(p []byte) (int, error) {
	switch m.ReadBehavior {
	case FailAlways:
		if m.ReadError == nil {
			return 0, errors.New("read error")
		}
		return 0, m.ReadError

	case FailAfterFirstRead:
		if m.SuccessfulReads > 0 {
			if m.ReadError == nil {
				return 0, errors.New("read error after first success")
			}
			return 0, m.ReadError
		}
		m.SuccessfulReads++
		return m.readContent(p)

	case FailAfterNReads:
		if m.SuccessfulReads >= m.MaxReads {
			if m.ReadError == nil {
				return 0, errors.New("read error after max reads")
			}
			return 0, m.ReadError
		}
		m.SuccessfulReads++
		return m.readContent(p)

	default: // Success
		return m.readContent(p)
	}
}

// readContent reads from the internal content buffer
func (m *MockReadCloser) readContent(p []byte) (int, error) {
	// If no content or we've read it all, return EOF
	if m.Content == nil || m.Position >= len(m.Content) {
		return 0, io.EOF
	}

	// Copy data from content to buffer
	n := copy(p, m.Content[m.Position:])
	m.Position += n

	// Return EOF if we've read everything
	var err error
	if m.Position >= len(m.Content) {
		err = io.EOF
	}

	return n, err
}

// Close implements io.Closer
func (m *MockReadCloser) Close() error {
	return m.CloseError
}

// Helper constructor functions for common cases

// NewErrorReader creates a MockReadCloser that always fails on Read
func NewErrorReader(err error) *MockReadCloser {
	return &MockReadCloser{
		ReadBehavior: FailAlways,
		ReadError:    err,
	}
}

// NewErrorCloser creates a MockReadCloser that reads successfully but fails on Close
func NewErrorCloser(data []byte, err error) *MockReadCloser {
	return &MockReadCloser{
		Content:    data,
		CloseError: err,
	}
}

// NewErrorReadCloser creates a MockReadCloser that always fails on Read but closes successfully
func NewErrorReadCloser(err error) *MockReadCloser {
	return &MockReadCloser{
		ReadBehavior: FailAlways,
		ReadError:    err,
	}
}

// NewMockReadCloser creates a MockReadCloser with the provided content
func NewMockReadCloser(content []byte) *MockReadCloser {
	return &MockReadCloser{
		Content: content,
	}
}

// StreamErrorReader always returns an error after the first successful read.
type StreamErrorReader struct {
	called bool
	data   []byte
	err    error
}

func (r *StreamErrorReader) Read(p []byte) (int, error) {
	if !r.called {
		r.called = true
		n := copy(p, r.data)
		return n, nil
	}
	return 0, r.err
}

func (r *StreamErrorReader) Close() error {
	return nil
}

// NewStreamErrorReader creates a new StreamErrorReader that returns the provided data
// on first read, and then returns the provided error on subsequent reads.
func NewStreamErrorReader(data []byte, err error) *StreamErrorReader {
	return &StreamErrorReader{
		data: data,
		err:  err,
	}
}

// Helper function to create test middleware
func CreateTestMiddleware(name string, fn func(req *http.Request)) middlewares.ConfigurableMiddleware {
	return middlewares.CreateMiddleware(
		name,
		nil,
		func(next core.RoundTripFunc) core.RoundTripFunc {
			return func(req *http.Request) (*http.Response, error) {
				fn(req)
				return next(req)
			}
		},
	)
}
