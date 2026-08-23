// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package images

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStopReporting(t *testing.T) {
	t.Parallel()

	tracker := &Tracker{
		reporter:    func(_, _ int64) {},
		bytesRead:   &atomic.Int64{},
		stopReports: make(chan struct{}),
	}

	tracker.StopReporting()
	require.NotPanics(t, func() {
		tracker.StopReporting()
	})
}
