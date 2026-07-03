package api

import (
	"context"
	"path"
)

// GetSecretRequest represents a request to read a secret from the Buildkite Agent API.
type GetSecretRequest struct {
	Key   string
	JobID string
}

// Secret represents a secret read from the Buildkite Agent API.
type Secret struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	UUID  string `json:"uuid"`
}

// GetSecret reads a secret from the Buildkite Agent API.
func (c *Client) GetSecret(ctx context.Context, req *GetSecretRequest) (*Secret, *Response, error) {
	// the endpoint is /jobs/:job_id/secrets?key=:key
	httpReq, err := c.newRequest(ctx, "GET", path.Join("jobs", railsPathEscape(req.JobID), "secrets"), nil)
	if err != nil {
		return nil, nil, err
	}

	q := httpReq.URL.Query()
	q.Add("key", req.Key)
	httpReq.URL.RawQuery = q.Encode()

	secret := &Secret{}
	resp, err := c.doRequest(httpReq, secret)
	if err != nil {
		return nil, resp, err
	}

	return secret, resp, nil
}
