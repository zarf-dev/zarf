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
func DefaultReport(l *slog.Logger, msg string, imageName string) Report {
	return func(bytesRead, totalBytes int64) {
		if totalBytes <= 0 {
			l.Warn("total bytes is a non-positive integer, can't report progress")
			return
		}
		percentComplete := float64(bytesRead) / float64(totalBytes) * 100
		remaining := float64(totalBytes) - float64(bytesRead)
		l.Info(msg, "name", imageName, "complete", fmt.Sprintf("%.1f%%", percentComplete), "remaining", utils.ByteFormat(remaining, 2))
	}
}

// trackedReader wraps an io.Reader to track bytes read incrementally
type trackedReader struct {
	reader    io.Reader
	bytesRead *atomic.Int64
}

// Read wraps the io.Reader interface to collect bytes read
func (pr *trackedReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 {
		pr.bytesRead.Add(int64(n))
	}
	return n, err
}

// trackedWriterToReader wraps an io.Reader that implements io.WriterTo.
type trackedWriterToReader struct {
	*trackedReader
	writerTo io.WriterTo
}

const defaultProgressInterval = 5 * time.Second

// WriteTo wraps the io.WriteTo interface to collect bytes read
func (pwr *trackedWriterToReader) WriteTo(w io.Writer) (int64, error) {
	written, err := pwr.writerTo.WriteTo(w)
	if written > 0 {
		pwr.bytesRead.Add(written)
	}
	return written, err
}

// StartReporting starts the reporting goroutine
func (tt *Tracker) StartReporting(ctx context.Context) {
	tt.wg.Add(1)
	go func() {
		defer tt.wg.Done()
		ticker := time.NewTicker(tt.reportInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				tt.reporter(tt.bytesRead.Load(), tt.totalBytes)
			case <-tt.stopReports:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// StopReporting stops the reporting goroutine
func (tt *Tracker) StopReporting() {
	if tt.stopReports != nil {
		close(tt.stopReports)
	}
	tt.wg.Wait()
}

// Tracker reports progress against totalBytes as bytesRead gets updated
type Tracker struct {
	reporter       Report
	reportInterval time.Duration
	bytesRead      *atomic.Int64
	totalBytes     int64

	stopReports chan struct{}
	wg          sync.WaitGroup
}

// TrackedTarget wraps an oras.Target to track progress
type TrackedTarget struct {
	oras.Target
	*Tracker
}

// NewTrackedTarget creates a new TrackedTarget
func NewTrackedTarget(target oras.Target, totalBytes int64, reporter Report) *TrackedTarget {
	tracker := &Tracker{
		reporter:       reporter,
		reportInterval: defaultProgressInterval,
		bytesRead:      &atomic.Int64{},
		totalBytes:     totalBytes,
		stopReports:    make(chan struct{}),
	}
	return &TrackedTarget{
		Target:  target,
		Tracker: tracker,
	}
}

// Push wraps the target push method with an appropriate tracked reader.
func (tt *TrackedTarget) Push(ctx context.Context, desc ocispec.Descriptor, content io.Reader) error {
	tReader := &trackedReader{
		reader:    content,
		bytesRead: tt.bytesRead,
	}
	var trackedReader io.Reader
	trackedReader = tReader
	// If content supports WriteTo, wrap it with progressWriterToReader
	if wt, ok := content.(io.WriterTo); ok {
		trackedReader = &trackedWriterToReader{
			writerTo:      wt,
			trackedReader: tReader,
		}
	}

	return tt.Target.Push(ctx, desc, trackedReader)
}
