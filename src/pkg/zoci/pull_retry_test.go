// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package zoci

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/config"
)

func TestPullRetryPolicyHonorsMaxAttemptsAtOverflowBoundary(t *testing.T) {
	t.Parallel()

	policy := newPullRetryPolicy("example.com/test:latest", 37, nil)
	response := &http.Response{StatusCode: http.StatusInternalServerError}

	delay, err := policy.Retry(35, response, nil)
	require.NoError(t, err)
	require.Equal(t, config.ZarfDefaultRetryMaxDelay, delay)

	delay, err = policy.Retry(36, response, nil)
	require.NoError(t, err)
	require.Equal(t, time.Duration(-1), delay)
}
