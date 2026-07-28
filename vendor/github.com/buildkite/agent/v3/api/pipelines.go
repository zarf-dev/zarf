package api

import (
	"context"
	"fmt"
)

// PipelineChange represents a Buildkite Agent API PipelineChange
type PipelineChange struct {
	// UUID identifies this pipeline change. We keep this constant during
	// retry loops so that work is not repeated on the API server
	UUID     string `json:"uuid"`
	Pipeline any    `json:"pipeline"`
	Replace  bool   `json:"replace,omitempty"`
}

type PipelineUploadStatus struct {
	State   string `json:"state"`
	Message string `json:"message"`
}

// UploadPipeline uploads the pipeline to the Buildkite Agent API. It does not wait for the
// pipeline to finish processing but will instead return with a redirect to the location to check
// the pipeline's status.
func (c *Client) UploadPipeline(
	ctx context.Context,
	jobId string,
	pipeline *PipelineChange,
	headers ...Header,
) (*Response, error) {
	u := fmt.Sprintf("jobs/%s/pipelines?async=true", railsPathEscape(jobId))

	req, err := c.newRequest(ctx, "POST", u, pipeline, headers...)
	if err != nil {
		return nil, err
	}

	return c.doRequest(req, nil)
}

func (c *Client) PipelineUploadStatus(
	ctx context.Context,
	jobId string,
	uuid string,
	headers ...Header,
) (*PipelineUploadStatus, *Response, error) {
	u := fmt.Sprintf("jobs/%s/pipelines/%s", railsPathEscape(jobId), railsPathEscape(uuid))

	req, err := c.newRequest(ctx, "GET", u, nil, headers...)
	if err != nil {
		return nil, nil, err
	}

	status := &PipelineUploadStatus{}
	resp, err := c.doRequest(req, status)
	return status, resp, err
}
