// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides image-related utilities for Zarf
package images

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry"
)

// Report defines a function to report download progress
type Report func(bytesRead, totalBytes int64)

// DefaultReport returns a default report function
func DefaultReport(l *slog.Logger, msg string) Report {
	return func(bytesRead, totalBytes int64) {
		percentComplete := float64(bytesRead) / float64(totalBytes) * 100
		remaining := float64(totalBytes) - float64(bytesRead)
		l.Info(msg, "complete", fmt.Sprintf("%.1f%%", percentComplete), "remaining", utils.ByteFormat(remaining, 2))
	}
}

// FIXME: probably set to like 5 seconds, but it's easier to test this way
const defaultProgressInterval = 1 * time.Second

// progressReadCloser wraps an io.ReadCloser to track bytes read
type progressReadCloser struct {
	reader    io.ReadCloser
	bytesRead *atomic.Int64
}

// Read implements io.Reader interface
func (pr *progressReadCloser) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 {
		pr.bytesRead.Add(int64(n))
	}
	return n, err
}

// Close implements io.Closer interface
func (pr *progressReadCloser) Close() error {
	return pr.reader.Close()
}

// progressTracker holds the common logic for progress reporting
type progressTracker struct {
	reporter       Report
	reportInterval time.Duration
	bytesRead      *atomic.Int64
	totalBytes     int64

	stopReports chan struct{}
	wg          sync.WaitGroup
}

// startReporting starts the reporting goroutine
func (pt *progressTracker) StartReporting() {
	pt.wg.Add(1)
	go func() {
		defer pt.wg.Done()
		ticker := time.NewTicker(pt.reportInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				pt.reporter(pt.bytesRead.Load(), pt.totalBytes)
			case <-pt.stopReports:
				return
			}
		}
	}()
}

// StopReporting stops the reporting goroutine
func (pt *progressTracker) StopReporting() {
	if pt.stopReports != nil {
		close(pt.stopReports)
	}
	pt.wg.Wait()
}

// ProgressReadOnlyTarget reports progress during pulls
type ProgressReadOnlyTarget interface {
	oras.ReadOnlyTarget
	StartReporting()
	StopReporting()
}

// progressReadOnlyTarget wraps an oras.ReadOnlyTarget to track download progress
type progressReadOnlyTarget struct {
	oras.ReadOnlyTarget
	*progressTracker
}

// progressReadOnlyReferenceTarget wraps an oras.ReadOnlyTarget to track download progress
type progressReadOnlyReferenceTarget struct {
	*progressReadOnlyTarget
	registry.ReferenceFetcher
}

// NewProgressReadOnlyTarget creates a new ProgressTarget with the given reporter
func NewProgressReadOnlyTarget(target oras.ReadOnlyTarget, totalBytes int64, reporter Report) ProgressReadOnlyTarget {
	core := &progressTracker{
		reporter:       reporter,
		reportInterval: defaultProgressInterval,
		bytesRead:      &atomic.Int64{},
		totalBytes:     totalBytes,
		stopReports:    make(chan struct{}),
	}
	pt := &progressReadOnlyTarget{
		ReadOnlyTarget:  target,
		progressTracker: core,
	}
	if refFetcher, ok := target.(registry.ReferenceFetcher); ok {
		return &progressReadOnlyReferenceTarget{
			progressReadOnlyTarget: pt,
			ReferenceFetcher:       refFetcher,
		}
	}
	return pt
}

// Fetch preforms the underlying Fetch method and tracks downloaded bytes
func (pt *progressReadOnlyTarget) Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	rc, err := pt.ReadOnlyTarget.Fetch(ctx, desc)
	if err != nil {
		return nil, err
	}

	prc := &progressReadCloser{
		reader:    rc,
		bytesRead: pt.bytesRead,
	}

	return prc, nil
}

// FetchReference preforms the underlying FetchReference method and tracks downloaded bytes
func (prt *progressReadOnlyReferenceTarget) FetchReference(ctx context.Context, reference string) (ocispec.Descriptor, io.ReadCloser, error) {
	targetDesc, rc, err := prt.ReferenceFetcher.FetchReference(ctx, reference)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	prc := &progressReadCloser{
		reader:    rc,
		bytesRead: prt.bytesRead,
	}
	return targetDesc, prc, nil
}

// progressReader wraps an io.Reader to track bytes read
type progressReader struct {
	reader    io.Reader
	bytesRead *atomic.Int64
}

// Read implements io.Reader interface
func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 {
		pr.bytesRead.Add(int64(n))
	}
	return n, err
}

// ProgressPushTarget reports progress during pushes
type ProgressPushTarget interface {
	oras.Target
	StartReporting()
	StopReporting()
}

// progressPushTarget wraps an oras.Target to track progress
type progressPushTarget struct {
	oras.Target
	*progressTracker
}

// NewProgressPushTarget creates a new ProgressPushTarget
func NewProgressPushTarget(target oras.Target, totalBytes int64, reporter Report) ProgressPushTarget {
	core := &progressTracker{
		reporter:       reporter,
		reportInterval: defaultProgressInterval,
		bytesRead:      &atomic.Int64{},
		totalBytes:     totalBytes,
		stopReports:    make(chan struct{}),
	}
	pt := &progressPushTarget{
		Target:          target,
		progressTracker: core,
	}
	return pt
}

func (pt *progressPushTarget) Push(ctx context.Context, desc ocispec.Descriptor, content io.Reader) error {
	pr := &progressReader{
		reader:    content,
		bytesRead: pt.bytesRead,
	}

	return pt.Target.Push(ctx, desc, pr)
}
