# gofetch

[![Go Reference](https://pkg.go.dev/badge/github.com/jzx17/gofetch.svg)](https://pkg.go.dev/github.com/jzx17/gofetch)
[![Go Report Card](https://goreportcard.com/badge/github.com/jzx17/gofetch)](https://goreportcard.com/report/github.com/jzx17/gofetch)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A powerful, production-ready HTTP client library for Go with middleware support, error handling, rate limiting, automatic retries, and more.

## Features

- 🔄 **Middleware System**: Easily extend functionality with a flexible middleware chain
- 🔁 **Automatic Retries**: Configure retry strategies with exponential backoff
- 🚦 **Rate Limiting**: Client-side rate limiting to respect server limits
- 📊 **Size Validation**: Protect against excessive request/response sizes
- 📝 **Request/Response Logging**: Configurable logging with multiple formats
- ⚡ **Streaming Support**: Handle large responses efficiently with streaming
- 🔄 **Async Operations**: Execute requests asynchronously with ease
- 🛡️ **TLS Configuration**: Advanced TLS security with sensible defaults
- 🧩 **Fluent API**: Intuitive, chainable methods for request building
- 🔄 **Connection Pooling**: Efficient connection reuse

## Installation

```bash
go get github.com/jzx17/gofetch
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jzx17/gofetch"
)

func main() {
	// Create a client with default settings
	client := gofetch.NewClient()

	// Make a simple GET request
	ctx := context.Background()
	resp, err := client.Get(ctx, "https://api.example.com/users", nil)
	if err != nil {
		log.Fatalf("Request failed: %v", err)
	}
	defer resp.CloseBody()

	// Check if the request was successful
	if !resp.IsSuccess() {
		log.Fatalf("Bad status: %s", resp.Status)
	}

	// Parse JSON response
	var users []map[string]interface{}
	if err := resp.JSON(&users); err != nil {
		log.Fatalf("Failed to parse JSON: %v", err)
	}

	// Print results
	fmt.Printf("Found %d users\n", len(users))
}
```

## Core Concepts

### Client

The `Client` is the main entry point for making HTTP requests:

```go
// Create with default settings
client := gofetch.NewClient()

// Create with custom options
client := gofetch.NewClient(
    gofetch.WithTimeout(10 * time.Second),
    gofetch.WithAutoBufferResponse(true),
    gofetch.WithMiddlewares(
        gofetch.LoggingMiddleware(gofetch.DefaultLoggingOptions()),
        gofetch.RetryMiddleware(gofetch.NewExponentialBackoffStrategy(
            500*time.Millisecond, 
            5*time.Second, 
            2.0, 
            3,
        )),
    ),
)
```

### Requests

Build requests with a fluent interface:

```go
// Simple GET request
req := gofetch.NewRequest("GET", "https://api.example.com/users")

// POST request with JSON body
req := gofetch.NewRequest("POST", "https://api.example.com/users")
    .WithJSONBody(map[string]interface{}{
        "name": "John Doe",
        "email": "john@example.com",
    })
    .WithHeader("X-API-Key", "your-api-key")
```

### Execute Requests

```go
// Standard execution
resp, err := client.Do(context.Background(), req)

// With execution options
resp, err := client.Execute(context.Background(), req,
    gofetch.WithStreamProcessing(),
    gofetch.WithRequestTimeout(5 * time.Second),
)

// Convenience methods
resp, err := client.Get(context.Background(), "https://api.example.com/users", nil)
resp, err := client.PostJSON(context.Background(), "https://api.example.com/users", userData, nil)
```

### Processing Responses

```go
// Check status code
if resp.IsSuccess() {
    // Handle success
} else if resp.IsClientError() {
    // Handle client error (4xx)
} else if resp.IsServerError() {
    // Handle server error (5xx)
}

// Parse JSON response
var data MyStruct
if err := resp.JSON(&data); err != nil {
    // Handle error
}

// Parse XML response
var xmlData MyXMLStruct
if err := resp.XML(&xmlData); err != nil {
    // Handle error
}

// Get raw bytes
bytes, err := resp.Bytes()

// Get string
str, err := resp.String()

// Save to file
err := resp.SaveToFile("output.json")
```

### Streaming Responses

```go
resp, err := client.DoStream(context.Background(), req)
if err != nil {
    log.Fatal(err)
}
defer resp.CloseBody()

// Stream chunks with a callback
err = resp.StreamChunks(func(chunk []byte) {
    // Process each chunk
    fmt.Printf("Received %d bytes\n", len(chunk))
})

// Stream with context
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
err = resp.StreamChunksWithContext(ctx, func(chunk []byte) {
    // Process each chunk
})
```

### Async Requests

```go
// Single async request
respChan := client.DoAsync(context.Background(), req)

// Process the result
result := <-respChan
if result.Error != nil {
    log.Fatalf("Request failed: %v", result.Error)
}
resp := result.Response
// Process response...

// Multiple concurrent requests
reqs := []*gofetch.Request{req1, req2, req3}
resultsChan := client.DoGroupAsync(context.Background(), reqs)

// Process results
results := <-resultsChan
for i, result := range results {
    if result.Error != nil {
        log.Printf("Request %d failed: %v", i, result.Error)
        continue
    }
    // Process result.Response...
}
```

## Middleware System

### Using Built-in Middlewares

```go
// Add logging middleware
client.Use(gofetch.LoggingMiddleware(gofetch.LoggingOptions{
    Level:              gofetch.LogLevelDebug,
    RequestBodyMaxLen:  1024,
    ResponseBodyMaxLen: 1024,
    HeadersToRedact:    []string{"Authorization", "Cookie"},
    LogFormat:          gofetch.LogFormatJSON,
}))

// Add retry middleware
client.Use(gofetch.RetryMiddleware(
    gofetch.NewExponentialBackoffStrategy(500*time.Millisecond, 5*time.Second, 2.0, 3),
))

// Add rate limiting middleware
client.Use(gofetch.RateLimitMiddleware(gofetch.RateLimitOptions{
    RequestsPerSecond: 5,
    Burst:             2,
    WaitOnLimit:       true,
    MaxWaitTime:       3 * time.Second,
}))

// Add size validation
client.Use(gofetch.SizeValidationMiddleware(gofetch.SizeConfig{
    MaxRequestBodySize:  5 * 1024 * 1024,  // 5MB
    MaxResponseBodySize: 10 * 1024 * 1024, // 10MB
}))
```

### Creating Custom Middlewares

```go
// Create a simple custom middleware
authMiddleware := gofetch.CreateMiddleware(
    "auth-middleware",
    nil,
    func(next gofetch.RoundTripFunc) gofetch.RoundTripFunc {
        return func(req *http.Request) (*http.Response, error) {
            // Add authorization header
            req.Header.Set("Authorization", "Bearer your-token")
            return next(req)
        }
    },
)

client.Use(authMiddleware)
```

## Advanced Configuration

### TLS Configuration

```go
// Create a custom TLS transport
tlsConfig := &tls.Config{
    MinVersion:         tls.VersionTLS13,
    InsecureSkipVerify: false,
    // Additional TLS settings...
}

transport, err := gofetch.NewTLSTransport(
    true,                  // Enable HTTP/2
    tlsConfig,             // TLS configuration
    5*time.Second,         // TLS handshake timeout
    100,                   // Max idle connections
    90*time.Second,        // Idle connection timeout
)
if err != nil {
    log.Fatalf("Failed to create TLS transport: %v", err)
}

client := gofetch.NewClient(gofetch.WithTransport(transport))
```

### Connection Pool Configuration

```go
client := gofetch.NewClient(
    gofetch.WithConnectionPool(100, 10), // maxIdle, maxIdlePerHost
)
```

## Error Handling

```go
resp, err := client.Do(context.Background(), req)
if err != nil {
    // Check for specific error types
    var clientErr *gofetch.ClientError
    if errors.As(err, &clientErr) {
        fmt.Printf("Client error in phase '%s': %v\n", clientErr.Phase, clientErr.Message)
    }

    var statusErr *gofetch.StatusError
    if errors.As(err, &statusErr) {
        fmt.Printf("Status error: %d for URL %s\n", statusErr.StatusCode, statusErr.URL)
    }

    var retryErr *gofetch.RetryError
    if errors.As(err, &retryErr) {
        fmt.Printf("Request failed after %d attempts: %v\n", retryErr.Attempts, retryErr.LastErr)
    }

    var rateLimitErr *gofetch.RateLimitExceededError
    if errors.As(err, &rateLimitErr) {
        fmt.Printf("Rate limit exceeded: %v req/s, retry after %v\n", 
            rateLimitErr.Limit, rateLimitErr.RetryAfter)
    }

    var timeoutErr *gofetch.TimeoutError
    if errors.As(err, &timeoutErr) {
        fmt.Printf("Request timed out: %v\n", timeoutErr.Err)
    }

    var sizeErr *gofetch.SizeError
    if errors.As(err, &sizeErr) {
        fmt.Printf("%s size %d exceeds maximum of %d\n", 
            sizeErr.Type, sizeErr.Current, sizeErr.Max)
    }
}
```

## License

MIT License - See the LICENSE file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request