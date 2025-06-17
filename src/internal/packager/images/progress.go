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
)

// Report defines a function to log progress
type Report func(bytesRead, totalBytes int64)

// DefaultReport returns a default report function
func DefaultReport(l *slog.Logger, msg string) Report {
	return func(bytesRead, totalBytes int64) {
		percentComplete := float64(bytesRead) / float64(totalBytes) * 100
		remaining := float64(totalBytes) - float64(bytesRead)
		l.Info(msg, "complete", fmt.Sprintf("%.1f%%", percentComplete), "remaining", utils.ByteFormat(remaining, 2))
	}
}

const defaultProgressInterval = 5 * time.Second

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

// progressReader wraps an io.Reader to track bytes read incrementally.
// It is used when the underlying reader does not implement io.WriterTo.
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

// progressWriterToReader wraps an io.Reader that also implements io.WriterTo.
// It tracks progress for both Read and WriteTo operations.
type progressWriterToReader struct {
	*progressReader
	writerTo io.WriterTo
}

// WriteTo implements io.WriterTo, tracks progress in a single update after the operation.
func (pwr *progressWriterToReader) WriteTo(w io.Writer) (int64, error) {
	written, err := pwr.writerTo.WriteTo(w)
	if written > 0 {
		pwr.bytesRead.Add(written)
	}
	return written, err
}

// ProgressTarget wraps an oras.Target to track progress
type ProgressTarget struct {
	oras.Target
	*progressTracker
}

// NewProgressTarget creates a new ProgressPushTarget
func NewProgressTarget(target oras.Target, totalBytes int64, reporter Report) *ProgressTarget {
	core := &progressTracker{
		reporter:       reporter,
		reportInterval: defaultProgressInterval,
		bytesRead:      &atomic.Int64{},
		totalBytes:     totalBytes,
		stopReports:    make(chan struct{}),
	}
	pt := &ProgressTarget{
		Target:          target,
		progressTracker: core,
	}
	return pt
}

// Push wraps the target push method with an appropriate progress reader.
// It checks if the content reader implements io.WriterTo to select the optimal wrapper.
func (pt *ProgressTarget) Push(ctx context.Context, desc ocispec.Descriptor, content io.Reader) error {
	pReader := &progressReader{
		reader:    content,
		bytesRead: pt.bytesRead,
	}
	var newReader io.Reader
	newReader = pReader
	// If content supports WriteTo, wrap it with progressWriterToReader
	if wt, ok := content.(io.WriterTo); ok {
		newReader = &progressWriterToReader{
			writerTo:       wt,
			progressReader: pReader,
		}
	}

	return pt.Target.Push(ctx, desc, newReader)
}
