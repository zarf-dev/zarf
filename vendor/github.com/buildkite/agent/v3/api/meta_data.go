package api

import (
	"context"
	"errors"
	"fmt"
)

// MetaData represents a Buildkite Agent API MetaData
type MetaData struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

// MetaDataExists represents a Buildkite Agent API MetaData Exists check
// response
type MetaDataExists struct {
	Exists bool `json:"exists"`
}

// Sets the meta data value
func (c *Client) SetMetaData(ctx context.Context, jobId string, metaData *MetaData) (*Response, error) {
	u := fmt.Sprintf("jobs/%s/data/set", railsPathEscape(jobId))

	req, err := c.newRequest(ctx, "POST", u, metaData)
	if err != nil {
		return nil, err
	}

	return c.doRequest(req, nil)
}

// Gets the meta data value
func (c *Client) GetMetaData(ctx context.Context, scope, id, key string) (*MetaData, *Response, error) {
	if scope != "job" && scope != "build" {
		return nil, nil, errors.New("scope must either be job or build")
	}

	u := fmt.Sprintf("%ss/%s/data/get", scope, railsPathEscape(id))
	m := &MetaData{Key: key}

	req, err := c.newRequest(ctx, "POST", u, m)
	if err != nil {
		return nil, nil, err
	}

	resp, err := c.doRequest(req, m)
	if err != nil {
		return nil, resp, err
	}

	return m, resp, err
}

// Returns true if the meta data key has been set, false if it hasn't.
func (c *Client) ExistsMetaData(ctx context.Context, scope, id, key string) (*MetaDataExists, *Response, error) {
	if scope != "job" && scope != "build" {
		return nil, nil, errors.New("scope must either be job or build")
	}

	u := fmt.Sprintf("%ss/%s/data/exists", scope, railsPathEscape(id))
	m := &MetaData{Key: key}

	req, err := c.newRequest(ctx, "POST", u, m)
	if err != nil {
		return nil, nil, err
	}

	e := new(MetaDataExists)
	resp, err := c.doRequest(req, e)
	if err != nil {
		return nil, resp, err
	}

	return e, resp, err
}

func (c *Client) MetaDataKeys(ctx context.Context, scope, id string) ([]string, *Response, error) {
	if scope != "job" && scope != "build" {
		return nil, nil, errors.New("scope must either be job or build")
	}

	u := fmt.Sprintf("%ss/%s/data/keys", scope, railsPathEscape(id))

	req, err := c.newRequest(ctx, "POST", u, nil)
	if err != nil {
		return nil, nil, err
	}

	keys := []string{}
	resp, err := c.doRequest(req, &keys)
	if err != nil {
		return nil, resp, err
	}

	return keys, resp, err
}
