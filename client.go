package gofetch

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync"
	"time"
)

// Client is a configurable API client that supports middleware chaining and request building.
type Client struct {
	client        *http.Client
	rt            http.RoundTripper
	baseTransport http.RoundTripper
	middlewares   []ConfigurableMiddleware
	timeout       time.Duration
	// autoBuffer controls whether non-streaming responses are fully read into memory.
	autoBuffer bool
	sizeConfig SizeConfig
	mu         sync.RWMutex // protects middlewares
}

// NewClient creates a new API client with default settings (30-second timeout, auto-buffering enabled),
// optionally configured by provided options.
func NewClient(options ...Option) *Client {
	c := &Client{
		rt:          http.DefaultTransport,
		middlewares: []ConfigurableMiddleware{},
		timeout:     30 * time.Second,
		autoBuffer:  true,
		sizeConfig:  DefaultSizeConfig(),
	}

	// Apply provided options.
	for _, opt := range options {
		opt(c)
	}

	// Determine the base RoundTripper.
	baseRt := http.DefaultTransport
	if c.client != nil && c.client.Transport != nil {
		baseRt = c.client.Transport
	} else if c.rt != nil {
		baseRt = c.rt
	}
	c.baseTransport = baseRt

	// Wrap the base transport with the middleware chain.
	wrappedRt := c.wrapTransport(c.baseTransport)

	if c.client == nil {
		timeout := 30 * time.Second
		if c.timeout != 0 {
			timeout = c.timeout
		}
		c.client = &http.Client{
			Transport: wrappedRt,
			Timeout:   timeout,
		}
	} else {
		if c.timeout != 0 {
			c.client.Timeout = c.timeout
		}
		c.client.Transport = wrappedRt
	}

	return c
}

// wrapTransport builds the middleware chain on top of the provided base RoundTripper.
func (c *Client) wrapTransport(base http.RoundTripper) http.RoundTripper {
	return RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		c.mu.RLock()
		mws := make([]ConfigurableMiddleware, len(c.middlewares))
		copy(mws, c.middlewares)
		c.mu.RUnlock()

		final := func(req *http.Request) (*http.Response, error) {
			return base.RoundTrip(req)
		}
		chain := ChainMiddlewares(final, mws...)
		return chain(req)
	})
}

// getBaseRoundTripper returns the unwrapped base RoundTripper.
func (c *Client) getBaseRoundTripper() http.RoundTripper {
	if c.baseTransport != nil {
		return c.baseTransport
	}
	return http.DefaultTransport
}

// Use adds a middleware to the client and returns the client for chaining.
func (c *Client) Use(mw ConfigurableMiddleware) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.middlewares = append(c.middlewares, mw)
	if c.client != nil {
		c.client.Transport = c.wrapTransport(c.getBaseRoundTripper())
	}
	return c
}

// UpdateMiddleware updates or adds a middleware.
func (c *Client) UpdateMiddleware(mw ConfigurableMiddleware) {
	c.mu.Lock()
	defer c.mu.Unlock()

	targetIdentity := mw.GetIdentifier()
	found := false
	for i, existing := range c.middlewares {
		if existing.GetIdentifier().Name == targetIdentity.Name {
			c.middlewares[i] = mw
			found = true
			break
		}
	}
	if !found {
		c.middlewares = append(c.middlewares, mw)
	}
	if c.client != nil {
		c.client.Transport = c.wrapTransport(c.getBaseRoundTripper())
	}
}

// RemoveMiddleware removes middleware by name.
func (c *Client) RemoveMiddleware(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	filtered := make([]ConfigurableMiddleware, 0, len(c.middlewares))
	for _, mw := range c.middlewares {
		if mw.GetIdentifier().Name != name {
			filtered = append(filtered, mw)
		}
	}
	c.middlewares = filtered
	if c.client != nil {
		c.client.Transport = c.wrapTransport(c.getBaseRoundTripper())
	}
}

// GetMiddlewares returns a copy of the current middleware list.
func (c *Client) GetMiddlewares() []ConfigurableMiddleware {
	c.mu.RLock()
	defer c.mu.RUnlock()

	mws := make([]ConfigurableMiddleware, len(c.middlewares))
	copy(mws, c.middlewares)
	return mws
}

// Do send the HTTP request built from the provided Request and returns a Response.
// For non-streaming requests, if autoBuffer is enabled, the full response is read into memory.
func (c *Client) Do(ctx context.Context, req *Request) (res *Response, err error) {
	httpReq, err := req.BuildHTTPRequest()
	if err != nil {
		return nil, NewRequestError("build request", err)
	}
	httpReq = httpReq.WithContext(ctx)
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, NewTransportError("execute request", err)
	}
	if c.autoBuffer {
		defer func() {
			if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
				err = NewResponseError("close response body", closeErr)
			}
		}()
		var bodyBuf bytes.Buffer
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, NewResponseError("read response body", err)
		}
		bodyBuf.Write(data)
		return &Response{Response: &http.Response{
			Status:     resp.Status,
			StatusCode: resp.StatusCode,
			Header:     resp.Header,
			Body:       io.NopCloser(bytes.NewReader(bodyBuf.Bytes())),
		}}, nil
	}
	return &Response{Response: resp}, nil
}

// DoWithTimeout is like Do but with a specific timeout for this request
func (c *Client) DoWithTimeout(parentCtx context.Context, req *Request, timeout time.Duration) (*Response, error) {
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()
	return c.Do(ctx, req)
}

// DoStream sends the HTTP request built from the provided Request and returns a Response for manual streaming.
// The caller is responsible for closing the response.
func (c *Client) DoStream(ctx context.Context, req *Request) (*Response, error) {
	httpReq, err := req.BuildHTTPRequest()
	if err != nil {
		return nil, NewRequestError("build HTTP request", err)
	}
	httpReq = httpReq.WithContext(ctx)
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, NewTransportError("execute HTTP request", err)
	}
	return &Response{Response: resp}, nil
}

// Execute sends HTTP request and returns a response with various options
func (c *Client) Execute(ctx context.Context, req *Request, opts ...ExecuteOption) (*Response, error) {
	config := defaultExecuteConfig()
	for _, opt := range opts {
		opt(config)
	}

	if config.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, config.timeout)
		defer cancel()
	}

	if config.stream {
		return c.DoStream(ctx, req)
	}

	return c.Do(ctx, req)
}

// ExecuteOption configures how the request is executed
type ExecuteOption func(*executeConfig)

type executeConfig struct {
	stream  bool
	timeout time.Duration
}

func defaultExecuteConfig() *executeConfig {
	return &executeConfig{
		stream:  false,
		timeout: 0,
	}
}

// WithStreamProcessing enables streaming response processing
func WithStreamProcessing() ExecuteOption {
	return func(c *executeConfig) {
		c.stream = true
	}
}

// WithRequestTimeout sets a specific timeout for this request
func WithRequestTimeout(timeout time.Duration) ExecuteOption {
	return func(c *executeConfig) {
		c.timeout = timeout
	}
}
