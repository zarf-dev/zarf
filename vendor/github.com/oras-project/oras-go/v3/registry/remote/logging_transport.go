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
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"sync/atomic"
)

// sensitiveHeaders is the set of headers whose values are scrubbed from logs.
var sensitiveHeaders = []string{
	"Authorization",
	"Cookie",
	"Proxy-Authorization",
	"Set-Cookie",
}

// loggingPayloadSizeLimit is the maximum number of response body bytes printed.
const loggingPayloadSizeLimit int64 = 16 * 1024 // 16 KiB

// requestCounter assigns a unique sequential ID to each logged
// request/response pair. Because oras-go performs concurrent HTTP requests
// (e.g. parallel blob fetches), the ID lets callers correlate a request log
// line with its corresponding response log line when output is interleaved.
var requestCounter atomic.Uint64

// LoggingTransport is an http.RoundTripper that logs every request and its
// response at slog.LevelDebug. It is safe for concurrent use.
//
// Usage:
//
//	reg := remote.NewRegistry("example.com")
//	reg.Client = &auth.Client{
//	    Client: &http.Client{
//	        Transport: remote.NewLoggingTransport(http.DefaultTransport, nil),
//	    },
//	}
type LoggingTransport struct {
	inner  http.RoundTripper
	logger *slog.Logger
}

// NewLoggingTransport wraps inner with request/response debug logging.
// If logger is nil, slog.Default() is used. If inner is nil,
// http.DefaultTransport is used.
func NewLoggingTransport(inner http.RoundTripper, logger *slog.Logger) *LoggingTransport {
	if inner == nil {
		inner = http.DefaultTransport
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &LoggingTransport{inner: inner, logger: logger}
}

// RoundTrip implements http.RoundTripper, logging the request and response.
func (t *LoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !t.logger.Enabled(req.Context(), slog.LevelDebug) {
		return t.inner.RoundTrip(req)
	}

	id := requestCounter.Add(1) - 1
	loggedURL := *req.URL
	loggedURL.RawQuery = ""
	loggedURL.ForceQuery = false
	loggedURL.Fragment = ""
	loggedURL.RawFragment = ""

	t.logger.Debug(req.Method,
		"id", id,
		"url", loggedURL.Redacted(),
		"header", formatHeaders(req.Header),
	)

	resp, err := t.inner.RoundTrip(req)
	if err != nil {
		t.logger.Debug("Response",
			"id", id,
			"error", err,
		)
		return resp, err
	}

	t.logger.Debug("Response",
		"id", id,
		"status", resp.Status,
		"header", formatHeaders(resp.Header),
		"body", formatResponseBody(resp),
	)
	return resp, nil
}

// formatHeaders returns a human-readable representation of the headers with
// sensitive values replaced by "*****".
func formatHeaders(header http.Header) string {
	if len(header) == 0 {
		return "   Empty header"
	}
	var parts []string
	for k, v := range header {
		val := strings.Join(v, ", ")
		for _, sensitive := range sensitiveHeaders {
			if strings.EqualFold(k, sensitive) {
				val = "*****"
				break
			}
		}
		parts = append(parts, fmt.Sprintf("   %q: %q", k, val))
	}
	return strings.Join(parts, "\n")
}

// formatResponseBody reads up to loggingPayloadSizeLimit bytes from the
// response body and returns them as a string, then restores resp.Body so
// subsequent reads by the caller see the full body. Non-printable content
// types and bodies containing potential credentials are not printed.
func formatResponseBody(resp *http.Response) string {
	if resp.Body == nil || resp.Body == http.NoBody {
		return "   No response body"
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		return "   Response body without content type not printed"
	}
	if !isLoggableContentType(contentType) {
		return fmt.Sprintf("   Response body of content type %q not printed", contentType)
	}

	// Read up to the limit into buf, then restore resp.Body by prepending
	// the already-read bytes ahead of the remaining body.
	var buf bytes.Buffer
	if _, err := io.CopyN(&buf, resp.Body, loggingPayloadSizeLimit+1); err != nil && err != io.EOF {
		return fmt.Sprintf("   Error reading response body: %v", err)
	}

	// Restore: callers see buf contents followed by any unread remainder.
	remaining := resp.Body
	resp.Body = struct {
		io.Reader
		io.Closer
	}{
		Reader: io.MultiReader(bytes.NewReader(buf.Bytes()), remaining),
		Closer: remaining,
	}

	body := buf.String()
	if len(body) == 0 {
		return "   Response body is empty"
	}
	if containsCredentialFields(body) {
		return "   Response body redacted (potential credentials)"
	}
	if int64(len(body)) > loggingPayloadSizeLimit {
		return body[:loggingPayloadSizeLimit] + "\n...(truncated)"
	}
	return body
}

// isLoggableContentType returns true for JSON and plain-text content types.
func isLoggableContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	switch mediaType {
	case "application/json", "text/plain", "text/html":
		return true
	}
	return strings.HasSuffix(mediaType, "+json")
}

// credentialFieldKeys are JSON object keys that may hold secrets. Matching is
// done on the quoted key, which is enough to exclude near-misses such as
// "token_type".
var credentialFieldKeys = []string{
	`"token"`,
	`"access_token"`,
	`"refresh_token"`,
	`"id_token"`,
	`"client_secret"`,
	`"password"`,
	`"secret"`,
}

// containsCredentialFields returns true if the body appears to contain
// authentication tokens that should not be logged.
func containsCredentialFields(body string) bool {
	lower := strings.ToLower(body)
	for _, key := range credentialFieldKeys {
		if strings.Contains(lower, key) {
			return true
		}
	}
	return false
}
