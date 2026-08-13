// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package zoci

import (
	"context"
	"fmt"

	"github.com/avast/retry-go/v4"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

// Retry executes fn using Zarf's OCI retry policy.
func Retry(ctx context.Context, retries int, fn func() error) error {
	l := logger.From(ctx)
	if retries <= 0 {
		if retries < 0 {
			return fmt.Errorf("retries cannot be negative")
		}
		l.Debug("retries set to default", "retries", DefaultRetries)
		retries = DefaultRetries
	}

	return retry.Do(
		fn,
		retry.Attempts(uint(retries)),
		retry.Delay(defaultDelayTime),
		retry.MaxDelay(defaultMaxDelayTime),
		retry.DelayType(retry.BackOffDelay),
		retry.LastErrorOnly(true),
		retry.Context(ctx),
		retry.OnRetry(func(n uint, err error) {
			if retries > 1 && n+1 < uint(retries) {
				l.Warn("operation failed, retrying", "attempt", n+1, "maxAttempts", retries, "error", err)
			}
		}),
	)
}
