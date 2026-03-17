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

	retry "github.com/avast/retry-go/v4"
	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

// retryAfterDuration is returned on a 429 so the custom DelayType can use it
// instead of stacking on top of the normal backoff.
type retryAfterDuration time.Duration

func (d retryAfterDuration) Error() string {
	return fmt.Sprintf("rate limited (HTTP 429), retry after %s", time.Duration(d))
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
	err = retry.Do(
		func() error {
			// Create the file
			file, createErr := os.Create(dst)
			if createErr != nil {
				return retry.Unrecoverable(fmt.Errorf(lang.ErrWritingFile, dst, createErr))
			}
			getErr := httpGetFile(ctx, src, file)
			closeErr := file.Close()
			return errors.Join(getErr, closeErr)
		},
		retry.Attempts(uint(config.ZarfDefaultRetries)),
		retry.Delay(config.ZarfDefaultRetryDelay),
		retry.MaxDelay(config.ZarfDefaultRetryMaxDelay),
		retry.DelayType(func(n uint, err error, rc *retry.Config) time.Duration {
			var rlErr retryAfterDuration
			if errors.As(err, &rlErr) {
				return time.Duration(rlErr)
			}
			return retry.BackOffDelay(n, err, rc)
		}),
		retry.LastErrorOnly(true),
		retry.Context(ctx),
		retry.OnRetry(func(n uint, err error) {
			if config.ZarfDefaultRetries > 1 && n+1 < uint(config.ZarfDefaultRetries) {
				l.Warn("retrying download",
					"attempt", n+1,
					"max_attempts", config.ZarfDefaultRetries,
					"url", src,
					"error", err,
				)
			}
		}),
	)
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
		return retry.Unrecoverable(fmt.Errorf("unable to create request for %s: %w", url, err))
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
					return retry.Unrecoverable(fmt.Errorf("rate limited (HTTP 429) with Retry-After %s exceeding %s: %s", d, maxRetryAfter, resp.Status))
				}
				return retryAfterDuration(d)
			}
			return fmt.Errorf("rate limited (HTTP 429): %s", resp.Status)
		}
		if resp.StatusCode >= 500 {
			return fmt.Errorf("server error: %s", resp.Status)
		}
		return retry.Unrecoverable(fmt.Errorf("bad HTTP status: %s", resp.Status))
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
