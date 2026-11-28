// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package oci

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/retry"
)

type retrySource struct {
	*remote.Repository
	policy retry.Policy
}

// Fetch delegates HTTP retries to ORAS and the configured policy.
// It adds no outer request retry loop.
func (s *retrySource) Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	ctx, cancel := context.WithCancel(ctx)
	body, err := s.Repository.Fetch(ctx, desc)
	if err != nil {
		cancel()
		return nil, err
	}
	// ORAS makes blobs advertising Accept-Ranges: bytes seekable, but not manifests.
	// Non-seekable bodies keep HTTP retries without resuming or replaying partial reads.
	policy := s.policy
	if _, ok := body.(io.Seeker); !ok {
		policy = nil
	}
	return &retryReader{ctx: ctx, cancel: cancel, source: s.Repository, desc: desc, policy: policy, body: body}, nil
}

// retryReader tracks bytes returned to its consumer, not bytes sent by the server
// or committed by the destination.
type retryReader struct {
	ctx    context.Context
	cancel context.CancelFunc
	source *remote.Repository
	desc   ocispec.Descriptor
	policy retry.Policy

	mu      sync.Mutex
	body    io.ReadCloser
	offset  int64
	attempt int
	err     error
	done    bool
}

func (r *retryReader) Close() error {
	r.cancel() // Unblock a read or reopen before taking the lock.
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.body == nil {
		return nil
	}
	err := r.body.Close()
	r.body = nil
	return err
}

func (r *retryReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(p) == 0 {
		return 0, nil
	}
	for {
		if err := r.ctx.Err(); err != nil {
			return 0, err
		}
		if r.done {
			return 0, r.err
		}
		if r.err != nil {
			if err := r.resume(); err != nil {
				r.err, r.done = err, true
				return 0, err
			}
		}
		n, err := r.body.Read(p)
		r.offset += int64(n)
		if err == io.EOF && r.offset < r.desc.Size {
			err = io.ErrUnexpectedEOF
		}
		r.err = err
		if err == io.EOF || r.offset >= r.desc.Size || r.policy == nil {
			r.done = err != nil
			return n, err
		}
		if n > 0 || err == nil {
			return n, nil
		}
	}
}

func (r *retryReader) resume() error {
	delay, err := r.policy.Retry(r.attempt, nil, r.err)
	if err != nil {
		return err
	}
	if delay < 0 {
		return r.err
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-r.ctx.Done():
		return r.ctx.Err()
	case <-timer.C:
	}
	r.attempt++
	if err := r.body.Close(); err != nil {
		return errors.Join(r.err, err)
	}
	r.body = nil
	r.body, err = r.source.Fetch(r.ctx, r.desc)
	if err != nil || r.offset == 0 {
		return err
	}
	seeker, ok := r.body.(io.Seeker)
	if !ok {
		return fmt.Errorf("blob %s cannot resume at %d: %w", r.desc.Digest, r.offset, errors.ErrUnsupported)
	}
	// ORAS closes the current response when seeking to the end, freeing its
	// connection before the next seek opens a range request at the saved offset.
	if _, err := seeker.Seek(0, io.SeekEnd); err != nil {
		return err
	}
	_, err = seeker.Seek(r.offset, io.SeekStart)
	return err
}
