package gofetch

import (
	"context"
)

// Get is a convenience method for sending GET requests.
func (c *Client) Get(ctx context.Context, url string, headers map[string]string) (*Response, error) {
	req := NewRequest("GET", url).WithHeaders(headers)
	return c.Do(ctx, req)
}

// Post is a convenience method for sending POST requests.
func (c *Client) Post(ctx context.Context, url string, body []byte, headers map[string]string) (*Response, error) {
	req := NewRequest("POST", url).WithBody(body).WithHeaders(headers)
	return c.Do(ctx, req)
}

// Put is a convenience method for sending PUT requests.
func (c *Client) Put(ctx context.Context, url string, body []byte, headers map[string]string) (*Response, error) {
	req := NewRequest("PUT", url).WithBody(body).WithHeaders(headers)
	return c.Do(ctx, req)
}

// Delete is a convenience method for sending DELETE requests.
func (c *Client) Delete(ctx context.Context, url string, headers map[string]string) (*Response, error) {
	req := NewRequest("DELETE", url).WithHeaders(headers)
	return c.Do(ctx, req)
}

// Patch is a convenience method for sending PATCH requests.
func (c *Client) Patch(ctx context.Context, url string, body []byte, headers map[string]string) (*Response, error) {
	req := NewRequest("PATCH", url).WithBody(body).WithHeaders(headers)
	return c.Do(ctx, req)
}

// Head is a convenience method for sending HEAD requests.
func (c *Client) Head(ctx context.Context, url string, headers map[string]string) (*Response, error) {
	req := NewRequest("HEAD", url).WithHeaders(headers)
	return c.Do(ctx, req)
}

// PostJSON is a convenience method for sending POST requests with JSON body.
func (c *Client) PostJSON(ctx context.Context, url string, data interface{}, headers map[string]string) (*Response, error) {
	req := NewRequest("POST", url).WithJSONBody(data).WithHeaders(headers)
	return c.Do(ctx, req)
}

// PutJSON is a convenience method for sending PUT requests with JSON body.
func (c *Client) PutJSON(ctx context.Context, url string, data interface{}, headers map[string]string) (*Response, error) {
	req := NewRequest("PUT", url).WithJSONBody(data).WithHeaders(headers)
	return c.Do(ctx, req)
}

// PatchJSON is a convenience method for sending PATCH requests with JSON body.
func (c *Client) PatchJSON(ctx context.Context, url string, data interface{}, headers map[string]string) (*Response, error) {
	req := NewRequest("PATCH", url).WithJSONBody(data).WithHeaders(headers)
	return c.Do(ctx, req)
}
