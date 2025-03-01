package gofetch

import (
	"context"
	"sync"
)

// DoAsync sends the HTTP request asynchronously. It launches a goroutine that calls Do and writes the result into a channel.
func (c *Client) DoAsync(ctx context.Context, req *Request) <-chan AsyncResponse {
	responseChan := make(chan AsyncResponse, 1)
	go func() {
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
		res, err := c.Execute(ctx, req, opts...)
		responseChan <- AsyncResponse{Response: res, Error: err}
		close(responseChan)
	}()
	return responseChan
}

// DoGroupAsync fires off multiple asynchronous HTTP requests concurrently,
// one for each provided Request, and returns a channel that eventually yields a slice of AsyncResponse.
func (c *Client) DoGroupAsync(ctx context.Context, requests ...*Request) <-chan []AsyncResponse {
	channels := make([]<-chan AsyncResponse, len(requests))
	for i, req := range requests {
		channels[i] = c.DoAsync(ctx, req)
	}
	return c.JoinAsyncResponses(channels...)
}

// JoinAsyncResponses aggregates multiple AsyncResponse channels into one.
func (c *Client) JoinAsyncResponses(channels ...<-chan AsyncResponse) <-chan []AsyncResponse {
	out := make(chan []AsyncResponse, 1)
	go func() {
		results := make([]AsyncResponse, len(channels))
		var wg sync.WaitGroup
		wg.Add(len(channels))
		for i, ch := range channels {
			go func(index int, ch <-chan AsyncResponse) {
				results[index] = <-ch
				wg.Done()
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
	return c.JoinAsyncResponses(channels...)
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
