package middlewares

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jzx17/gofetch/core"
)

// LogLevel defines the logging verbosity
type LogLevel int

const (
	// LogLevelNone disables logging
	LogLevelNone LogLevel = iota
	// LogLevelError logs only errors
	LogLevelError
	// LogLevelInfo logs requests and responses
	LogLevelInfo
	// LogLevelDebug logs detailed information including headers
	LogLevelDebug
)

// LoggingOptions configures the logging middleware
type LoggingOptions struct {
	// Level controls the verbosity of logging
	Level LogLevel
	// Writer is where logs are written (defaults to os.Stderr)
	Writer io.Writer
	// RequestBodyMaxLen is the maximum number of request body bytes to log (0 = don't log body)
	RequestBodyMaxLen int
	// ResponseBodyMaxLen is the maximum number of response body bytes to log (0 = don't log body)
	ResponseBodyMaxLen int
	// HeadersToRedact are headers whose values will be redacted in logs
	HeadersToRedact []string
	// TimestampFormat controls the format of timestamps in logs
	TimestampFormat string
	// LogFormat controls how logs are formatted
	LogFormat LogFormat
}

// LogFormat defines the format of logs
type LogFormat int

const (
	// LogFormatText outputs logs as plain text
	LogFormatText LogFormat = iota
	// LogFormatJSON outputs logs as JSON
	LogFormatJSON
)

// DefaultLoggingOptions returns default logging options
func DefaultLoggingOptions() LoggingOptions {
	return LoggingOptions{
		Level:              LogLevelInfo,
		Writer:             os.Stderr,
		RequestBodyMaxLen:  0,
		ResponseBodyMaxLen: 0,
		HeadersToRedact:    []string{"Authorization", "Cookie", "Set-Cookie"},
		TimestampFormat:    time.RFC3339,
		LogFormat:          LogFormatText,
	}
}

// LoggingMiddleware creates a middleware that logs requests and responses
func LoggingMiddleware(options LoggingOptions) ConfigurableMiddleware {
	if options.Writer == nil {
		options.Writer = os.Stderr
	}
	if options.TimestampFormat == "" {
		options.TimestampFormat = time.RFC3339
	}

	wrapper := func(next core.RoundTripFunc) core.RoundTripFunc {
		return func(req *http.Request) (*http.Response, error) {
			if options.Level == LogLevelNone {
				return next(req)
			}

			start := time.Now()

			// Read and buffer request body if needed for logging
			var reqBodyCopy []byte
			if options.RequestBodyMaxLen > 0 && req.Body != nil && req.GetBody != nil {
				bodyReader, err := req.GetBody()
				if err == nil {
					reqBodyCopy, err = io.ReadAll(io.LimitReader(bodyReader, int64(options.RequestBodyMaxLen+1)))
					if err != nil {
						// If we can't read the body, log the error but continue
						_, _ = fmt.Fprintf(options.Writer, "Error reading request body: %v\n", err)
					}

					// Always attempt to close the reader
					closeErr := bodyReader.Close()
					if closeErr != nil {
						_, _ = fmt.Fprintf(options.Writer, "Error closing request body reader: %v\n", closeErr)
					}
				}
			}

			// Log request
			if options.Level >= LogLevelInfo {
				logRequest(req, reqBodyCopy, options)
			}

			// Execute request
			resp, err := next(req)

			// Calculate duration
			duration := time.Since(start)

			// Log response or error
			if err != nil {
				if options.Level >= LogLevelError {
					logError(req, err, duration, options)
				}
			} else if options.Level >= LogLevelInfo {
				// Optionally read and buffer response body for logging
				var respBodyCopy []byte
				if options.ResponseBodyMaxLen > 0 && resp.Body != nil {
					// Save the original body
					bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, int64(options.ResponseBodyMaxLen+1)))
					if err != nil {
						_, _ = fmt.Fprintf(options.Writer, "Error reading response body: %v\n", err)
						// Fall back to empty bytes but continue
						bodyBytes = []byte{}
					}

					closeErr := resp.Body.Close()
					if closeErr != nil {
						_, _ = fmt.Fprintf(options.Writer, "Error closing response body: %v\n", closeErr)
					}

					// Create a new ReadCloser for the response
					respBodyCopy = bodyBytes
					resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				}

				logResponse(req, resp, respBodyCopy, duration, options)
			}

			return resp, err
		}
	}

	return CreateMiddleware("logging", options, wrapper)
}

// logRequest logs HTTP request details
func logRequest(req *http.Request, body []byte, options LoggingOptions) {
	timestamp := time.Now().Format(options.TimestampFormat)

	if options.LogFormat == LogFormatText {
		_, err := fmt.Fprintf(options.Writer, "[%s] → Request: %s %s\n", timestamp, req.Method, req.URL)
		if err != nil {
			// If we can't write logs, print to stderr as fallback
			_, _ = fmt.Fprintf(os.Stderr, "Error writing to log: %v\n", err)
		}

		if options.Level >= LogLevelDebug {
			// Log headers with redaction
			_, err := fmt.Fprintln(options.Writer, "  Headers:")
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Error writing log headers: %v\n", err)
				return
			}

			for key, values := range req.Header {
				redacted := isHeaderRedacted(key, options.HeadersToRedact)

				if redacted {
					_, err := fmt.Fprintf(options.Writer, "    %s: [REDACTED]\n", key)
					if err != nil {
						_, _ = fmt.Fprintf(os.Stderr, "Error writing redacted header: %v\n", err)
						return
					}
				} else {
					for _, value := range values {
						_, err := fmt.Fprintf(options.Writer, "    %s: %s\n", key, value)
						if err != nil {
							_, _ = fmt.Fprintf(os.Stderr, "Error writing header value: %v\n", err)
							return
						}
					}
				}
			}

			// Log body if available
			if len(body) > 0 {
				truncated := len(body) > options.RequestBodyMaxLen
				if truncated {
					body = body[:options.RequestBodyMaxLen]
				}

				_, err := fmt.Fprintln(options.Writer, "  Body:")
				if err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "Error writing body header: %v\n", err)
					return
				}

				truncatedMsg := ""
				if truncated {
					truncatedMsg = "... [truncated]"
				}

				_, err = fmt.Fprintf(options.Writer, "    %s%s\n", string(body), truncatedMsg)
				if err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "Error writing body content: %v\n", err)
				}
			}
		}
	} else if options.LogFormat == LogFormatJSON {
		// Create log entry as JSON
		entry := map[string]interface{}{
			"timestamp": timestamp,
			"type":      "request",
			"method":    req.Method,
			"url":       req.URL.String(),
		}

		if options.Level >= LogLevelDebug {
			// Add headers with redaction
			headers := make(map[string]interface{})
			for key, values := range req.Header {
				if isHeaderRedacted(key, options.HeadersToRedact) {
					headers[key] = "[REDACTED]"
				} else if len(values) == 1 {
					headers[key] = values[0]
				} else {
					headers[key] = values
				}
			}
			entry["headers"] = headers

			// Add body if available
			if len(body) > 0 {
				truncated := len(body) > options.RequestBodyMaxLen
				if truncated {
					body = body[:options.RequestBodyMaxLen]
					entry["body_truncated"] = true
				}

				// Try to parse JSON body
				var jsonBody interface{}
				if err := json.Unmarshal(body, &jsonBody); err == nil {
					entry["body"] = jsonBody
				} else {
					entry["body"] = string(body)
				}
			}
		}

		// Output JSON log entry
		jsonData, err := json.Marshal(entry)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error marshaling JSON log entry: %v\n", err)
			return
		}

		_, err = fmt.Fprintln(options.Writer, string(jsonData))
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error writing JSON log: %v\n", err)
		}
	}
}

// logResponse logs HTTP response details
func logResponse(req *http.Request, resp *http.Response, body []byte, duration time.Duration, options LoggingOptions) {
	timestamp := time.Now().Format(options.TimestampFormat)

	if options.LogFormat == LogFormatText {
		_, err := fmt.Fprintf(options.Writer, "[%s] ← Response: %s %s → %d %s (%s)\n",
			timestamp, req.Method, req.URL, resp.StatusCode, resp.Status, duration)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error writing response log: %v\n", err)
			return
		}

		if options.Level >= LogLevelDebug {
			// Log headers with redaction
			_, _ = fmt.Fprintln(options.Writer, "  Headers:")
			for key, values := range resp.Header {
				redacted := isHeaderRedacted(key, options.HeadersToRedact)

				if redacted {
					_, _ = fmt.Fprintf(options.Writer, "    %s: [REDACTED]\n", key)
				} else {
					for _, value := range values {
						_, _ = fmt.Fprintf(options.Writer, "    %s: %s\n", key, value)
					}
				}
			}

			// Log body if available
			if len(body) > 0 {
				truncated := len(body) > options.ResponseBodyMaxLen
				if truncated {
					body = body[:options.ResponseBodyMaxLen]
				}

				_, _ = fmt.Fprintln(options.Writer, "  Body:")
				truncatedMsg := ""
				if truncated {
					truncatedMsg = "... [truncated]"
				}
				_, _ = fmt.Fprintf(options.Writer, "    %s%s\n", string(body), truncatedMsg)
			}
		}
	} else if options.LogFormat == LogFormatJSON {
		// Create log entry as JSON
		entry := map[string]interface{}{
			"timestamp":   timestamp,
			"type":        "response",
			"method":      req.Method,
			"url":         req.URL.String(),
			"status_code": resp.StatusCode,
			"status":      resp.Status,
			"duration_ms": duration.Milliseconds(),
		}

		if options.Level >= LogLevelDebug {
			// Add headers with redaction
			headers := make(map[string]interface{})
			for key, values := range resp.Header {
				if isHeaderRedacted(key, options.HeadersToRedact) {
					headers[key] = "[REDACTED]"
				} else if len(values) == 1 {
					headers[key] = values[0]
				} else {
					headers[key] = values
				}
			}
			entry["headers"] = headers

			// Add body if available
			if len(body) > 0 {
				truncated := len(body) > options.ResponseBodyMaxLen
				if truncated {
					body = body[:options.ResponseBodyMaxLen]
					entry["body_truncated"] = true
				}

				// Try to parse JSON body
				var jsonBody interface{}
				if err := json.Unmarshal(body, &jsonBody); err == nil {
					entry["body"] = jsonBody
				} else {
					entry["body"] = string(body)
				}
			}
		}

		// Output JSON log entry
		jsonData, err := json.Marshal(entry)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error marshaling JSON error log: %v\n", err)
			return
		}

		_, err = fmt.Fprintln(options.Writer, string(jsonData))
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error writing JSON error log: %v\n", err)
		}
	}
}

// logError logs error details
func logError(req *http.Request, err error, duration time.Duration, options LoggingOptions) {
	timestamp := time.Now().Format(options.TimestampFormat)

	if options.LogFormat == LogFormatText {
		_, writeErr := fmt.Fprintf(options.Writer, "[%s] ✗ Error: %s %s → %v (%s)\n",
			timestamp, req.Method, req.URL, err, duration)
		if writeErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error writing error log: %v\n", writeErr)
			return
		}
	} else if options.LogFormat == LogFormatJSON {
		// Create log entry as JSON
		entry := map[string]interface{}{
			"timestamp":   timestamp,
			"type":        "error",
			"method":      req.Method,
			"url":         req.URL.String(),
			"error":       err.Error(),
			"duration_ms": duration.Milliseconds(),
		}

		// Output JSON log entry
		jsonData, _ := json.Marshal(entry)
		_, _ = fmt.Fprintln(options.Writer, string(jsonData))
	}
}

// isHeaderRedacted checks if a header should be redacted
func isHeaderRedacted(header string, redactHeaders []string) bool {
	header = strings.ToLower(header)
	for _, h := range redactHeaders {
		if strings.ToLower(h) == header {
			return true
		}
	}
	return false
}

// NewLoggingMiddleware creates a logging middleware with custom options
func NewLoggingMiddleware(level LogLevel) ConfigurableMiddleware {
	options := DefaultLoggingOptions()
	options.Level = level
	return LoggingMiddleware(options)
}

// WithLogWriter sets the writer for log output
func WithLogWriter(writer io.Writer) func(*LoggingOptions) {
	return func(o *LoggingOptions) {
		o.Writer = writer
	}
}

// WithLogFormat sets the format for log output
func WithLogFormat(format LogFormat) func(*LoggingOptions) {
	return func(o *LoggingOptions) {
		o.LogFormat = format
	}
}

// WithHeadersToRedact sets headers to redact in logs
func WithHeadersToRedact(headers ...string) func(*LoggingOptions) {
	return func(o *LoggingOptions) {
		o.HeadersToRedact = headers
	}
}

// WithBodyLogging enables body logging with max lengths
func WithBodyLogging(reqMax, respMax int) func(*LoggingOptions) {
	return func(o *LoggingOptions) {
		o.RequestBodyMaxLen = reqMax
		o.ResponseBodyMaxLen = respMax
	}
}

// ConfigureLoggingMiddleware creates a logging middleware with custom options
func ConfigureLoggingMiddleware(optFuncs ...func(*LoggingOptions)) ConfigurableMiddleware {
	options := DefaultLoggingOptions()
	for _, fn := range optFuncs {
		fn(&options)
	}
	return LoggingMiddleware(options)
}
