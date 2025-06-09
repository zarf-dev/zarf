// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides image-related utilities for Zarf
package images

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"

	"github.com/zarf-dev/zarf/src/pkg/logger"
)

// ProgressReporter defines a function to report download progress
type ProgressReporter func(bytesRead, totalBytes int64)

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

// ProgressTarget wraps an oras.ReadOnlyTarget to track download progress
type ProgressTarget struct {
	oras.ReadOnlyTarget
	reporter     ProgressReporter
	reportPeriod time.Duration
	bytesRead    *atomic.Int64
	totalBytes   int64

	// Track whether the reporting goroutine is running
	reportingStarted bool
	mu               sync.Mutex
	stopReports      chan struct{}
	wg               sync.WaitGroup
	ctx              context.Context
	cancel           context.CancelFunc
}

// NewProgressTarget creates a new ProgressTarget with the given reporter
func NewProgressTarget(target oras.ReadOnlyTarget, totalBytes int64, reporter ProgressReporter) *ProgressTarget {
	return NewProgressTargetWithPeriod(target, totalBytes, reporter, 1*time.Second)
}

// NewProgressTargetWithPeriod creates a new ProgressTarget with a custom reporting period
func NewProgressTargetWithPeriod(target oras.ReadOnlyTarget, totalBytes int64, reporter ProgressReporter, reportPeriod time.Duration) *ProgressTarget {
	ctx, cancel := context.WithCancel(context.Background())
	return &ProgressTarget{
		ReadOnlyTarget:   target,
		reporter:         reporter,
		reportPeriod:     reportPeriod,
		bytesRead:        &atomic.Int64{},
		totalBytes:       totalBytes,
		stopReports:      make(chan struct{}),
		reportingStarted: false,
		ctx:              ctx,
		cancel:           cancel,
	}
}

// startReporting starts the reporting goroutine if it hasn't been started already
func (pt *ProgressTarget) startReporting() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.reportingStarted {
		return
	}

	pt.reportingStarted = true
	pt.wg.Add(1)

	go func() {
		defer pt.wg.Done()
		ticker := time.NewTicker(pt.reportPeriod)
		defer ticker.Stop()

		lastReported := int64(0)

		// Wait for the first tick before reporting anything
		select {
		case <-ticker.C:
			// First tick elapsed
		case <-pt.stopReports:
			return
		case <-pt.ctx.Done():
			return
		}

		for {
			select {
			case <-ticker.C:
				current := pt.bytesRead.Load()
				// Only report if there's been progress since the last report
				if current > lastReported {
					pt.reporter(current, pt.totalBytes)
					lastReported = current
				}
			case <-pt.stopReports:
				// Report final progress before exiting
				current := pt.bytesRead.Load()
				if current > lastReported {
					pt.reporter(current, pt.totalBytes)
				}
				return
			case <-pt.ctx.Done():
				return
			}
		}
	}()
}

// Fetch overrides the Fetch method to track downloaded bytes
func (pt *ProgressTarget) Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	// Start the reporting goroutine if it hasn't been started yet
	pt.startReporting()

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

// Resolve overrides the Resolve method from the ReadOnlyTarget interface
func (pt *ProgressTarget) Resolve(ctx context.Context, reference string) (ocispec.Descriptor, error) {
	return pt.ReadOnlyTarget.Resolve(ctx, reference)
}

// Exists overrides the Exists method from the ReadOnlyTarget interface
func (pt *ProgressTarget) Exists(ctx context.Context, desc ocispec.Descriptor) (bool, error) {
	return pt.ReadOnlyTarget.Exists(ctx, desc)
}

// StopReporting stops the reporting goroutine
func (pt *ProgressTarget) StopReporting() {
	pt.mu.Lock()
	if pt.reportingStarted {
		close(pt.stopReports)
		pt.cancel()
		pt.reportingStarted = false
	}
	pt.mu.Unlock()
	pt.wg.Wait()
}

// DefaultProgressReporter returns a default progress reporter that uses the message package
func DefaultProgressReporter() ProgressReporter {
	return func(bytesRead, totalBytes int64) {
		percentComplete := float64(bytesRead) / float64(totalBytes) * 100
		logger.Default().Info("Downloading image", "percent complete", percentComplete)
	}
}
