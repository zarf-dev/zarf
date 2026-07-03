package api

import (
	"context"
	"fmt"
	"time"
)

// Artifact represents an artifact on the Buildkite Agent API
type Artifact struct {
	// The ID of the artifact. The ID is assigned to it after a successful
	// batch creation
	ID string `json:"id"`

	// The path to the artifact relative to the working directory
	Path string `json:"path"`

	// The absolute path to the artifact
	AbsolutePath string `json:"absolute_path"`

	// The glob path used to find this artifact
	GlobPath string `json:"glob_path"`

	// The size of the file in bytes
	FileSize int64 `json:"file_size"`

	// A SHA-1 hash of the uploaded file
	Sha1Sum string `json:"sha1sum"`

	// A SHA-2 256-bit hash of the uploaded file, possibly empty
	Sha256Sum string `json:"sha256sum"`

	// ID of the job that created this artifact (from API)
	JobID string `json:"job_id"`

	// UTC timestamp this artifact was considered created
	CreatedAt time.Time `json:"created_at"`

	// The HTTP url to this artifact once it's been uploaded
	URL string `json:"url,omitempty"`

	// The destination specified on the command line when this file was
	// uploaded
	UploadDestination string `json:"upload_destination,omitempty"`

	// Information on how to upload this artifact.
	UploadInstructions *ArtifactUploadInstructions `json:"-"`

	// A specific Content-Type to use on upload
	ContentType string `json:"content_type,omitempty"`
}

type ArtifactBatch struct {
	ID                 string      `json:"id"`
	Artifacts          []*Artifact `json:"artifacts"`
	UploadDestination  string      `json:"upload_destination"`
	MultipartSupported bool        `json:"multipart_supported,omitempty"`
}

// ArtifactUploadInstructions describes how to upload an artifact to Buildkite
// artifact storage.
type ArtifactUploadInstructions struct {
	// Used for a single-part upload.
	Action ArtifactUploadAction `json:"action"`

	// Used for a multi-part upload.
	Actions []ArtifactUploadAction `json:"actions"`

	// Contains other data necessary for interpreting instructions.
	Data map[string]string `json:"data"`
}

// ArtifactUploadAction describes one action needed to upload an artifact or
// part of an artifact to Buildkite artifact storage.
type ArtifactUploadAction struct {
	URL        string `json:"url,omitempty"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	FileInput  string `json:"file_input"`
	PartNumber int    `json:"part_number,omitempty"`
}

type ArtifactBatchCreateResponse struct {
	ID          string   `json:"id"`
	ArtifactIDs []string `json:"artifact_ids"`

	// These instructions apply to all artifacts. The template contains
	// variable interpolations such as ${artifact:path}.
	InstructionsTemplate *ArtifactUploadInstructions `json:"upload_instructions"`

	// These instructions apply to specific artifacts, necessary for multipart
	// uploads. It overrides InstructionTemplate and should not contain
	// interpolations. Map: artifact ID -> instructions for that artifact.
	PerArtifactInstructions map[string]*ArtifactUploadInstructions `json:"per_artifact_instructions"`
}

// ArtifactSearchOptions specifies the optional parameters to the
// ArtifactsService.Search method.
type ArtifactSearchOptions struct {
	Query              string `url:"query,omitempty"`
	Scope              string `url:"scope,omitempty"`
	State              string `url:"state,omitempty"`
	IncludeRetriedJobs bool   `url:"include_retried_jobs,omitempty"`
	IncludeDuplicates  bool   `url:"include_duplicates,omitempty"`
}

// ArtifactState represents the state of a single artifact, when calling UpdateArtifacts.
type ArtifactState struct {
	ID        string `json:"id"`
	State     string `json:"state"`
	Multipart bool   `json:"multipart,omitempty"`
	// If this artifact was a multipart upload and is complete, we need the
	// the ETag from each uploaded part so that they can be joined together.
	MultipartETags []ArtifactPartETag `json:"multipart_etags,omitempty"`
}

// ArtifactPartETag associates an ETag to a part number for a multipart upload.
type ArtifactPartETag struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
}

type ArtifactBatchUpdateRequest struct {
	Artifacts []ArtifactState `json:"artifacts"`
}

// CreateArtifacts takes a slice of artifacts, and creates them on Buildkite as a batch.
func (c *Client) CreateArtifacts(ctx context.Context, jobID string, batch *ArtifactBatch) (*ArtifactBatchCreateResponse, *Response, error) {
	u := fmt.Sprintf("jobs/%s/artifacts", railsPathEscape(jobID))

	req, err := c.newRequest(ctx, "POST", u, batch)
	if err != nil {
		return nil, nil, err
	}

	createResponse := new(ArtifactBatchCreateResponse)
	resp, err := c.doRequest(req, createResponse)
	if err != nil {
		return nil, resp, err
	}

	return createResponse, resp, err
}

// UpdateArtifacts updates Buildkite with one or more artifact states.
func (c *Client) UpdateArtifacts(ctx context.Context, jobID string, artifactStates []ArtifactState) (*Response, error) {
	u := fmt.Sprintf("jobs/%s/artifacts", railsPathEscape(jobID))
	payload := ArtifactBatchUpdateRequest{
		Artifacts: artifactStates,
	}

	req, err := c.newRequest(ctx, "PUT", u, payload)
	if err != nil {
		return nil, err
	}

	return c.doRequest(req, nil)
}

// SearchArtifacts searches Buildkite for a set of artifacts
func (c *Client) SearchArtifacts(ctx context.Context, buildID string, opt *ArtifactSearchOptions) ([]*Artifact, *Response, error) {
	u := fmt.Sprintf("builds/%s/artifacts/search", railsPathEscape(buildID))
	u, err := addOptions(u, opt)
	if err != nil {
		return nil, nil, err
	}

	req, err := c.newRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	a := []*Artifact{}
	resp, err := c.doRequest(req, &a)
	if err != nil {
		return nil, resp, err
	}

	return a, resp, err
}
