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

package httputil

import (
	"errors"
	"fmt"
	"io"
	"net/http"
)

// Client is an interface for a HTTP client.
// This interface is defined inside this package to prevent potential import
// loop.
type Client interface {
	// Do sends an HTTP request and returns an HTTP response.
	Do(*http.Request) (*http.Response, error)
}

// readSeekCloser seeks http body by starting new connections lazily on Read.
type readSeekCloser struct {
	client Client
	req    *http.Request
	rc     io.ReadCloser
	size   int64
	offset int64
	// rcOffset is the position of the next byte rc yields. rc can be read
	// directly only while rcOffset equals offset.
	rcOffset int64
	closed   bool
}

// NewReadSeekCloser returns a seeker to make the HTTP response seekable.
// Callers should ensure that the server supports Range request.
func NewReadSeekCloser(client Client, req *http.Request, respBody io.ReadCloser, size int64) io.ReadSeekCloser {
	return &readSeekCloser{
		client: client,
		req:    req,
		rc:     respBody,
		size:   size,
	}
}

// Read reads the content body and counts offset. If a previous Seek moved
// offset away from rcOffset, Read first starts a new connection to the remote
// at offset.
func (rsc *readSeekCloser) Read(p []byte) (n int, err error) {
	if rsc.closed {
		return 0, errors.New("read: already closed")
	}
	if rsc.offset != rsc.rcOffset {
		req := rsc.req.Clone(rsc.req.Context())
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", rsc.offset, rsc.size-1))
		resp, err := rsc.client.Do(req)
		if err != nil {
			return 0, fmt.Errorf("read: %s %q: %w", req.Method, req.URL, err)
		}
		if resp.StatusCode != http.StatusPartialContent {
			resp.Body.Close()
			return 0, fmt.Errorf("read: %s %q: unexpected status code %d", resp.Request.Method, resp.Request.URL, resp.StatusCode)
		}

		rsc.rc.Close()
		rsc.rc = resp.Body
		rsc.rcOffset = rsc.offset
	}

	n, err = rsc.rc.Read(p)
	rsc.offset += int64(n)
	rsc.rcOffset += int64(n)
	return n, err
}

// Seek sets the offset for the next Read without sending any request, so
// errors from the remote are reported by the next Read instead.
func (rsc *readSeekCloser) Seek(offset int64, whence int) (int64, error) {
	if rsc.closed {
		return 0, errors.New("seek: already closed")
	}
	switch whence {
	case io.SeekCurrent:
		offset += rsc.offset
	case io.SeekStart:
		// no-op
	case io.SeekEnd:
		offset += rsc.size
	default:
		return 0, errors.New("seek: invalid whence")
	}
	if offset < 0 {
		return 0, errors.New("seek: an attempt was made to move the pointer before the beginning of the content")
	}
	// Past the end there is nothing to request, so drop the body now and let
	// Read report EOF instead of sending an invalid Range.
	if offset >= rsc.size {
		rsc.rc.Close()
		rsc.rc = http.NoBody
		rsc.rcOffset = offset
	}
	rsc.offset = offset
	return offset, nil
}

// Close closes the content body.
func (rsc *readSeekCloser) Close() error {
	if rsc.closed {
		return nil
	}
	rsc.closed = true
	return rsc.rc.Close()
}
