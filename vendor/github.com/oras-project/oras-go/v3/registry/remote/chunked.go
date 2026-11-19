/*
Copyright The ORAS Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package remote

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/oras-project/oras-go/v3/registry/remote/internal/errutil"
)

// headerOCIChunkMinLength is the response header a registry uses to advertise
// the minimum size, in bytes, of every chunk except the last.
// Reference: https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#pushing-a-blob-in-chunks
const headerOCIChunkMinLength = "OCI-Chunk-Min-Length"

// maxChunkMinLength is the largest registry-advertised OCI-Chunk-Min-Length
// honored when it raises the caller's chunk size. Each chunk is buffered in
// memory, so a larger advertised minimum makes the push fall back to a
// monolithic upload, which streams the content without buffering.
const maxChunkMinLength int64 = 32 * 1024 * 1024 // 32 MiB

// errChunkedUploadNotStarted signals that a chunked upload failed before any
// byte of the content was consumed. The caller may safely fall back to a
// monolithic upload with the original reader.
var errChunkedUploadNotStarted = errors.New("chunked blob upload not started")

// chunkedUpload holds the mutable state of a single chunked upload session.
type chunkedUpload struct {
	// location is the next URL to PATCH/PUT, updated after each request.
	location *url.URL
	// chunk is the effective chunk size, raised to any registry-advertised
	// OCI-Chunk-Min-Length.
	chunk int64
	// offset is the number of bytes uploaded so far (the next Content-Range
	// start).
	offset int64
	// digester computes the digest of the streamed content.
	digester digest.Digester
	// auth is the Authorization header reused across requests in the session.
	auth string
	// origin is the registry URL the session was opened on; auth is reused only on its host.
	origin *url.URL
}

// pushChunked uploads expected in chunks per the OCI Distribution Spec: it opens
// an upload session (POST), streams content in PATCH requests of the effective
// chunk size, and closes the session with a final PUT carrying the digest.
//
// If the session cannot be opened before any content is consumed, it returns an
// error wrapping [errChunkedUploadNotStarted] so the caller can fall back to a
// monolithic upload. Once the first PATCH is issued the reader is consumed and
// no fallback is possible; later failures return a wrapped error and
// best-effort cancel the session.
//
// Reference: https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#pushing-a-blob-in-chunks
func (s *blobStore) pushChunked(ctx context.Context, expected ocispec.Descriptor, content io.Reader, chunkSize int64) error {
	up, err := s.openUploadSession(ctx, chunkSize)
	if err != nil {
		// The session never opened; no bytes consumed, fallback is safe.
		return fmt.Errorf("%w: %w", errChunkedUploadNotStarted, err)
	}

	// Digest with the descriptor's own algorithm, not the canonical one. The
	// allowlist is checked before the digester is built, so an algorithm this
	// package does not act on cannot consume the upload session.
	if err := checkDigestAlgorithm("chunked blob push", expected.Digest); err != nil {
		s.cancelUpload(ctx, up.location)
		return err
	}
	algo := expected.Digest.Algorithm()
	up.digester = algo.Digester()

	// A chunk at least as large as the blob means a lone PATCH plus the closing
	// PUT, which is strictly worse than a monolithic POST/PUT, and it would size
	// the read buffer at the whole blob however large the registry's advertised
	// minimum. Nothing is consumed yet, so release the session and fall back.
	if up.chunk >= expected.Size {
		s.cancelUpload(ctx, up.location)
		return fmt.Errorf("%w: effective chunk size %d is not smaller than blob size %d", errChunkedUploadNotStarted, up.chunk, expected.Size)
	}

	// Each chunk is buffered in memory, so a registry-advertised minimum must not
	// raise the caller's chunk size without bound and defeat the limit the caller
	// set MaxChunkSize to establish. Nothing is consumed yet, so release the
	// session and fall back to a monolithic upload, which does not buffer.
	if up.chunk > chunkSize && up.chunk > maxChunkMinLength {
		s.cancelUpload(ctx, up.location)
		return fmt.Errorf("%w: advertised minimum chunk size %d exceeds limit %d", errChunkedUploadNotStarted, up.chunk, maxChunkMinLength)
	}

	// Bound the read at the declared size so an over-long reader cannot push
	// more bytes than the descriptor claims.
	if err := s.uploadChunks(ctx, up, io.LimitReader(content, expected.Size)); err != nil {
		return err
	}

	if computed := up.digester.Digest(); computed != expected.Digest {
		s.cancelUpload(ctx, up.location)
		return fmt.Errorf("chunked blob push: content digest %s does not match expected digest %s", computed, expected.Digest)
	}
	if up.offset != expected.Size {
		s.cancelUpload(ctx, up.location)
		return fmt.Errorf("chunked blob push: uploaded %d bytes, expected %d", up.offset, expected.Size)
	}

	return s.closeUploadSession(ctx, up, expected.Digest)
}

// openUploadSession performs the POST that starts a blob upload and returns the
// initialised session state. The returned session has its chunk size raised to
// any registry-advertised OCI-Chunk-Min-Length.
func (s *blobStore) openUploadSession(ctx context.Context, chunkSize int64) (*chunkedUpload, error) {
	url := buildRepositoryBlobUploadURL(s.repo.plainHTTP(), s.repo.reference())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.ContentLength = 0

	resp, err := s.repo.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		return nil, errutil.ParseErrorResponse(resp)
	}
	location, err := resolveUploadLocation(resp, req)
	if err != nil {
		return nil, err
	}
	if minLen := parseChunkMinLength(resp); minLen > chunkSize {
		chunkSize = minLen
	}
	var authHeader string
	if resp.Request != nil {
		authHeader = resp.Request.Header.Get("Authorization")
	}
	return &chunkedUpload{
		location: location,
		chunk:    chunkSize,
		auth:     authHeader,
		origin:   req.URL,
	}, nil
}

// uploadChunks streams content in PATCH requests of up.chunk bytes, updating the
// running digest, byte offset, and push location as it goes. The session is
// already open on entry, so every failure path cancels it.
func (s *blobStore) uploadChunks(ctx context.Context, up *chunkedUpload, content io.Reader) error {
	hasher := up.digester.Hash()
	buf := make([]byte, up.chunk)
	for {
		n, readErr := io.ReadFull(content, buf)
		if readErr == io.EOF {
			// Clean end of stream on a chunk boundary.
			return nil
		}
		if readErr != nil && readErr != io.ErrUnexpectedEOF {
			s.cancelUpload(ctx, up.location)
			return fmt.Errorf("chunked blob push: failed to read content after %d bytes: %w", up.offset, readErr)
		}
		if n == 0 {
			return nil
		}
		_, _ = hasher.Write(buf[:n])

		req, err := http.NewRequestWithContext(ctx, http.MethodPatch, up.location.String(), bytes.NewReader(buf[:n]))
		if err != nil {
			s.cancelUpload(ctx, up.location)
			return fmt.Errorf("chunked blob push: failed to build PATCH request: %w", err)
		}
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("Content-Range", fmt.Sprintf("%d-%d", up.offset, up.offset+int64(n)-1))
		req.ContentLength = int64(n)
		resp, err := s.sendChunkRequest(up, req, func() (*http.Request, error) {
			retry, rerr := http.NewRequestWithContext(ctx, http.MethodPatch, up.location.String(), bytes.NewReader(buf[:n]))
			if rerr != nil {
				return nil, rerr
			}
			retry.Header.Set("Content-Type", "application/octet-stream")
			retry.Header.Set("Content-Range", fmt.Sprintf("%d-%d", up.offset, up.offset+int64(n)-1))
			retry.ContentLength = int64(n)
			return retry, nil
		})
		if err != nil {
			s.cancelUpload(ctx, up.location)
			return fmt.Errorf("chunked blob push: PATCH failed after %d bytes: %w", up.offset, err)
		}
		if resp.StatusCode != http.StatusAccepted {
			respErr := errutil.ParseErrorResponse(resp)
			resp.Body.Close()
			s.cancelUpload(ctx, up.location)
			return fmt.Errorf("chunked blob push: unexpected PATCH status %d after %d bytes: %w", resp.StatusCode, up.offset, respErr)
		}
		next, err := resolveUploadLocation(resp, req)
		rangeHeader := resp.Header.Get("Range")
		resp.Body.Close()
		if err != nil {
			s.cancelUpload(ctx, up.location)
			return fmt.Errorf("chunked blob push: invalid PATCH location after %d bytes: %w", up.offset, err)
		}
		// The registry echoes the total range it has stored so far as
		// "Range: 0-<end>" (some registries emit the HTTP "bytes=0-<end>"
		// form). When present and parsable, its end must match the last byte
		// we just sent; a mismatch means the registry stored a different amount
		// than we streamed and every later Content-Range would drift. Fail fast
		// naming both offsets. An absent or unparsable header is tolerated,
		// leaving behavior as it was before this check.
		if end, ok := parseRangeEnd(rangeHeader); ok {
			if want := up.offset + int64(n) - 1; end != want {
				s.cancelUpload(ctx, next)
				return fmt.Errorf("chunked blob push: registry stored up to byte %d, expected %d after %d bytes", end, want, up.offset)
			}
		}
		up.location = next
		up.offset += int64(n)
	}
}

// sendChunkRequest sends req, reusing the session Authorization header when set.
// A cached header lets auth.Client.Do skip its challenge round trip, but it also
// suppresses token refresh (auth/client.go returns early when Authorization is
// present). Registry tokens routinely expire mid-upload, so on a 401 the header
// is dropped and the request is rebuilt and retried once, letting the auth
// client re-authenticate. The refreshed token is captured back into up.auth for
// the remaining requests. rebuild must return an equivalent request carrying no
// Authorization header (the body is re-supplied from the caller's buffer).
func (s *blobStore) sendChunkRequest(up *chunkedUpload, req *http.Request, rebuild func() (*http.Request, error)) (*http.Response, error) {
	reuseAuth := up.auth != "" && sameUploadHost(req.URL, up.origin)
	if reuseAuth {
		req.Header.Set("Authorization", up.auth)
	}
	resp, err := s.repo.do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized && reuseAuth {
		// The cached token was rejected; drop it and let the auth client
		// re-authenticate on a fresh request.
		resp.Body.Close()
		up.auth = ""
		retry, rerr := rebuild()
		if rerr != nil {
			return nil, rerr
		}
		resp, err = s.repo.do(retry)
		if err != nil {
			return nil, err
		}
	}
	// Carry the (possibly refreshed) Authorization header forward.
	if resp.Request != nil && sameUploadHost(resp.Request.URL, up.origin) {
		if auth := resp.Request.Header.Get("Authorization"); auth != "" {
			up.auth = auth
		}
	}
	return resp, nil
}

// closeUploadSession issues the final PUT that commits the blob under
// finalDigest and verifies the registry-reported digest when present.
func (s *blobStore) closeUploadSession(ctx context.Context, up *chunkedUpload, finalDigest digest.Digest) error {
	// Keep the bare session URL for cancellation: the commit URL below carries
	// the digest query, which has no meaning on a DELETE.
	sessionURL := *up.location

	q := up.location.Query()
	q.Set("digest", finalDigest.String())
	up.location.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, up.location.String(), http.NoBody)
	if err != nil {
		s.cancelUpload(ctx, &sessionURL)
		return fmt.Errorf("chunked blob push: failed to build PUT request: %w", err)
	}
	req.ContentLength = 0
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := s.sendChunkRequest(up, req, func() (*http.Request, error) {
		retry, rerr := http.NewRequestWithContext(ctx, http.MethodPut, up.location.String(), http.NoBody)
		if rerr != nil {
			return nil, rerr
		}
		retry.ContentLength = 0
		retry.Header.Set("Content-Type", "application/octet-stream")
		return retry, nil
	})
	if err != nil {
		s.cancelUpload(ctx, &sessionURL)
		return fmt.Errorf("chunked blob push: PUT failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		respErr := errutil.ParseErrorResponse(resp)
		s.cancelUpload(ctx, &sessionURL)
		return fmt.Errorf("chunked blob push: unexpected PUT status %d: %w", resp.StatusCode, respErr)
	}
	// The blob is committed once the PUT returns 201; the session is gone, so a
	// digest mismatch here needs no cancellation.
	return verifyPushContentDigest(resp, finalDigest, nil)
}

// cancelUpload issues a best-effort DELETE to release an in-progress upload
// session. Per the OCI spec, clients SHOULD ignore any failures.
// Reference: https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#canceling-a-blob-upload
func (s *blobStore) cancelUpload(ctx context.Context, location *url.URL) {
	// Detach from ctx: cancellation or deadline expiry is exactly when the
	// session most needs releasing.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, location.String(), http.NoBody)
	if err != nil {
		return
	}
	resp, err := s.repo.do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

// parseChunkMinLength reads the registry-advertised OCI-Chunk-Min-Length header
// from a session response, returning 0 when absent, unparsable, or negative.
func parseChunkMinLength(resp *http.Response) int64 {
	v := resp.Header.Get(headerOCIChunkMinLength)
	if v == "" {
		return 0
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// parseRangeEnd reads the inclusive end byte from a registry's Range header on a
// chunked-upload 202 response. The OCI Distribution Spec uses "0-<end>"; some
// registries emit the HTTP-conventional "bytes=0-<end>". It returns ok=false
// when the header is absent or cannot be parsed, so callers treat an
// unparsable Range as absent rather than fatal.
func parseRangeEnd(v string) (int64, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	v = strings.TrimPrefix(v, "bytes=")
	dash := strings.LastIndex(v, "-")
	if dash < 0 {
		return 0, false
	}
	end, err := strconv.ParseInt(strings.TrimSpace(v[dash+1:]), 10, 64)
	if err != nil || end < 0 {
		return 0, false
	}
	return end, true
}
