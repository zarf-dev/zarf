// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic utility functions.
package utils

import (
	"io"
	"net/http"
	"time"

	"oras.land/oras-go/v2/registry/remote/retry"
)

type progressBar interface {
	Add(int)
	Write([]byte) (n int, err error)
	Stop()
	Successf(format string, args ...interface{})
}

// Transport is an http.RoundTripper that keeps track of the in-flight
// request and add hooks to report upload progress.
type Transport struct {
	Base        http.RoundTripper
	ProgressBar progressBar
}

// NewTransport returns a custom transport that tracks an http.RoundTripper and a message.ProgressBar.
func NewTransport(base http.RoundTripper, bar progressBar) *Transport {
	return &Transport{
		Base:        base,
		ProgressBar: bar,
	}
}

// RoundTrip is mirrored from retry, but instead of calling retry's private t.roundTrip(), this uses
// our own which has interactions w/ message.ProgressBar
//
// https://github.com/oras-project/oras-go/blob/main/registry/remote/retry/client.go
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	policy := retry.DefaultPolicy
	attempt := 0
	for {
		resp, respErr := t.roundTrip(req)
		duration, err := policy.Retry(attempt, resp, respErr)
		if err != nil {
			if respErr == nil {
				resp.Body.Close()
			}
			return nil, err
		}
		if duration < 0 {
			return resp, respErr
		}

		// rewind the body if possible
		if req.Body != nil {
			if req.GetBody == nil {
				// body can't be rewound, so we can't retry
				return resp, respErr
			}
			body, err := req.GetBody()
			if err != nil {
				// failed to rewind the body, so we can't retry
				return resp, respErr
			}
			req.Body = body
		}

		// close the response body if needed
		if respErr == nil {
			resp.Body.Close()
		}

		timer := time.NewTimer(duration)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
		attempt++
	}
}

// roundTrip calls base roundtrip while keeping track of the current request.
// this is currently only used to track the progress of publishes, not pulls.
func (t *Transport) roundTrip(req *http.Request) (resp *http.Response, err error) {
	if req.Method != http.MethodHead && req.Body != nil && t.ProgressBar != nil {
		req.Body = io.NopCloser(io.TeeReader(req.Body, t.ProgressBar))
	}

	resp, err = t.Base.RoundTrip(req)

	if resp != nil && req.Method == http.MethodHead && err == nil && t.ProgressBar != nil {
		if resp.ContentLength > 0 {
			t.ProgressBar.Add(int(resp.ContentLength))
		}
	}
	return resp, err
}
