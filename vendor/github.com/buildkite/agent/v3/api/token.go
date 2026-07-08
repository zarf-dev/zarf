package api

import (
	"context"
	"net/http"
)

// AgentTokenIdentity describes token identity information.
type AgentTokenIdentity struct {
	UUID                  string `json:"uuid"`
	Description           string `json:"description"`
	TokenType             string `json:"token_type"`
	OrganizationSlug      string `json:"organization_slug"`
	OrganizationUUID      string `json:"organization_uuid"`
	ClusterUUID           string `json:"cluster_uuid"`
	ClusterName           string `json:"cluster_name"`
	OrganizationQueueUUID string `json:"organization_queue_uuid"`
	OrganizationQueueKey  string `json:"organization_queue_key"`
}

// GetTokenIdentity gets the identity information of an agent token.
func (c *Client) GetTokenIdentity(ctx context.Context) (*AgentTokenIdentity, *Response, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "token", nil)
	if err != nil {
		return nil, nil, err
	}

	ident := new(AgentTokenIdentity)
	resp, err := c.doRequest(req, ident)
	if err != nil {
		return nil, resp, err
	}

	return ident, resp, nil
}
