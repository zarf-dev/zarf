package api

import (
	"context"
	"fmt"
)

type OIDCToken struct {
	Token string `json:"token"`
}

type OIDCTokenRequest struct {
	Job            string
	Audience       string
	Lifetime       int
	Claims         []string
	AWSSessionTags []string
}

func (c *Client) OIDCToken(ctx context.Context, methodReq *OIDCTokenRequest) (*OIDCToken, *Response, error) {
	m := &struct {
		Audience       string   `json:"audience,omitempty"`
		Lifetime       int      `json:"lifetime,omitempty"`
		Claims         []string `json:"claims,omitempty"`
		AWSSessionTags []string `json:"aws_session_tags,omitempty"`
	}{
		Audience:       methodReq.Audience,
		Lifetime:       methodReq.Lifetime,
		Claims:         methodReq.Claims,
		AWSSessionTags: methodReq.AWSSessionTags,
	}

	u := fmt.Sprintf("jobs/%s/oidc/tokens", railsPathEscape(methodReq.Job))
	httpReq, err := c.newRequest(ctx, "POST", u, m)
	if err != nil {
		return nil, nil, err
	}

	t := &OIDCToken{}
	resp, err := c.doRequest(httpReq, t)
	if err != nil {
		return nil, resp, err
	}

	return t, resp, nil
}
