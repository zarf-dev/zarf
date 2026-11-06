// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package oci

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

func TestRetryReader(t *testing.T) {
	data := []byte("each interrupted read advances its own cursor")
	desc := content.NewDescriptorFromBytes(ocispec.MediaTypeImageLayer, data)
	for _, tt := range []struct {
		name        string
		retries     int
		retryable   retry.Predicate
		maxConns    int
		disabled    bool
		noRanges    bool
		withdraw    bool
		corrupt     bool
		rangeStatus int
		wantErr     string
		wantRanges  []int
	}{
		{name: "repeated interruptions", retries: 2, wantRanges: []int{3, 7}},
		{name: "resumes without a spare connection", retries: 2, maxConns: 1, wantRanges: []int{3, 7}},
		{name: "disabled", disabled: true, wantErr: "unexpected EOF"},
		{name: "zero retries", wantErr: "unexpected EOF"},
		{name: "budget is not reset by progress", retries: 1, wantErr: "unexpected EOF", wantRanges: []int{3}},
		{name: "no range support", retries: 2, noRanges: true, wantErr: "unexpected EOF"},
		{name: "withdrawn range support", retries: 2, withdraw: true, wantErr: "unsupported"},
		{name: "rejected range is terminal", retries: 2, rangeStatus: http.StatusForbidden, wantErr: "403", wantRanges: []int{3}},
		{
			name:    "failed HTTP recovery is not retried by the reader",
			retries: 2,
			retryable: func(response *http.Response, err error) (bool, error) {
				return err != nil || response.StatusCode == http.StatusServiceUnavailable, nil
			},
			rangeStatus: http.StatusServiceUnavailable,
			wantErr:     "503",
			wantRanges:  []int{3, 3, 3},
		},
		{name: "ignored range is terminal", retries: 2, rangeStatus: http.StatusOK, wantErr: "unexpected status code 200", wantRanges: []int{3}},
		{name: "corruption cannot pass verification", retries: 2, corrupt: true, wantErr: "mismatched digest", wantRanges: []int{3, 7}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var fullGets atomic.Int32
			var mu sync.Mutex
			var ranges []int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				start, end := 0, len(data)
				if !tt.noRanges && (!tt.withdraw || fullGets.Load() == 0) {
					w.Header().Set("Accept-Ranges", "bytes")
				}
				if value := r.Header.Get("Range"); value != "" {
					_, err := fmt.Sscanf(value, "bytes=%d-", &start)
					assert.NoError(t, err)
					mu.Lock()
					ranges = append(ranges, start)
					mu.Unlock()
					if tt.rangeStatus != 0 {
						w.WriteHeader(tt.rangeStatus)
						return
					}
					w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(data)-1, len(data)))
					w.Header().Set("Content-Length", strconv.Itoa(len(data)-start))
					w.WriteHeader(http.StatusPartialContent)
					if start == 3 {
						end = 7
					}
				} else {
					w.Header().Set("Content-Length", strconv.Itoa(len(data)))
					if fullGets.Add(1) == 1 {
						end = 3
					}
				}
				payload := bytes.Clone(data[start:end])
				if tt.corrupt && start == 7 {
					payload[0] ^= 1
				}
				_, err := w.Write(payload)
				assert.NoError(t, err)
				if end < len(data) {
					assert.NoError(t, http.NewResponseController(w).Flush())
					panic(http.ErrAbortHandler)
				}
			}))
			defer server.Close()
			p := testRetryPolicy(tt.retries)
			if tt.retryable != nil {
				p.Retryable = tt.retryable
			}
			var policy retry.Policy = p
			if tt.disabled {
				policy = nil
			}
			remote, err := NewOrasRemote(strings.TrimPrefix(server.URL, "http://")+"/source", ocispec.Platform{},
				WithPlainHTTP(true), WithCredentialFunc(auth.StaticCredential("", auth.EmptyCredential)), WithRetryPolicy(policy),
				WithTransport(&http.Transport{MaxConnsPerHost: tt.maxConns}))
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			_, err = content.FetchAll(ctx, remote, desc)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
			mu.Lock()
			defer mu.Unlock()
			require.Equal(t, tt.wantRanges, ranges)
		})
	}
}

func TestRetryIndependentReaders(t *testing.T) {
	data := []byte("bytes returned with an error still advance each reader's cursor")
	desc := content.NewDescriptorFromBytes(ocispec.MediaTypeImageLayer, data)
	fullGets := 0
	var ranges []int
	transport := retryTestTransport(func(r *http.Request) (*http.Response, error) {
		status, size := http.StatusOK, len(data)
		body := io.NopCloser(bytes.NewReader(data))
		if value := r.Header.Get("Range"); value != "" {
			var offset int
			_, err := fmt.Sscanf(value, "bytes=%d-", &offset)
			require.NoError(t, err)
			ranges = append(ranges, offset)
			status, size, body = http.StatusPartialContent, len(data)-offset, io.NopCloser(bytes.NewReader(data[offset:]))
		} else {
			fullGets++
			if fullGets <= 2 {
				body = io.NopCloser(retryTestReadError{Reader: bytes.NewReader(data[:fullGets*3])})
			}
		}
		return &http.Response{StatusCode: status, ContentLength: int64(size), Header: http.Header{"Accept-Ranges": {"bytes"}}, Body: body, Request: r}, nil
	})
	remote, err := NewOrasRemote("registry.example.com/source", ocispec.Platform{},
		WithCredentialFunc(auth.StaticCredential("", auth.EmptyCredential)), WithRetryPolicy(testRetryPolicy(1)),
		func(o *OrasRemote) { o.progTransport.Base = transport })
	require.NoError(t, err)
	first, err := remote.Fetch(t.Context(), desc)
	require.NoError(t, err)
	defer func() { require.NoError(t, first.Close()) }()
	second, err := remote.Fetch(t.Context(), desc)
	require.NoError(t, err)
	defer func() { require.NoError(t, second.Close()) }()
	for _, reader := range []io.Reader{first, second} {
		got, err := content.ReadAll(reader, desc)
		require.NoError(t, err)
		require.Equal(t, data, got)
	}
	require.Equal(t, []int{3, 6}, ranges)
}

func TestRetryNativeHTTPPolicy(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		data := []byte("native HTTP retries retain Retry-After")
		desc := content.NewDescriptorFromBytes(ocispec.MediaTypeImageLayer, data)
		requests := 0
		transport := retryTestTransport(func(r *http.Request) (*http.Response, error) {
			requests++
			if requests == 1 {
				return &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{"Retry-After": {"60"}}, Body: http.NoBody, Request: r}, nil
			}
			return &http.Response{StatusCode: http.StatusOK, ContentLength: desc.Size, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(data)), Request: r}, nil
		})
		policy := testRetryPolicy(1)
		policy.Backoff, policy.MaxWait = retry.DefaultBackoff, time.Minute
		remote, err := NewOrasRemote("registry.example.com/source", ocispec.Platform{},
			WithCredentialFunc(auth.StaticCredential("", auth.EmptyCredential)), WithRetryPolicy(policy),
			func(o *OrasRemote) { o.progTransport.Base = transport })
		require.NoError(t, err)
		start := time.Now()
		got, err := content.FetchAll(t.Context(), remote, desc)
		require.NoError(t, err)
		require.Equal(t, data, got)
		require.Equal(t, 2, requests)
		require.Equal(t, time.Minute, time.Since(start))
	})
}

func TestRetryStreamingSinglePUT(t *testing.T) {
	data := []byte("source recovery must keep the destination upload streaming")
	desc := content.NewDescriptorFromBytes(ocispec.MediaTypeImageLayer, data)
	prefixDelivered := make(chan struct{})
	var fullGets, ranges, puts atomic.Int32
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		if value := r.Header.Get("Range"); value != "" {
			ranges.Add(1)
			assert.Equal(t, fmt.Sprintf("bytes=3-%d", len(data)-1), value)
			w.Header().Set("Content-Length", strconv.Itoa(len(data)-3))
			w.Header().Set("Content-Range", fmt.Sprintf("bytes 3-%d/%d", len(data)-1, len(data)))
			w.WriteHeader(http.StatusPartialContent)
			_, err := w.Write(data[3:])
			assert.NoError(t, err)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		if fullGets.Add(1) == 1 {
			_, err := w.Write(data[:3])
			assert.NoError(t, err)
			assert.NoError(t, http.NewResponseController(w).Flush())
			// A whole-blob buffer cannot release this gate.
			select {
			case <-prefixDelivered:
			case <-r.Context().Done():
			}
			panic(http.ErrAbortHandler)
		}
		_, err := w.Write(data)
		assert.NoError(t, err)
	}))
	defer source.Close()
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Header().Set("Location", "/upload/session")
			w.WriteHeader(http.StatusAccepted)
			return
		}
		assert.Equal(t, http.MethodPut, r.Method)
		puts.Add(1)
		digester := desc.Digest.Algorithm().Digester()
		_, err := io.CopyN(digester.Hash(), r.Body, 3)
		assert.NoError(t, err)
		close(prefixDelivered)
		_, err = io.Copy(digester.Hash(), r.Body)
		assert.NoError(t, err)
		assert.Equal(t, desc.Digest, digester.Digest())
		w.WriteHeader(http.StatusCreated)
	}))
	defer destination.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	src, err := NewOrasRemote(strings.TrimPrefix(source.URL, "http://")+"/source", ocispec.Platform{},
		WithPlainHTTP(true), WithCredentialFunc(auth.StaticCredential("", auth.EmptyCredential)), WithRetryPolicy(testRetryPolicy(1)))
	require.NoError(t, err)
	dst, err := remote.NewRepository(strings.TrimPrefix(destination.URL, "http://") + "/destination")
	require.NoError(t, err)
	dst.PlainHTTP, dst.Client = true, destination.Client()
	reader, err := src.Fetch(ctx, desc)
	require.NoError(t, err)
	defer func() { require.NoError(t, reader.Close()) }()
	require.NoError(t, dst.Blobs().Push(ctx, desc, reader))
	require.EqualValues(t, 1, puts.Load())
	require.EqualValues(t, 1, ranges.Load())
}

func TestRetryCancellation(t *testing.T) {
	for _, mode := range []string{"read", "reopen", "backoff"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			blocked := make(chan struct{})
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Accept-Ranges", "bytes")
				w.Header().Set("Content-Length", "20")
				if requests.Add(1) == 1 {
					_, err := w.Write([]byte("one"))
					assert.NoError(t, err)
					assert.NoError(t, http.NewResponseController(w).Flush())
					if mode != "read" {
						panic(http.ErrAbortHandler)
					}
				}
				close(blocked)
				<-r.Context().Done()
			}))
			defer func() { cancel(); server.Close() }()
			policy := testRetryPolicy(1)
			if mode == "backoff" {
				policy.Backoff = func(int, *http.Response) time.Duration { close(blocked); return time.Hour }
				policy.MaxWait = time.Hour
			}
			remote, err := NewOrasRemote(strings.TrimPrefix(server.URL, "http://")+"/source", ocispec.Platform{},
				WithPlainHTTP(true), WithCredentialFunc(auth.StaticCredential("", auth.EmptyCredential)), WithRetryPolicy(policy))
			require.NoError(t, err)
			reader, err := remote.Fetch(ctx, content.NewDescriptorFromBytes(ocispec.MediaTypeImageLayer, make([]byte, 20)))
			require.NoError(t, err)
			defer func() { require.NoError(t, reader.Close()) }()
			done := make(chan error, 1)
			go func() { _, err := io.Copy(io.Discard, reader); done <- err }()
			select {
			case <-blocked:
			case <-time.After(5 * time.Second):
				t.Fatal("source did not reach the blocked operation")
			}
			if mode == "read" {
				cancel()
			} else {
				require.NoError(t, reader.Close())
			}
			select {
			case err := <-done:
				require.ErrorIs(t, err, context.Canceled)
			case <-time.After(time.Second):
				t.Fatal("cancellation did not unblock the reader")
			}
		})
	}
}

func testRetryPolicy(retries int) *retry.GenericPolicy {
	return &retry.GenericPolicy{
		MaxRetry: retries,
		Retryable: func(response *http.Response, err error) (bool, error) {
			if err != nil {
				return errors.Is(err, io.ErrUnexpectedEOF), nil
			}
			return response.StatusCode == http.StatusServiceUnavailable || response.StatusCode == http.StatusTooManyRequests, nil
		},
		Backoff: func(int, *http.Response) time.Duration { return 0 },
	}
}

type retryTestTransport func(*http.Request) (*http.Response, error)

func (f retryTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type retryTestReadError struct{ io.Reader }

func (r retryTestReadError) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if n > 0 {
		err = io.ErrUnexpectedEOF
	}
	return n, err
}
