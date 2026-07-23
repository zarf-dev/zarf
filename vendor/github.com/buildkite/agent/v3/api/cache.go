package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/buildkite/agent/v3/internal/agenthttp"
	"github.com/google/go-querystring/query"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// Cache API "not found" messages. The cache service returns HTTP 404 with one
// of these messages in the JSON body to indicate semantically-distinct cases.
const (
	CacheRegistryNotFound = "Cache registry not found"
	CacheEntryNotFound    = "Cache entry not found"
)

// ErrCacheEntryNotFound is reserved for callers that want a sentinel for the
// "no entry" condition. The cache methods themselves report it via the
// (resp, exists, err) return shape; this value is exported for parity.
var ErrCacheEntryNotFound = errors.New("cache entry not found")

var cacheTracer = otel.Tracer("github.com/buildkite/agent/v3/api/cache")

// CacheEntryCreateReq is the request body for creating a cache entry.
type CacheEntryCreateReq struct {
	Store        string   `json:"store"`
	Key          string   `json:"key"`
	FallbackKeys []string `json:"fallback_keys"`
	Compression  string   `json:"compression"`
	FileSize     int      `json:"file_size"`
	Digest       string   `json:"digest"`
	Paths        []string `json:"paths"`
	Platform     string   `json:"platform"`
	Pipeline     string   `json:"pipeline"`
	Branch       string   `json:"branch"`
	Organization string   `json:"owner"`
}

// CacheEntryRetrieveReq is the query for retrieving a cache entry.
type CacheEntryRetrieveReq struct {
	Key          string `url:"key"`
	Branch       string `url:"branch"`
	FallbackKeys string `url:"fallback_keys"`
}

// CacheEntryRetrieveResp describes the cache entry to download.
type CacheEntryRetrieveResp struct {
	Store                string    `json:"store"`
	Key                  string    `json:"key"`
	Fallback             bool      `json:"fallback"`
	StoreObjectName      string    `json:"store_object_name"`
	ExpiresAt            time.Time `json:"expires_at"`
	CompressionType      string    `json:"compression_type"`
	Multipart            bool      `json:"multipart"`
	DownloadInstructions []string  `json:"download_instructions"`
	Message              string    `json:"message"`
}

// CacheEntryCreateResp describes where and how to upload the new cache entry.
type CacheEntryCreateResp struct {
	UploadID           string   `json:"upload_id"`
	StoreObjectName    string   `json:"store_object_name"`
	Multipart          bool     `json:"multipart"`
	UploadInstructions []string `json:"upload_instructions"`
	Message            string   `json:"message"`
}

// CacheEntryPeekReq is the query for checking whether a cache entry exists.
type CacheEntryPeekReq struct {
	Key    string `url:"key"`
	Branch string `url:"branch"`
}

// CacheEntryPeekResp describes the cache entry returned by a peek.
type CacheEntryPeekResp struct {
	Store        string    `json:"store"`
	Digest       string    `json:"digest"`
	ExpiresAt    time.Time `json:"expires_at"`
	Compression  string    `json:"compression"`
	Message      string    `json:"message"`
	FileSize     int       `json:"file_size"`
	Paths        []string  `json:"paths"`
	Pipeline     string    `json:"pipeline"`
	Branch       string    `json:"branch"`
	Owner        string    `json:"owner"`
	Platform     string    `json:"platform"`
	Key          string    `json:"key"`
	FallbackKeys []string  `json:"fallback_keys"`
	CreatedAt    time.Time `json:"created_at"`
	AgentID      string    `json:"agent_id"`
	JobID        string    `json:"job_id"`
	BuildID      string    `json:"build_id"`
}

// CacheRegistryResp describes a configured cache registry.
type CacheRegistryResp struct {
	UUID  string `json:"uuid"`
	Name  string `json:"name"`
	Store string `json:"store"`
}

// CacheEntryCommitReq is the request body for committing a previously created cache entry.
type CacheEntryCommitReq struct {
	UploadID string `json:"upload_id"`
}

// CacheEntryCommitResp acknowledges a commit.
type CacheEntryCommitResp struct {
	Message string `json:"message"`
}

// CacheRegistry retrieves information about a cache registry.
func (c *Client) CacheRegistry(ctx context.Context, registry string) (CacheRegistryResp, *Response, error) {
	ctx, span := cacheTracer.Start(ctx, "Client.CacheRegistry")
	defer span.End()

	var cacheResp CacheRegistryResp

	req, err := c.newRequest(ctx, http.MethodGet, cachePath("/cache_registries/%s", registry), nil)
	if err != nil {
		return cacheResp, nil, cacheSpanErr(span, "failed to create request: %w", err)
	}

	apiResp, err := c.cacheDo(req, &cacheResp)
	if err != nil {
		return cacheResp, apiResp, cacheSpanErr(span, "%w", err)
	}
	if apiResp.StatusCode != http.StatusOK {
		return cacheResp, apiResp, cacheSpanErr(span, "failed to get cache registry: %s", apiResp.Status)
	}
	return cacheResp, apiResp, nil
}

// CacheEntryPeekExists checks whether a cache entry exists.
// Returns (resp, true, _, nil) on hit, (resp, false, _, nil) on miss (HTTP 404
// with CacheEntryNotFound), or (resp, false, _, err) on any other failure.
func (c *Client) CacheEntryPeekExists(ctx context.Context, registry string, peek CacheEntryPeekReq) (CacheEntryPeekResp, bool, *Response, error) {
	ctx, span := cacheTracer.Start(ctx, "Client.CacheEntryPeekExists")
	defer span.End()

	var cacheResp CacheEntryPeekResp

	path, err := cacheQueryPath("/cache_registries/%s/peek", registry, peek)
	if err != nil {
		return cacheResp, false, nil, cacheSpanErr(span, "%w", err)
	}

	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return cacheResp, false, nil, cacheSpanErr(span, "failed to create request: %w", err)
	}

	apiResp, err := c.cacheDo(req, &cacheResp)
	if err != nil {
		return cacheResp, false, apiResp, cacheSpanErr(span, "%w", err)
	}
	cacheResp, exists, err := interpretCacheResponse(span, apiResp, cacheResp)
	return cacheResp, exists, apiResp, err
}

// CacheEntryCreate creates a new cache entry and returns upload instructions.
func (c *Client) CacheEntryCreate(ctx context.Context, registry string, create CacheEntryCreateReq) (CacheEntryCreateResp, *Response, error) {
	ctx, span := cacheTracer.Start(ctx, "Client.CacheEntryCreate")
	defer span.End()

	var cacheResp CacheEntryCreateResp

	req, err := c.newRequest(ctx, http.MethodPut, cachePath("/cache_registries/%s/store", registry), &create)
	if err != nil {
		return cacheResp, nil, cacheSpanErr(span, "failed to create request: %w", err)
	}

	apiResp, err := c.cacheDo(req, &cacheResp)
	if err != nil {
		return cacheResp, apiResp, cacheSpanErr(span, "%w", err)
	}
	if apiResp.StatusCode != http.StatusOK {
		return cacheResp, apiResp, cacheSpanErr(span, "failed to save: %s", apiResp.Status)
	}
	return cacheResp, apiResp, nil
}

// CacheEntryCommit marks a previously created cache entry as committed.
func (c *Client) CacheEntryCommit(ctx context.Context, registry string, commit CacheEntryCommitReq) (CacheEntryCommitResp, *Response, error) {
	ctx, span := cacheTracer.Start(ctx, "Client.CacheEntryCommit")
	defer span.End()

	var cacheResp CacheEntryCommitResp

	req, err := c.newRequest(ctx, http.MethodPut, cachePath("/cache_registries/%s/commit", registry), &commit)
	if err != nil {
		return cacheResp, nil, cacheSpanErr(span, "failed to create request: %w", err)
	}

	apiResp, err := c.cacheDo(req, &cacheResp)
	if err != nil {
		return cacheResp, apiResp, cacheSpanErr(span, "%w", err)
	}
	if apiResp.StatusCode != http.StatusOK {
		return cacheResp, apiResp, cacheSpanErr(span, "failed to commit: %s", apiResp.Status)
	}
	return cacheResp, apiResp, nil
}

// CacheEntryRetrieve retrieves download instructions for a cache entry.
// Returns (resp, true, _, nil) on hit (possibly via a fallback key),
// (resp, false, _, nil) on miss, or (resp, false, _, err) on any other failure.
func (c *Client) CacheEntryRetrieve(ctx context.Context, registry string, retrieve CacheEntryRetrieveReq) (CacheEntryRetrieveResp, bool, *Response, error) {
	ctx, span := cacheTracer.Start(ctx, "Client.CacheEntryRetrieve")
	defer span.End()

	var cacheResp CacheEntryRetrieveResp

	path, err := cacheQueryPath("/cache_registries/%s/retrieve", registry, retrieve)
	if err != nil {
		return cacheResp, false, nil, cacheSpanErr(span, "%w", err)
	}

	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return cacheResp, false, nil, cacheSpanErr(span, "failed to create request: %w", err)
	}

	apiResp, err := c.cacheDo(req, &cacheResp)
	if err != nil {
		return cacheResp, false, apiResp, cacheSpanErr(span, "%w", err)
	}

	cacheResp, exists, err := interpretCacheResponse(span, apiResp, cacheResp)
	return cacheResp, exists, apiResp, err
}

// cachePath formats a cache API path with URL-safe escaping for path components.
func cachePath(format string, args ...any) string {
	escaped := make([]any, len(args))
	for i, a := range args {
		escaped[i] = url.PathEscape(fmt.Sprint(a))
	}
	return fmt.Sprintf(format, escaped...)
}

// cacheQueryPath formats a cache API path and appends url-tagged query params.
func cacheQueryPath(format, registry string, params any) (string, error) {
	q, err := query.Values(params)
	if err != nil {
		return "", fmt.Errorf("failed to marshal query params: %w", err)
	}
	return cachePath(format, registry) + "?" + q.Encode(), nil
}

// cacheDo dispatches req through the agent HTTP stack (debug-HTTP, trace-HTTP)
// and decodes a JSON body into resp. Unlike Client.doRequest, it does not
// treat non-2xx as an error — cache responses use 404 + a message to signal a
// miss, and callers want to inspect the status themselves.
func (c *Client) cacheDo(req *http.Request, resp any) (*Response, error) {
	httpResp, err := agenthttp.Do(c.logger, c.client, req,
		agenthttp.WithDebugHTTP(c.conf.DebugHTTP),
		agenthttp.WithTraceHTTP(c.conf.TraceHTTP),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to do request: %w", err)
	}
	defer httpResp.Body.Close()              //nolint:errcheck
	defer io.Copy(io.Discard, httpResp.Body) //nolint:errcheck

	apiResp := newResponse(httpResp)

	if httpResp.StatusCode >= 500 {
		return apiResp, fmt.Errorf("request failed with status: %s", httpResp.Status)
	}
	if httpResp.Body == http.NoBody {
		return apiResp, nil
	}

	contentType := httpResp.Header.Get("Content-Type")
	if !isJSONContent(contentType) {
		return apiResp, fmt.Errorf("unexpected content type: %s", contentType)
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return apiResp, fmt.Errorf("failed to read response body: %w", err)
	}
	if len(body) == 0 {
		return apiResp, nil
	}
	if err := json.Unmarshal(body, resp); err != nil {
		return apiResp, fmt.Errorf("failed to decode response body: %w", err)
	}
	return apiResp, nil
}

// cacheMessage is the subset of cache responses needed to classify a 404.
type cacheMessage interface {
	cacheMessage() string
}

func (r CacheEntryPeekResp) cacheMessage() string     { return r.Message }
func (r CacheEntryRetrieveResp) cacheMessage() string { return r.Message }

// interpretCacheResponse maps the dual "200 = hit, 404 + message = miss"
// convention into the (resp, exists, err) return shape used by peek/retrieve.
func interpretCacheResponse[T cacheMessage](span oteltrace.Span, apiResp *Response, cacheResp T) (T, bool, error) {
	if apiResp.StatusCode == http.StatusOK {
		return cacheResp, true, nil
	}
	switch apiResp.StatusCode {
	case http.StatusNotFound:
		switch cacheResp.cacheMessage() {
		case CacheEntryNotFound:
			return cacheResp, false, nil
		case CacheRegistryNotFound:
			return cacheResp, false, cacheSpanErr(span, "cache registry not found: %s", apiResp.Status)
		}
		return cacheResp, false, cacheSpanErr(span, "not found: %s", apiResp.Status)
	case http.StatusBadRequest:
		return cacheResp, false, cacheSpanErr(span, "bad request: %s", apiResp.Status)
	default:
		return cacheResp, false, cacheSpanErr(span, "request failed with status: %s", apiResp.Status)
	}
}

func cacheSpanErr(span oteltrace.Span, format string, args ...any) error {
	err := fmt.Errorf(format, args...)
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	return err
}

// isJSONContent reports whether contentType represents JSON, including media
// types with suffix (e.g. application/problem+json) or parameters (e.g.
// application/json; charset=utf-8).
func isJSONContent(contentType string) bool {
	contentType = strings.TrimSpace(strings.ToLower(contentType))
	if i := strings.Index(contentType, ";"); i != -1 {
		contentType = strings.TrimSpace(contentType[:i])
	}
	return contentType == "application/json" ||
		strings.HasPrefix(contentType, "application/") && strings.HasSuffix(contentType, "+json")
}
