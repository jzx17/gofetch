package gofetch

import (
	"github.com/jzx17/gofetch/core"
	"github.com/jzx17/gofetch/middlewares"
	"time"
)

type Request = core.Request
type Response = core.Response
type AsyncResponse = core.AsyncResponse
type SizeConfig = core.SizeConfig
type StreamOption = core.StreamOption

var NewRequest = core.NewRequest
var DefaultSizeConfig = core.DefaultSizeConfig
var WithBufferSize = core.WithBufferSize

type RoundTripFunc = core.RoundTripFunc
type TLSTransport = core.TLSTransport
type ConfigurableMiddleware = middlewares.ConfigurableMiddleware
type MiddlewareIdentifier = middlewares.MiddlewareIdentifier
type Middleware = middlewares.Middleware

var NewTLSTransport = core.NewTLSTransport
var CreateMiddleware = middlewares.CreateMiddleware
var ChainMiddlewares = middlewares.ChainMiddlewares
var SizeValidationMiddleware = middlewares.SizeValidationMiddleware
var RetryMiddleware = middlewares.RetryMiddleware
var RateLimitMiddleware = middlewares.RateLimitMiddleware
var LoggingMiddleware = middlewares.LoggingMiddleware

type SizeError = middlewares.SizeError
type RetryError = middlewares.RetryError
type TimeoutError = middlewares.TimeoutError
type RateLimitExceededError = middlewares.RateLimitExceededError
type RateLimitOptions = middlewares.RateLimitOptions
type LoggingOptions = middlewares.LoggingOptions
type LogLevel = middlewares.LogLevel
type LogFormat = middlewares.LogFormat

// RequestMethod represents HTTP request methods
type RequestMethod string

// HTTP methods as constants
const (
	MethodGet     RequestMethod = "GET"
	MethodPost    RequestMethod = "POST"
	MethodPut     RequestMethod = "PUT"
	MethodDelete  RequestMethod = "DELETE"
	MethodHead    RequestMethod = "HEAD"
	MethodOptions RequestMethod = "OPTIONS"
	MethodPatch   RequestMethod = "PATCH"
	MethodTrace   RequestMethod = "TRACE"
)

// RequestOption is a function that configures a Request
type RequestOption func(*Request)

// WithHeader adds a header to the request
func WithHeader(key, value string) RequestOption {
	return func(r *Request) {
		r.WithHeader(key, value)
	}
}

// WithQueryParam adds a query parameter to the request
func WithQueryParam(key, value string) RequestOption {
	return func(r *Request) {
		r.WithQueryParam(key, value)
	}
}

// WithJSONBody sets a JSON body on the request
func WithJSONBody(data interface{}) RequestOption {
	return func(r *Request) {
		r.WithJSONBody(data)
	}
}

// WithBody sets a byte slice as the request body
func WithBody(body []byte) RequestOption {
	return func(r *Request) {
		r.WithBody(body)
	}
}

// WithHeaders adds multiple headers to the request
func WithHeaders(headers map[string]string) RequestOption {
	return func(r *Request) {
		r.WithHeaders(headers)
	}
}

// WithMultipartForm adds a multipart form to the request
func WithMultipartForm(fields map[string]string, files map[string]string) RequestOption {
	return func(r *Request) {
		r.WithMultipartForm(fields, files)
	}
}

// WithChunkedEncoding enables chunked transfer encoding
func WithChunkedEncoding() RequestOption {
	return func(r *Request) {
		r.WithChunkedEncoding()
	}
}

// NewRequestWithOptions creates a new request with the given options
func NewRequestWithOptions(method string, url string, opts ...RequestOption) *Request {
	req := NewRequest(method, url)
	for _, opt := range opts {
		opt(req)
	}
	return req
}

// NewGetRequest creates a new GET request
func NewGetRequest(url string, opts ...RequestOption) *Request {
	return NewRequestWithOptions("GET", url, opts...)
}

// NewPostRequest creates a new POST request
func NewPostRequest(url string, opts ...RequestOption) *Request {
	return NewRequestWithOptions("POST", url, opts...)
}

// NewPutRequest creates a new PUT request
func NewPutRequest(url string, opts ...RequestOption) *Request {
	return NewRequestWithOptions("PUT", url, opts...)
}

// NewDeleteRequest creates a new DELETE request
func NewDeleteRequest(url string, opts ...RequestOption) *Request {
	return NewRequestWithOptions("DELETE", url, opts...)
}

// NewPatchRequest creates a new PATCH request
func NewPatchRequest(url string, opts ...RequestOption) *Request {
	return NewRequestWithOptions("PATCH", url, opts...)
}

// NewJSONRequest creates a new request with JSON body
func NewJSONRequest(method, url string, data interface{}, opts ...RequestOption) *Request {
	opts = append([]RequestOption{WithJSONBody(data)}, opts...)
	return NewRequestWithOptions(method, url, opts...)
}

// NewWebhookRequest creates a customized request for webhook delivery
func NewWebhookRequest(url string, payload interface{}, signature string) *Request {
	return NewJSONRequest("POST", url, payload,
		WithHeader("X-Webhook-Signature", signature),
		WithHeader("User-Agent", "go-requests-webhook-client/1.0"))
}

// RetryStrategy defines how retry attempts are spaced
type RetryStrategy interface {
	// NextDelay returns the delay to wait for the next retry attempt
	NextDelay(attempt int, lastError error) time.Duration
}

// ConstantRetryStrategy implements a constant delay between retries
type ConstantRetryStrategy struct {
	Delay time.Duration
}

func (s *ConstantRetryStrategy) NextDelay(attempt int, lastError error) time.Duration {
	return s.Delay
}

// ExponentialRetryStrategy implements exponential backoff
type ExponentialRetryStrategy struct {
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Factor       float64
}

func (s *ExponentialRetryStrategy) NextDelay(attempt int, lastError error) time.Duration {
	delay := s.InitialDelay * time.Duration(s.Factor)
	if delay > s.MaxDelay {
		return s.MaxDelay
	}
	return delay
}
