package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/buildkite/roko"
)

type GithubCodeAccessTokenRequest struct {
	RepoURL string `json:"repo_url,omitempty"`
}

type GithubCodeAccessTokenResponse struct {
	Token string `json:"token,omitempty"`
}

func (c *Client) GenerateGithubCodeAccessToken(ctx context.Context, repoURL, jobID string) (string, *Response, error) {
	u := fmt.Sprintf("jobs/%s/github_code_access_token", railsPathEscape(jobID))

	req, err := c.newRequest(ctx, http.MethodPost, u, GithubCodeAccessTokenRequest{RepoURL: repoURL})
	if err != nil {
		return "", nil, err
	}

	r := roko.NewRetrier(
		roko.WithMaxAttempts(3),
		roko.WithStrategy(roko.Constant(5*time.Second)),
	)

	var g GithubCodeAccessTokenResponse

	resp, err := roko.DoFunc(ctx, r, func(r *roko.Retrier) (*Response, error) {
		resp, err := c.doRequest(req, &g)
		if err == nil {
			return resp, nil
		}

		if resp != nil {
			if !IsRetryableStatus(resp) {
				r.Break()
				return resp, err
			}

			if resp.Header.Get("Retry-After") != "" {
				retryAfter, errParseDuration := time.ParseDuration(resp.Header.Get("Retry-After") + "s")
				if errParseDuration == nil {
					r.SetNextInterval(retryAfter)
				}
			}
		}

		return resp, err
	})
	if err != nil {
		return "", resp, err
	}

	return g.Token, resp, nil
}
