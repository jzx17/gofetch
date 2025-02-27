package gofetch

import "fmt"

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
