// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
package images

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// newTestTracker creates a Tracker instance with a customizable report interval for testing.
func newTestTracker(bytesRead, totalBytes int64, reporter Report, interval time.Duration) *Tracker {
	atomicBytesRead := &atomic.Int64{}
	atomicBytesRead.Add(bytesRead)
	return &Tracker{
		reporter:       reporter,
		reportInterval: interval,
		bytesRead:      atomicBytesRead,
		totalBytes:     totalBytes,
		stopReports:    make(chan struct{}),
	}
}

func TestTracker_ReportingCycle(t *testing.T) {
	t.Parallel()

	var reportCount atomic.Int32
	var lastBytesRead, lastTotalBytes atomic.Int64

	reporterFunc := func(bytesRead, totalBytes int64) {
		reportCount.Add(1)
		lastBytesRead.Store(bytesRead)
		lastTotalBytes.Store(totalBytes)
	}

	testInterval := 10 * time.Millisecond
	initialBytesRead := int64(10)
	totalBytes := int64(1000)
	tracker := newTestTracker(initialBytesRead, totalBytes, reporterFunc, testInterval)

	// Start reporting and wait for 4 cycles
	tracker.StartReporting(t.Context())
	time.Sleep(testInterval*4 + testInterval/2)

	// Stop reporting
	tracker.StopReporting()
	time.Sleep(testInterval * 2)

	// Verify four cycles occurred
	require.Equal(t, int32(4), reportCount.Load())
	require.Equal(t, initialBytesRead, lastBytesRead.Load())
	require.Equal(t, totalBytes, lastTotalBytes.Load())
}
