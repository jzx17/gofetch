package gofetch

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DoAsyncFunc is the function signature for the DoAsync method
type DoAsyncFunc func(ctx context.Context, req *Request) <-chan AsyncResponse

// DoAsync sends the HTTP request asynchronously. It launches a goroutine that calls Do and writes the result into a channel.
func (c *Client) DoAsync(ctx context.Context, req *Request) <-chan AsyncResponse {
	responseChan := make(chan AsyncResponse, 1)
	go func() {
		// Add panic recovery to prevent crashes
		defer func() {
			if r := recover(); r != nil {
				responseChan <- AsyncResponse{Error: fmt.Errorf("panic occurred: %v", r)}
				close(responseChan)
			}
		}()
		res, err := c.Do(ctx, req)
		responseChan <- AsyncResponse{Response: res, Error: err}
		close(responseChan)
	}()
	return responseChan
}

// DoStreamAsync is similar to DoAsync but uses DoStream to allow manual streaming.
func (c *Client) DoStreamAsync(ctx context.Context, req *Request) <-chan AsyncResponse {
	responseChan := make(chan AsyncResponse, 1)
	go func() {
		// Add panic recovery to prevent crashes
		defer func() {
			if r := recover(); r != nil {
				responseChan <- AsyncResponse{Error: fmt.Errorf("panic occurred: %v", r)}
				close(responseChan)
			}
		}()
		res, err := c.DoStream(ctx, req)
		responseChan <- AsyncResponse{Response: res, Error: err}
		close(responseChan)
	}()
	return responseChan
}

// ExecuteAsync is like Execute but runs asynchronously and returns a channel with the result
func (c *Client) ExecuteAsync(ctx context.Context, req *Request, opts ...ExecuteOption) <-chan AsyncResponse {
	responseChan := make(chan AsyncResponse, 1)
	go func() {
		// Add panic recovery to prevent crashes
		defer func() {
			if r := recover(); r != nil {
				responseChan <- AsyncResponse{Error: fmt.Errorf("panic occurred: %v", r)}
				close(responseChan)
			}
		}()
		res, err := c.Execute(ctx, req, opts...)
		responseChan <- AsyncResponse{Response: res, Error: err}
		close(responseChan)
	}()
	return responseChan
}

// DoGroupAsync fires off multiple asynchronous HTTP requests concurrently,
// one for each provided Request, and returns a channel that eventually yields a slice of AsyncResponse.
func (c *Client) DoGroupAsync(ctx context.Context, requests []*Request) <-chan []AsyncResponse {
	channels := make([]<-chan AsyncResponse, len(requests))
	for i, req := range requests {
		channels[i] = c.DoAsync(ctx, req)
	}
	return c.JoinAsyncResponses(ctx, channels...)
}

// GroupOptions specifies options for group async operations
type GroupOptions struct {
	IndividualTimeout time.Duration // Timeout for individual requests within a group
	BufferSize        int           // Buffer size for result channel
}

// DoGroupAsyncWithOptions is like DoGroupAsync but with additional options
func (c *Client) DoGroupAsyncWithOptions(ctx context.Context, requests []*Request, opts GroupOptions) <-chan []AsyncResponse {
	// Apply individual timeouts if specified
	channels := make([]<-chan AsyncResponse, len(requests))
	cancelFuncs := make([]context.CancelFunc, len(requests))

	for i, req := range requests {
		requestCtx := ctx

		if opts.IndividualTimeout > 0 {
			var cancel context.CancelFunc
			requestCtx, cancel = context.WithTimeout(ctx, opts.IndividualTimeout)
			cancelFuncs[i] = cancel
		}

		// Use a new context for each request
		thisReq := req.Clone().WithContext(requestCtx)
		channels[i] = c.DoAsync(requestCtx, thisReq)
	}

	// Use specified buffer size or default to 1
	bufferSize := 1
	if opts.BufferSize > 0 {
		bufferSize = opts.BufferSize
	}

	out := make(chan []AsyncResponse, bufferSize)
	go func() {
		// Add panic recovery to prevent crashes
		defer func() {
			// Ensure all timeouts are cleaned up
			for _, cancel := range cancelFuncs {
				if cancel != nil {
					cancel()
				}
			}

			if r := recover(); r != nil {
				close(out)
			}
		}()

		results := make([]AsyncResponse, len(channels))
		var wg sync.WaitGroup
		wg.Add(len(channels))

		for i, ch := range channels {
			go func(index int, ch <-chan AsyncResponse) {
				defer wg.Done()
				defer func() {
					// Clean up the timeout when done
					if cancelFuncs[index] != nil {
						cancelFuncs[index]()
						cancelFuncs[index] = nil
					}
				}()

				select {
				case resp := <-ch:
					results[index] = resp
				case <-ctx.Done():
					results[index] = AsyncResponse{Error: ctx.Err()}
				}
			}(i, ch)
		}

		wg.Wait()
		out <- results
		close(out)
	}()

	return out
}

// JoinAsyncResponses aggregates multiple AsyncResponse channels into one.
func (c *Client) JoinAsyncResponses(ctx context.Context, channels ...<-chan AsyncResponse) <-chan []AsyncResponse {
	out := make(chan []AsyncResponse, 1)
	go func() {
		// Add panic recovery to prevent crashes
		defer func() {
			if r := recover(); r != nil {
				close(out)
			}
		}()

		results := make([]AsyncResponse, len(channels))
		var wg sync.WaitGroup
		wg.Add(len(channels))

		for i, ch := range channels {
			go func(index int, ch <-chan AsyncResponse) {
				defer wg.Done()
				select {
				case resp := <-ch:
					results[index] = resp
				case <-ctx.Done():
					results[index] = AsyncResponse{Error: ctx.Err()}
				}
			}(i, ch)
		}

		wg.Wait()
		out <- results
		close(out)
	}()
	return out
}

// ExecuteGroupAsync executes multiple requests concurrently with the same execution options
func (c *Client) ExecuteGroupAsync(ctx context.Context, requests []*Request, opts ...ExecuteOption) <-chan []AsyncResponse {
	channels := make([]<-chan AsyncResponse, len(requests))
	for i, req := range requests {
		channels[i] = c.ExecuteAsync(ctx, req, opts...)
	}
	return c.JoinAsyncResponses(ctx, channels...)
}

// ExecuteGroupAsyncWithOptions is like ExecuteGroupAsync but with additional group options
func (c *Client) ExecuteGroupAsyncWithOptions(ctx context.Context, requests []*Request, groupOpts GroupOptions, execOpts ...ExecuteOption) <-chan []AsyncResponse {
	channels := make([]<-chan AsyncResponse, len(requests))
	cancelFuncs := make([]context.CancelFunc, len(requests))

	for i, req := range requests {
		requestCtx := ctx

		if groupOpts.IndividualTimeout > 0 {
			var cancel context.CancelFunc
			requestCtx, cancel = context.WithTimeout(ctx, groupOpts.IndividualTimeout)
			cancelFuncs[i] = cancel
		}

		// Use a new context for each request
		thisReq := req.Clone().WithContext(requestCtx)
		channels[i] = c.ExecuteAsync(requestCtx, thisReq, execOpts...)
	}

	bufferSize := 1
	if groupOpts.BufferSize > 0 {
		bufferSize = groupOpts.BufferSize
	}

	out := make(chan []AsyncResponse, bufferSize)
	go func() {
		defer func() {
			// Ensure all timeouts are cleaned up
			for _, cancel := range cancelFuncs {
				if cancel != nil {
					cancel()
				}
			}

			if r := recover(); r != nil {
				close(out)
			}
		}()

		results := make([]AsyncResponse, len(channels))
		var wg sync.WaitGroup
		wg.Add(len(channels))

		for i, ch := range channels {
			go func(index int, ch <-chan AsyncResponse) {
				defer wg.Done()
				defer func() {
					// Clean up the timeout when done
					if cancelFuncs[index] != nil {
						cancelFuncs[index]()
						cancelFuncs[index] = nil
					}
				}()

				select {
				case resp := <-ch:
					results[index] = resp
				case <-ctx.Done():
					results[index] = AsyncResponse{Error: ctx.Err()}
				}
			}(i, ch)
		}

		wg.Wait()
		out <- results
		close(out)
	}()

	return out
}

// GetAsync is a convenience wrapper for asynchronous GET requests.
func (c *Client) GetAsync(ctx context.Context, url string, headers map[string]string) <-chan AsyncResponse {
	req := NewRequest("GET", url).WithHeaders(headers)
	return c.DoAsync(ctx, req)
}

// PostAsync is a convenience wrapper for asynchronous POST requests.
func (c *Client) PostAsync(ctx context.Context, url string, body []byte, headers map[string]string) <-chan AsyncResponse {
	req := NewRequest("POST", url).WithBody(body).WithHeaders(headers)
	return c.DoAsync(ctx, req)
}
