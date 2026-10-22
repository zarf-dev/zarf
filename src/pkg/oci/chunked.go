// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package oci

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/errcode"
)

// PushChunkSize bounds upload request bodies and replay buffers. At the default
// concurrency of six, chunk buffers use at most 96 MiB.
const PushChunkSize int64 = 16 * 1024 * 1024

// chunkedTarget adds chunked blob uploads until ORAS v3 provides them natively.
// Embedding oras.Target leaves manifests, tags, and reads with ORAS, without
// exposing Repository.Mount, whose fallback would upload monolithically.
type chunkedTarget struct {
	oras.Target
	repo      *remote.Repository
	chunkSize int64
}

// NewChunkedTarget uploads blobs larger than PushChunkSize in OCI PATCH chunks.
// Smaller blobs and manifests retain ORAS's monolithic upload behavior.
func NewChunkedTarget(repo *remote.Repository) oras.Target {
	return &chunkedTarget{Target: repo, repo: repo, chunkSize: PushChunkSize}
}

func (t *chunkedTarget) Push(ctx context.Context, desc ocispec.Descriptor, content io.Reader) (err error) {
	manifestTypes := t.repo.ManifestMediaTypes
	if len(manifestTypes) == 0 {
		manifestTypes = []string{ocispec.MediaTypeImageManifest, ocispec.MediaTypeImageIndex,
			"application/vnd.docker.distribution.manifest.v2+json",
			"application/vnd.docker.distribution.manifest.list.v2+json",
			"application/vnd.oci.artifact.manifest.v1+json"}
	}
	if desc.Size <= t.chunkSize || slices.Contains(manifestTypes, desc.MediaType) {
		return t.Target.Push(ctx, desc, content)
	}
	if err := desc.Digest.Validate(); err != nil {
		return err
	}
	ctx = auth.AppendRepositoryScope(ctx, t.repo.Reference, auth.ActionPull, auth.ActionPush)
	scheme := "https"
	if t.repo.PlainHTTP {
		scheme = "http"
	}
	location := &url.URL{Scheme: scheme, Host: t.repo.Reference.Registry, Path: "/v2/" + t.repo.Reference.Repository + "/blobs/uploads/"}
	origin := *location
	resp, err := t.request(ctx, http.MethodPost, location, nil, "", http.StatusAccepted)
	if err != nil {
		return err
	}
	location, err = uploadLocation(resp, &origin)
	closeErr := resp.Body.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	active := true
	defer func() {
		if err != nil && active {
			// Do not leave an unfinished session behind, even after cancellation.
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			if response, cleanupErr := t.request(cleanupCtx, http.MethodDelete, location, nil, "", http.StatusNoContent); cleanupErr == nil {
				defer response.Body.Close() //nolint:errcheck // Cleanup must preserve the original upload error.
			}
		}
	}()
	// A registry may require larger non-final chunks. Fall back before reading
	// content rather than letting an untrusted header enlarge our replay buffer.
	fallback := func(reader io.Reader) error {
		response, cancelErr := t.request(ctx, http.MethodDelete, location, nil, "", http.StatusNoContent)
		if cancelErr != nil {
			logger.From(ctx).Debug("unable to cancel chunked upload before monolithic fallback", "error", cancelErr)
		} else if closeErr := response.Body.Close(); closeErr != nil {
			logger.From(ctx).Debug("unable to close upload cancellation response", "error", closeErr)
		}
		active = false
		return t.Target.Push(ctx, desc, reader)
	}
	if minimum, parseErr := strconv.ParseInt(resp.Header.Get("OCI-Chunk-Min-Length"), 10, 64); parseErr == nil && minimum > t.chunkSize {
		return fallback(content)
	}
	buffer := make([]byte, t.chunkSize)
	for offset := int64(0); offset < desc.Size; {
		size := min(t.chunkSize, desc.Size-offset)
		if _, err := io.ReadFull(content, buffer[:size]); err != nil {
			return fmt.Errorf("reading upload chunk at byte %d: %w", offset, err)
		}
		resp, err := t.request(ctx, http.MethodPatch, location, buffer[:size], fmt.Sprintf("%d-%d", offset, offset+size-1), http.StatusAccepted)
		if err != nil {
			var responseErr *errcode.ErrorResponse
			// Generic 400/404 responses can indicate invalid requests or expired
			// sessions, rather than unsupported PATCH uploads.
			if offset == 0 && errors.As(err, &responseErr) && (responseErr.StatusCode == http.StatusMethodNotAllowed || responseErr.StatusCode == http.StatusNotImplemented) {
				return fallback(io.MultiReader(bytes.NewReader(buffer[:size]), content))
			}
			return err
		}
		next, locationErr := uploadLocation(resp, &origin)
		closeErr := resp.Body.Close()
		if locationErr != nil {
			return locationErr
		}
		location = next
		if closeErr != nil {
			return closeErr
		}
		if stored := resp.Header.Get("Range"); stored != "" && stored != fmt.Sprintf("0-%d", offset+size-1) && stored != fmt.Sprintf("bytes=0-%d", offset+size-1) {
			return fmt.Errorf("registry acknowledged unexpected upload range %q", stored)
		}
		offset += size
	}
	commit := *location
	query := commit.Query()
	query.Set("digest", desc.Digest.String())
	commit.RawQuery = query.Encode()
	resp, err = t.request(ctx, http.MethodPut, &commit, nil, "", http.StatusCreated)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

func (t *chunkedTarget) request(ctx context.Context, method string, location *url.URL, body []byte, byteRange string, status int) (*http.Response, error) {
	// A bytes.Reader gives auth.Client and the retry transport a replayable body.
	req, err := http.NewRequestWithContext(ctx, method, location.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if method == http.MethodPatch || method == http.MethodPut {
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	if byteRange != "" {
		req.Header.Set("Content-Range", byteRange)
	}
	client := t.repo.Client
	if client == nil {
		client = auth.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != status {
		defer resp.Body.Close() //nolint:errcheck // Preserve the registry error if closing its response fails.
		var envelope struct {
			Errors errcode.Errors `json:"errors"`
		}
		// Keep registry error details without reading an unbounded error body.
		if decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&envelope); decodeErr != nil {
			envelope.Errors = nil
		}
		return nil, &errcode.ErrorResponse{Method: method, URL: location, StatusCode: resp.StatusCode, Errors: envelope.Errors}
	}
	return resp, nil
}

func uploadLocation(resp *http.Response, origin *url.URL) (*url.URL, error) {
	location, err := resp.Location()
	if err != nil {
		return nil, fmt.Errorf("invalid upload Location: %w", err)
	}
	port := func(u *url.URL) string {
		if p := u.Port(); p != "" {
			return p
		}
		if u.Scheme == "https" {
			return "443"
		}
		return "80"
	}
	if location.User != nil || location.Scheme != origin.Scheme || location.Hostname() != origin.Hostname() || port(location) != port(origin) {
		return nil, fmt.Errorf("upload Location must use the registry origin %s://%s", origin.Scheme, origin.Host)
	}
	return location, nil
}
