package api

import (
	"context"
	"fmt"
)

type Build struct {
	UUID string `json:"uuid"`
}

// CancelBuild cancels a build with the given UUID
func (c *Client) CancelBuild(ctx context.Context, uuid string) (*Build, *Response, error) {
	u := fmt.Sprintf("builds/%s/cancel", railsPathEscape(uuid))

	req, err := c.newRequest(ctx, "POST", u, nil)
	if err != nil {
		return nil, nil, err
	}

	build := new(Build)
	resp, err := c.doRequest(req, build)
	if err != nil {
		return nil, resp, err
	}

	return build, resp, nil
}
