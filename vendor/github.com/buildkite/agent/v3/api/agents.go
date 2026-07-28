package api

import "context"

// AgentRegisterRequest is a call to register on the Buildkite Agent API
type AgentRegisterRequest struct {
	Name               string   `json:"name"`
	Hostname           string   `json:"hostname"`
	OS                 string   `json:"os"`
	Arch               string   `json:"arch"`
	ScriptEvalEnabled  bool     `json:"script_eval_enabled"`
	IgnoreInDispatches bool     `json:"ignore_in_dispatches"`
	Priority           string   `json:"priority,omitempty"`
	Version            string   `json:"version"`
	Build              string   `json:"build"`
	Tags               []string `json:"meta_data"`
	PID                int      `json:"pid,omitempty"`
	MachineID          string   `json:"machine_id,omitempty"`
	Features           []string `json:"features"`
}

// AgentRegisterResponse is the response from the Buildkite Agent API
type AgentRegisterResponse struct {
	UUID              string            `json:"id"`
	Name              string            `json:"name"`
	AccessToken       string            `json:"access_token"`
	Endpoint          string            `json:"endpoint"`
	RequestHeaders    map[string]string `json:"request_headers"`
	PingInterval      int               `json:"ping_interval"`
	JobStatusInterval int               `json:"job_status_interval"`
	HeartbeatInterval int               `json:"heartbeat_interval"`
	Tags              []string          `json:"meta_data"`
}

// Registers the agent against the Buildkite Agent API. The client for this
// call must be authenticated using an Agent Registration Token
func (c *Client) Register(ctx context.Context, regReq *AgentRegisterRequest) (*AgentRegisterResponse, *Response, error) {
	req, err := c.newRequest(ctx, "POST", "register", regReq)
	if err != nil {
		return nil, nil, err
	}

	a := new(AgentRegisterResponse)
	resp, err := c.doRequest(req, a)
	if err != nil {
		return nil, resp, err
	}

	// If Buildkite told us to use Buildkite-* request headers, store those
	c.setRequestHeaders(a.RequestHeaders)

	return a, resp, err
}

// Connect connects the agent to the Buildkite Agent API (calls the connect
// method - it doesn't necessarily open a new underlying network connection!).
func (c *Client) Connect(ctx context.Context) (*Response, error) {
	req, err := c.newRequest(ctx, "POST", "connect", nil)
	if err != nil {
		return nil, err
	}

	return c.doRequest(req, nil)
}

// Disconnect disconnects the agent from the Buildkite Agent API (calls the
// disconnect method - it doesn't necessarily close the underlying network
// connection!).
func (c *Client) Disconnect(ctx context.Context) (*Response, error) {
	req, err := c.newRequest(ctx, "POST", "disconnect", nil)
	if err != nil {
		return nil, err
	}

	return c.doRequest(req, nil)
}

// AgentStopRequest is a call to stop the agent via the Buildkite Agent API
type AgentStopRequest struct {
	Force bool `json:"force,omitempty"`
}

// Stop stops the agent via the Buildkite Agent API
func (c *Client) Stop(ctx context.Context, stopReq *AgentStopRequest) (*Response, error) {
	req, err := c.newRequest(ctx, "POST", "stop", stopReq)
	if err != nil {
		return nil, err
	}

	return c.doRequest(req, nil)
}

// AgentPauseRequest is a call to pause the agent via the Buildkite Agent API.
type AgentPauseRequest struct {
	Note             string `json:"note,omitempty"`
	TimeoutInMinutes int    `json:"timeout_in_minutes,omitempty"`
}

// Pause pauses the agent via the Buildkite Agent API.
func (c *Client) Pause(ctx context.Context, pauseReq *AgentPauseRequest) (*Response, error) {
	req, err := c.newRequest(ctx, "POST", "pause", pauseReq)
	if err != nil {
		return nil, err
	}

	return c.doRequest(req, nil)
}

// Resume resumes a paused agent via the Buildkite Agent API.
func (c *Client) Resume(ctx context.Context) (*Response, error) {
	req, err := c.newRequest(ctx, "POST", "resume", nil)
	if err != nil {
		return nil, err
	}

	return c.doRequest(req, nil)
}
