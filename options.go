package gofetch

import (
	"net/http"
	"time"
)

type Option func(*Client)

// WithHTTPClient sets a custom *http.Client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.client = client
	}
}

// WithTransport sets a custom http.RoundTripper for the client.
func WithTransport(rt http.RoundTripper) Option {
	return func(c *Client) {
		c.rt = rt
	}
}

// WithConnectionPool sets the maximum idle connections and maximum idle connections per host.
func WithConnectionPool(maxIdle, maxIdlePerHost int) Option {
	return func(c *Client) {
		if ct, ok := c.rt.(*http.Transport); ok {
			ct.MaxIdleConns = maxIdle
			ct.MaxIdleConnsPerHost = maxIdlePerHost
		}
	}
}

// WithTimeout sets the timeout for HTTP requests.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.timeout = timeout
		if c.client != nil {
			c.client.Timeout = timeout
		}
	}
}

// WithMiddlewares adds one or more middleware functions to the client.
func WithMiddlewares(mws ...ConfigurableMiddleware) Option {
	return func(c *Client) {
		c.middlewares = append(c.middlewares, mws...)
	}
}

// WithAutoBufferResponse configures whether non-streaming responses are fully buffered into memory.
// Set to false if you wish to handle the response stream manually. Defaults to true.
func WithAutoBufferResponse(autoBuffer bool) Option {
	return func(c *Client) {
		c.autoBuffer = autoBuffer
	}
}

// WithSizeConfig sets the size configuration and updates the middleware.
// Returns an error if any size value is negative.
func WithSizeConfig(config SizeConfig) Option {
	return func(c *Client) {
		// Validate all size values
		if config.MaxRequestBodySize < 0 {
			panic("RequestBodySize must be greater than or equal to 0")
		}
		if config.MaxResponseBodySize < 0 {
			panic("ResponseBodySize must be greater than or equal to 0")
		}
		if config.MaxStreamSize < 0 {
			panic("StreamSize must be greater than or equal to 0")
		}

		c.sizeConfig = config
		sizeMiddleware := SizeValidationMiddleware(config)

		c.Use(sizeMiddleware)

	}
}
