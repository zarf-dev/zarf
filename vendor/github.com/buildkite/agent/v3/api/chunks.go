package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
)

// Chunk represents a Buildkite Agent API Chunk
type Chunk struct {
	Data     []byte
	Sequence uint64
	Offset   uint64
	Size     uint64
}

// Uploads the chunk to the Buildkite Agent API. This request sends the
// compressed log directly as a request body.
func (c *Client) UploadChunk(ctx context.Context, jobId string, chunk *Chunk) (*Response, error) {
	// Create a compressed buffer of the log content
	body := &bytes.Buffer{}
	gzipper := gzip.NewWriter(body)
	if _, err := gzipper.Write(chunk.Data); err != nil {
		return nil, err
	}
	if err := gzipper.Close(); err != nil {
		return nil, err
	}

	// Pass most params as query
	u := fmt.Sprintf("jobs/%s/chunks?sequence=%d&offset=%d&size=%d", railsPathEscape(jobId), chunk.Sequence, chunk.Offset, chunk.Size)
	req, err := c.newFormRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, err
	}

	// Mark the request as a direct compressed log chunk
	req.Header.Add("Content-Type", "text/plain")
	req.Header.Add("Content-Encoding", "gzip")

	return c.doRequest(req, nil)
}
