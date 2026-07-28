package api

import (
	"context"
	"fmt"

	"github.com/buildkite/go-pipeline"
)

// Job represents a Buildkite Agent API Job
type Job struct {
	ID                    string                     `json:"id"`
	Endpoint              string                     `json:"endpoint"`
	State                 string                     `json:"state"`
	Env                   map[string]string          `json:"env"`
	Step                  pipeline.CommandStep       `json:"step"`
	MatrixPermutation     pipeline.MatrixPermutation `json:"matrix_permutation"`
	ChunksMaxSizeBytes    uint64                     `json:"chunks_max_size_bytes"`
	ChunksIntervalSeconds int                        `json:"chunks_interval_seconds"`
	LogMaxSizeBytes       uint64                     `json:"log_max_size_bytes"`
	Token                 string                     `json:"token"`
	ExitStatus            string                     `json:"exit_status"`
	Signal                string                     `json:"signal"`
	SignalReason          string                     `json:"signal_reason"`
	StartedAt             string                     `json:"started_at"`
	FinishedAt            string                     `json:"finished_at"`
	RunnableAt            string                     `json:"runnable_at"`
	ChunksFailedCount     int                        `json:"chunks_failed_count"`
	TraceParent           string                     `json:"traceparent"`
	TraceState            string                     `json:"tracestate"`
	Priority              int                        `json:"priority"`
}

type JobState struct {
	State string `json:"state,omitempty"`
}

type jobStartRequest struct {
	StartedAt string `json:"started_at,omitempty"`
}

type JobFinishRequest struct {
	ExitStatus              string `json:"exit_status,omitempty"`
	Signal                  string `json:"signal,omitempty"`
	SignalReason            string `json:"signal_reason,omitempty"`
	FinishedAt              string `json:"finished_at,omitempty"`
	ChunksFailedCount       int    `json:"chunks_failed_count"`
	IgnoreAgentInDispatches *bool  `json:"ignore_agent_in_dispatches,omitempty"`
}

// GetJobState returns the state of a given job
func (c *Client) GetJobState(ctx context.Context, id string) (*JobState, *Response, error) {
	u := fmt.Sprintf("jobs/%s", railsPathEscape(id))

	req, err := c.newRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	s := new(JobState)
	resp, err := c.doRequest(req, s)
	if err != nil {
		return nil, resp, err
	}

	return s, resp, err
}

// Acquires a job using its ID
func (c *Client) AcquireJob(ctx context.Context, id string, headers ...Header) (*Job, *Response, error) {
	u := fmt.Sprintf("jobs/%s/acquire", railsPathEscape(id))

	req, err := c.newRequest(ctx, "PUT", u, nil, headers...)
	if err != nil {
		return nil, nil, err
	}

	j := new(Job)
	resp, err := c.doRequest(req, j)
	if err != nil {
		return nil, resp, err
	}

	return j, resp, err
}

// AcceptJob accepts the passed in job. Returns the job with its finalized set of
// environment variables (when a job is accepted, the agents environment is
// applied to the job)
func (c *Client) AcceptJob(ctx context.Context, jobID string) (*Job, *Response, error) {
	u := fmt.Sprintf("jobs/%s/accept", railsPathEscape(jobID))

	req, err := c.newRequest(ctx, "PUT", u, nil)
	if err != nil {
		return nil, nil, err
	}

	j := new(Job)
	resp, err := c.doRequest(req, j)
	if err != nil {
		return nil, resp, err
	}

	return j, resp, err
}

// StartJob starts the passed in job
func (c *Client) StartJob(ctx context.Context, job *Job) (*Response, error) {
	u := fmt.Sprintf("jobs/%s/start", railsPathEscape(job.ID))

	req, err := c.newRequest(ctx, "PUT", u, &jobStartRequest{
		StartedAt: job.StartedAt,
	})
	if err != nil {
		return nil, err
	}

	return c.doRequest(req, nil)
}

// FinishJob finishes the passed in job
func (c *Client) FinishJob(ctx context.Context, job *Job, ignoreAgentInDispatches *bool) (*Response, error) {
	u := fmt.Sprintf("jobs/%s/finish", railsPathEscape(job.ID))

	req, err := c.newRequest(ctx, "PUT", u, &JobFinishRequest{
		FinishedAt:              job.FinishedAt,
		ExitStatus:              job.ExitStatus,
		Signal:                  job.Signal,
		SignalReason:            job.SignalReason,
		ChunksFailedCount:       job.ChunksFailedCount,
		IgnoreAgentInDispatches: ignoreAgentInDispatches,
	})
	if err != nil {
		return nil, err
	}

	return c.doRequest(req, nil)
}

// JobUpdateResponse is the response from updating a job
type JobUpdateResponse struct {
	ID string `json:"id"`
}

// UpdateJob updates mutable attributes on a job
func (c *Client) UpdateJob(ctx context.Context, id string, attrs map[string]string) (*JobUpdateResponse, *Response, error) {
	u := fmt.Sprintf("jobs/%s", railsPathEscape(id))

	req, err := c.newRequest(ctx, "PUT", u, attrs)
	if err != nil {
		return nil, nil, err
	}

	j := new(JobUpdateResponse)
	resp, err := c.doRequest(req, j)
	if err != nil {
		return nil, resp, err
	}

	return j, resp, err
}
