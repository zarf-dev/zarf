package api

import (
	"context"
	"fmt"
	"iter"
	"net/url"

	"connectrpc.com/connect"
	agentedgev1 "github.com/buildkite/agent/v3/api/proto/gen"
	"github.com/buildkite/agent/v3/api/proto/gen/agentedgev1connect"
)

// StreamPings opens a ConnectRPC channel for streaming pings. It returns an
// iterator over received messages and any error that occurs.
func (c *Client) StreamPings(ctx context.Context, agentID string, opts ...connect.ClientOption) (iter.Seq2[*agentedgev1.StreamPingsResponse, error], error) {
	// The streaming endpoint is the same as the main endpoint,
	// minus the `/v3/`.
	u, err := url.Parse(c.conf.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("parsing endpoint: %w", err)
	}
	u.Path = "/"

	cl := agentedgev1connect.NewAgentEdgeServiceClient(
		c.client,
		u.String(),
		connect.WithGRPC(),
		connect.WithClientOptions(opts...),
	)

	// In order to set request headers, we need to tweak a value set in the
	// context. To me, this feels too much like burying optional parameters
	// in a context, which I think is bad - https://pkg.go.dev/context says:
	// "Use context Values only for request-scoped data that transits processes
	// and APIs, not for passing optional parameters to functions."
	ctx, callInfo := connect.NewClientContext(ctx)
	h := callInfo.RequestHeader()

	// Add any request headers specified by the server during register/ping
	for k, values := range c.requestHeaders {
		for _, v := range values {
			h.Add(k, v)
		}
	}

	// The Authorization header is added by the custom transport.
	// Other methods add User-Agent in newRequest.
	// Note that this does not set the entire header.
	// ConnectRPC takes our value here and adds its own component *before* our
	// own, which violates the convention of decreasing importance
	// (see RFC 7231 section 5.5.3).
	h.Set("User-Agent", c.conf.UserAgent)
	stream, err := cl.StreamPings(ctx, connect.NewRequest(&agentedgev1.StreamPingsRequest{
		AgentId: agentID,
	}))
	if err != nil {
		return nil, fmt.Errorf("from StreamPings: %w", err)
	}

	return func(yield func(*agentedgev1.StreamPingsResponse, error) bool) {
		defer stream.Close() //nolint:errcheck // Best-effort cleanup
		for stream.Receive() {
			if !yield(stream.Msg(), nil) {
				return
			}
		}
		if err := stream.Err(); err != nil {
			yield(nil, err)
		}
	}, nil
}
