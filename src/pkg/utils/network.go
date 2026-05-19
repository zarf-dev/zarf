// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic helper functions.
package utils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/clock"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

// retryAfterDuration is returned on a 429 so the retry loop can honor
// the server-requested delay before the normal exponential sleep.
type retryAfterDuration time.Duration

func (d retryAfterDuration) Error() string {
	return fmt.Sprintf("rate limited (HTTP 429), retry after %s", time.Duration(d))
}

// unrecoverableError wraps errors that must not be retried.
type unrecoverableError struct{ err error }

func (e unrecoverableError) Error() string {
	return e.err.Error()
}

func (e unrecoverableError) Unwrap() error {
	return e.err
}

func parseChecksum(src string) (string, string, error) {
	atSymbolCount := strings.Count(src, "@")
	var checksum string
	if atSymbolCount > 0 {
		parsed, err := url.Parse(src)
		if err != nil {
			return src, checksum, fmt.Errorf("unable to parse the URL: %s", src)
		}
		if atSymbolCount == 1 && parsed.User != nil {
			return src, checksum, nil
		}

		index := strings.LastIndex(src, "@")
		checksum = src[index+1:]
		src = src[:index]
	}
	return src, checksum, nil
}

// DownloadToFile downloads a given URL to the target filepath (including the cosign key if necessary).
func DownloadToFile(ctx context.Context, src, dst string) (err error) {
	// check if the parsed URL has a checksum
	// if so, remove it and use the checksum to validate the file
	src, checksum, err := parseChecksum(src)
	if err != nil {
		return err
	}

	err = helpers.CreateDirectory(filepath.Dir(dst), helpers.ReadWriteExecuteUser)
	if err != nil {
		return fmt.Errorf(lang.ErrCreatingDir, filepath.Dir(dst), err)
	}

	l := logger.From(ctx)

	// resetInterval is larger than the total retry window so the backoff never auto-resets
	expDelay := wait.Backoff{
		Duration: config.ZarfDefaultRetryDelay,
		Factor:   2.0,
		Steps:    config.ZarfDefaultRetries,
		Cap:      config.ZarfDefaultRetryMaxDelay,
	}.DelayWithReset(clock.RealClock{}, time.Hour)

	// when a 429 Retry-After is seen the condition sets retryAfterOverride so
	// the next sleep uses that duration instead of the exponential one
	var retryAfterOverride time.Duration
	retryDelay := wait.DelayFunc(func() time.Duration {
		if retryAfterOverride > 0 {
			d := retryAfterOverride
			retryAfterOverride = 0
			return d
		}
		return expDelay()
	})

	attempt := 0
	err = retryDelay.Until(ctx, true, false, func(ctx context.Context) (bool, error) {
		file, createErr := os.Create(dst)
		if createErr != nil {
			return false, fmt.Errorf(lang.ErrWritingFile, dst, createErr)
		}
		getErr := httpGetFile(ctx, src, file)
		closeErr := file.Close()
		joinedErr := errors.Join(getErr, closeErr)
		if joinedErr == nil {
			return true, nil
		}
		if unrecovErr, ok := errors.AsType[unrecoverableError](joinedErr); ok {
			return false, unrecovErr.err
		}
		if rlErr, ok := errors.AsType[retryAfterDuration](joinedErr); ok {
			retryAfterOverride = time.Duration(rlErr)
		}
		attempt++
		l.Warn("retrying download",
			"attempt", attempt,
			"maxAttempts", config.ZarfDefaultRetries,
			"url", src,
			"error", joinedErr,
		)
		return false, nil
	})
	if err != nil {
		return err
	}

	// If the file has a checksum, validate it
	if 0 < len(checksum) {
		received, err := helpers.GetSHA256OfFile(dst)
		if err != nil {
			return err
		}
		if received != checksum {
			return fmt.Errorf("shasum mismatch for file %s: expected %s, got %s ", dst, checksum, received)
		}
	}

	return nil
}

func httpGetFile(ctx context.Context, url string, destinationFile *os.File) (err error) {
	l := logger.From(ctx)
	l.Info("download start", "url", url)
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return &unrecoverableError{fmt.Errorf("unable to create request for %s: %w", url, err)}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("unable to download the file %s: %w", url, err)
	}
	defer func() {
		err2 := resp.Body.Close()
		err = errors.Join(err, err2)
	}()

	// Check server response
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusTooManyRequests {
			if d := parseRetryAfter(resp.Header.Get("Retry-After")); d > 0 {
				const maxRetryAfter = 60 * time.Second
				if d > maxRetryAfter {
					return &unrecoverableError{fmt.Errorf("rate limited (HTTP 429) with Retry-After %s exceeding %s: %s", d, maxRetryAfter, resp.Status)}
				}
				return retryAfterDuration(d)
			}
			return fmt.Errorf("rate limited (HTTP 429): %s", resp.Status)
		}
		if resp.StatusCode >= 500 {
			return fmt.Errorf("server error: %s", resp.Status)
		}
		return &unrecoverableError{fmt.Errorf("bad HTTP status: %s", resp.Status)}
	}

	// Copy response body to file
	if _, err = io.Copy(destinationFile, resp.Body); err != nil {
		return fmt.Errorf("unable to save the file %s: %w", destinationFile.Name(), err)
	}
	l.Debug("download successful", "url", url, "size", resp.ContentLength, "duration", time.Since(start))
	return nil
}

// parseRetryAfter parses the Retry-After header value into a duration.
// It supports both delay-seconds (integer) and HTTP-date formats.
func parseRetryAfter(value string) time.Duration {
	if value == "" {
		return 0
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.Duration(seconds) * time.Second
	}
	if t, err := http.ParseTime(value); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}
