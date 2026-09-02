package api

import (
	"context"
	"fmt"
)

// HeaderTimes represents a set of header times that are associated with a job
// log.
type HeaderTimes struct {
	Times map[string]string `json:"header_times"`
}

// SaveHeaderTimes saves the header times to the job
func (c *Client) SaveHeaderTimes(ctx context.Context, jobId string, headerTimes *HeaderTimes) (*Response, error) {
	u := fmt.Sprintf("jobs/%s/header_times", railsPathEscape(jobId))

	req, err := c.newRequest(ctx, "POST", u, headerTimes)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequest(req, nil)
	if err != nil {
		return resp, err
	}

	return resp, err
}
