// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package images provides image-related utilities for Zarf
package images

import (
	"context"
	"io"
	"math"
	"sync"
	"sync/atomic"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"oras.land/oras-go/v2"
)

// ProgressReporter defines a function to report download progress
type ProgressReporter func(bytesRead, totalBytes int64)

// DefaultProgressReporter returns a default progress reporter that uses the message package
func DefaultProgressReporter() ProgressReporter {
	return func(bytesRead, totalBytes int64) {
		percentComplete := float64(bytesRead) / float64(totalBytes) * 100
		formattedPercent := math.Floor(percentComplete*10) / 10
		logger.Default().Info("Downloading image", "percent complete", formattedPercent)
	}
}

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

// NewProgressTarget creates a new ProgressTarget with the given reporter
func NewProgressTarget(target oras.ReadOnlyTarget, totalBytes int64, reporter ProgressReporter) *ProgressTarget {
	return NewProgressTargetWithPeriod(target, totalBytes, reporter, 1*time.Second)
}

// NewProgressTargetWithPeriod creates a new ProgressTarget with a custom reporting period
func NewProgressTargetWithPeriod(target oras.ReadOnlyTarget, totalBytes int64, reporter ProgressReporter, reportInterval time.Duration) *ProgressTarget {
	return &ProgressTarget{
		ReadOnlyTarget:   target,
		reporter:         reporter,
		reportInterval:   reportInterval,
		bytesRead:        &atomic.Int64{},
		totalBytes:       totalBytes,
		stopReports:      make(chan struct{}),
		reportingStarted: false,
	}
}

// startReporting starts the reporting goroutine if it hasn't been started already
func (pt *ProgressTarget) startReporting(ctx context.Context) {
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

		// Wait for the first tick before reporting anything
		select {
		case <-ticker.C:
			// First tick elapsed
		case <-pt.stopReports:
			return
		case <-ctx.Done():
			return
		}

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

// Fetch overrides the Fetch method to track downloaded bytes
func (pt *ProgressTarget) Fetch(ctx context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
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
		pt.reportingStarted = false
	}
	pt.mu.Unlock()
	pt.wg.Wait()
}
