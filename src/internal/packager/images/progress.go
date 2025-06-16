// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides image-related utilities for Zarf
package images

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry"
)

// ProgressReporter defines a function to report download progress
type ProgressReporter func(bytesRead, totalBytes int64)

// DefaultProgressReporter returns a default progress reporter
func DefaultProgressReporter() ProgressReporter {
	return func(bytesRead, totalBytes int64) {
		percentComplete := float64(bytesRead) / float64(totalBytes) * 100
		remaining := float64(totalBytes) - float64(bytesRead)
		logger.Default().Info("image pull in progress", "complete", fmt.Sprintf("%.1f%%", percentComplete), "remaining", utils.ByteFormat(remaining, 2))
	}
}

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

// ProgressReadOnlyTarget reports progress during pulls
type ProgressReadOnlyTarget interface {
	oras.ReadOnlyTarget
	StopReporting()
}

// progressReadOnlyTarget wraps an oras.ReadOnlyTarget to track download progress
type progressReadOnlyTarget struct {
	oras.ReadOnlyTarget
	reporter       ProgressReporter
	reportInterval time.Duration
	bytesRead      *atomic.Int64
	totalBytes     int64

	// Track whether the reporting goroutine is running
	reportingStarted bool
	mu               sync.Mutex
	stopReports      chan struct{}
	wg               sync.WaitGroup
}

type progressReferenceTarget struct {
	*progressReadOnlyTarget
	registry.ReferenceFetcher
}

// NewProgressTarget creates a new ProgressTarget with the given reporter
func NewProgressTarget(target oras.ReadOnlyTarget, totalBytes int64, reporter ProgressReporter) ProgressReadOnlyTarget {
	return NewProgressTargetWithPeriod(target, totalBytes, reporter, defaultProgressInterval)
}

// NewProgressTargetWithPeriod creates a new ProgressTarget with a custom reporting period
func NewProgressTargetWithPeriod(target oras.ReadOnlyTarget, totalBytes int64, reporter ProgressReporter, reportInterval time.Duration) ProgressReadOnlyTarget {
	pt := &progressReadOnlyTarget{
		ReadOnlyTarget:   target,
		reporter:         reporter,
		reportInterval:   reportInterval,
		bytesRead:        &atomic.Int64{},
		totalBytes:       totalBytes,
		stopReports:      make(chan struct{}),
		reportingStarted: false,
	}
	if refFetcher, ok := target.(registry.ReferenceFetcher); ok {
		return &progressReferenceTarget{
			progressReadOnlyTarget: pt,
			ReferenceFetcher:       refFetcher,
		}
	}
	return pt
}

// startReporting starts the reporting goroutine if it hasn't been started already
func (pt *progressReadOnlyTarget) startReporting(ctx context.Context) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.reportingStarted {
		return
	}

	pt.reportingStarted = true
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
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Fetch preforms the underlying Fetch method and tracks downloaded bytes
func (pt *progressReadOnlyTarget) Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	// Start the reporting goroutine if it hasn't been started yet
	pt.startReporting(ctx)

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

// StopReporting stops the reporting goroutine
func (pt *progressReadOnlyTarget) StopReporting() {
	if pt.reportingStarted {
		close(pt.stopReports)
		pt.reportingStarted = false
	}
	pt.wg.Wait()
}

// FetchReference preforms the underlying FetchReference method and tracks downloaded bytes
func (pft *progressReferenceTarget) FetchReference(ctx context.Context, reference string) (ocispec.Descriptor, io.ReadCloser, error) {
	target, rc, err := pft.ReferenceFetcher.FetchReference(ctx, reference)
	if err != nil {
		return ocispec.Descriptor{}, nil, err
	}
	prc := &progressReadCloser{
		reader:    rc,
		bytesRead: pft.bytesRead,
	}
	return target, prc, nil
}

// progressReader wraps an io.ReadCloser to track bytes read
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

// ProgressPushTarget reports progress during pulls
type ProgressPushTarget interface {
	oras.Target
	StopReporting()
}

// progressTarget wraps an oras.ReadOnlyTarget to track download progress
type progressPushTarget struct {
	oras.Target
	reporter       ProgressReporter
	reportInterval time.Duration
	bytesRead      *atomic.Int64
	totalBytes     int64

	// Track whether the reporting goroutine is running
	reportingStarted bool
	mu               sync.Mutex
	stopReports      chan struct{}
	wg               sync.WaitGroup
}

// NewProgressPushTarget creates a new ProgressPushTarget
func NewProgressPushTarget(target oras.Target, totalBytes int64, reporter ProgressReporter) ProgressPushTarget {
	pt := &progressPushTarget{
		Target:           target,
		reporter:         reporter,
		reportInterval:   defaultProgressInterval,
		bytesRead:        &atomic.Int64{},
		totalBytes:       totalBytes,
		stopReports:      make(chan struct{}),
		reportingStarted: false,
	}
	return pt
}

func (pt *progressPushTarget) Push(ctx context.Context, desc ocispec.Descriptor, content io.Reader) error {
	// Start the reporting goroutine if it hasn't been started yet
	pt.startReporting(ctx)

	prc := &progressReader{
		reader:    content,
		bytesRead: pt.bytesRead,
	}

	return pt.Target.Push(ctx, desc, prc)
}

// startReporting starts the reporting goroutine if it hasn't been started already
func (pt *progressPushTarget) startReporting(ctx context.Context) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.reportingStarted {
		return
	}

	pt.reportingStarted = true
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
			case <-ctx.Done():
				return
			}
		}
	}()
}

// StopReporting stops the reporting goroutine
func (pt *progressPushTarget) StopReporting() {
	if pt.reportingStarted {
		close(pt.stopReports)
		pt.reportingStarted = false
	}
	pt.wg.Wait()
}
