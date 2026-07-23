package api

import "context"

// Ping represents a Buildkite Agent API Ping
type Ping struct {
	Action         string            `json:"action,omitempty"`
	Message        string            `json:"message,omitempty"`
	Job            *Job              `json:"job,omitempty"`
	Endpoint       string            `json:"endpoint,omitempty"`
	RequestHeaders map[string]string `json:"request_headers,omitzero"` // omit nil, keep empty map
}

// Pings the API and returns any work the client needs to perform
func (c *Client) Ping(ctx context.Context) (*Ping, *Response, error) {
	req, err := c.newRequest(ctx, "GET", "ping", nil)
	if err != nil {
		return nil, nil, err
	}

	ping := new(Ping)
	resp, err := c.doRequest(req, ping)
	if err != nil {
		return nil, resp, err
	}

	// If Buildkite told us to use Buildkite-* request headers, store those
	c.setRequestHeaders(ping.RequestHeaders)

	return ping, resp, err
}
