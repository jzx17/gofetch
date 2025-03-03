package gofetch

import (
	"errors"
	"fmt"
)

// ClientError represents an error that occurred during request execution.
type ClientError struct {
	Phase   string // "request", "transport", "response"
	Message string
	Cause   error
}

func (e *ClientError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Phase, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Phase, e.Message)
}

func (e *ClientError) Unwrap() error {
	return e.Cause
}

// NewRequestError creates a new error during request building.
func NewRequestError(msg string, cause error) error {
	return &ClientError{
		Phase:   "request",
		Message: msg,
		Cause:   cause,
	}
}

// NewTransportError creates a new error during request transport.
func NewTransportError(msg string, cause error) error {
	return &ClientError{
		Phase:   "transport",
		Message: msg,
		Cause:   cause,
	}
}

// NewResponseError creates a new error during response handling.
func NewResponseError(msg string, cause error) error {
	return &ClientError{
		Phase:   "response",
		Message: msg,
		Cause:   cause,
	}
}

// StatusError represents an error due to an unexpected HTTP status code.
type StatusError struct {
	StatusCode int
	Status     string
	URL        string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("unexpected status code %d (%s) for %s", e.StatusCode, e.Status, e.URL)
}

// NewStatusError creates a new error for unexpected status codes.
func NewStatusError(resp *Response) *StatusError {
	return &StatusError{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		URL:        resp.Request.URL.String(),
	}
}

// IsStatusError checks if an error is a StatusError with a specific code.
func IsStatusError(err error, code int) bool {
	var statusErr *StatusError
	if err == nil {
		return false
	}
	if ok := As(err, &statusErr); ok {
		return statusErr.StatusCode == code
	}
	return false
}

// As is a convenience wrapper around errors.As.
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

// NetworkErrorWrapper provides additional context for network errors.
type NetworkErrorWrapper struct {
	Operation string
	URL       string
	Cause     error
}

func (e *NetworkErrorWrapper) Error() string {
	return fmt.Sprintf("network error during %s to %s: %v", e.Operation, e.URL, e.Cause)
}

func (e *NetworkErrorWrapper) Unwrap() error {
	return e.Cause
}

// NewNetworkError wraps a network error with additional context.
func NewNetworkError(op string, url string, err error) error {
	return &NetworkErrorWrapper{
		Operation: op,
		URL:       url,
		Cause:     err,
	}
}
