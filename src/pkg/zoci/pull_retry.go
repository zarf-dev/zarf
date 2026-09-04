// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package zoci

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"oras.land/oras-go/v2/registry/remote/retry"
)

type pullRetryPolicy struct {
	reference string
	retries   int
	log       *slog.Logger
}

func newPullRetryPolicy(reference string, retries int, log *slog.Logger) *pullRetryPolicy {
	return &pullRetryPolicy{reference: reference, retries: retries, log: log}
}

func (p *pullRetryPolicy) Retry(attempt int, resp *http.Response, requestErr error) (time.Duration, error) {
	if attempt >= p.retries-1 || !isRetryablePullError(resp, requestErr) {
		return -1, nil
	}

	delay, delayErr := pullRetryDelay(attempt, resp)
	if delayErr != nil {
		return 0, delayErr
	}
	if p.log != nil {
		p.log.Warn("OCI pull request failed, retrying",
			"attempt", attempt+1,
			"maxAttempts", p.retries,
			"reference", p.reference,
			"delay", delay,
			"status", responseStatus(resp),
			"error", requestErr,
		)
	}
	return delay, nil
}

func isRetryablePullError(resp *http.Response, err error) bool {
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false
		}
		var networkErr net.Error
		return errors.As(err, &networkErr) && networkErr.Timeout()
	}
	if resp == nil {
		return false
	}
	return resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError
}

func pullRetryDelay(attempt int, resp *http.Response) (time.Duration, error) {
	if resp != nil && resp.StatusCode == http.StatusTooManyRequests {
		if delay := utils.ParseRetryAfter(resp.Header.Get("Retry-After")); delay > 0 {
			if delay > utils.MaxRetryAfter {
				return 0, fmt.Errorf("rate limited (HTTP 429) with Retry-After %s exceeding %s: %s", delay, utils.MaxRetryAfter, resp.Status)
			}
			return delay, nil
		}
	}

	delay := config.ZarfDefaultRetryDelay
	for range attempt {
		if delay >= config.ZarfDefaultRetryMaxDelay || delay > config.ZarfDefaultRetryMaxDelay-delay {
			return config.ZarfDefaultRetryMaxDelay, nil
		}
		delay *= 2
	}
	if delay > config.ZarfDefaultRetryMaxDelay {
		delay = config.ZarfDefaultRetryMaxDelay
	}
	return delay, nil
}

func responseStatus(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}

var _ retry.Policy = (*pullRetryPolicy)(nil)
