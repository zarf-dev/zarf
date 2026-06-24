// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci contains functions for interacting with Zarf packages stored in OCI registries.
package zoci

import (
	"context"
	"fmt"
	"time"

	"github.com/avast/retry-go/v4"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/pkg/images"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"oras.land/oras-go/v2"
)

// OCITimestampFormat is the format used for the OCI timestamp annotation
const OCITimestampFormat = time.RFC3339

// PushPackage publishes the zarf package to the remote repository.
func (r *Remote) PushPackage(ctx context.Context, pkgLayout *layout.PackageLayout, opts PublishOptions) (_ ocispec.Descriptor, err error) {
	l := logger.From(ctx)

	start := time.Now()
	if opts.OCIConcurrency == 0 {
		opts.OCIConcurrency = DefaultConcurrency
	}

	// disallow infinite or negative
	if opts.Retries <= 0 {
		if opts.Retries < 0 {
			return ocispec.Descriptor{}, fmt.Errorf("retries cannot be negative")
		}
		l.Debug("retries set to default", "retries", DefaultRetries)
		opts.Retries = DefaultRetries
	}

	if pkgLayout.Digest() == "" {
		return ocispec.Descriptor{}, fmt.Errorf("package layout has no digest; manifest must be computed before publishing")
	}

	copyOpts := r.OrasRemote.GetDefaultCopyOpts()
	copyOpts.Concurrency = opts.OCIConcurrency

	totalSize := pkgLayout.TotalSize()

	var publishedDesc ocispec.Descriptor
	err = retry.Do(
		func() error {
			l.Info("pushing package to registry", "destination", r.Repo().Reference.String(),
				"architecture", pkgLayout.Pkg.Build.Architecture, "size", utils.ByteFormat(float64(totalSize), 2))

			trackedRemote := images.NewTrackedTarget(
				r.Repo(),
				totalSize,
				images.DefaultReport(r.Log(), "package publish in progress", r.Repo().Reference.String()),
			)
			trackedRemote.StartReporting(ctx)
			defer trackedRemote.StopReporting()

			var copyErr error
			publishedDesc, copyErr = oras.Copy(ctx, pkgLayout, pkgLayout.Digest(), trackedRemote, "", copyOpts)
			if copyErr != nil {
				return copyErr
			}

			return r.OrasRemote.UpdateIndex(ctx, r.Repo().Reference.Reference, publishedDesc)
		},
		retry.Attempts(uint(opts.Retries)),
		retry.Delay(defaultDelayTime),
		retry.MaxDelay(defaultMaxDelayTime),
		retry.DelayType(retry.BackOffDelay), // exponential backoff
		retry.LastErrorOnly(true),
		retry.Context(ctx),
		retry.OnRetry(func(n uint, err error) {
			// Only log retry if retries are enabled and this is not the last attempt
			if opts.Retries > 1 && n+1 < uint(opts.Retries) {
				l.Warn("retrying package push",
					"attempt", n+1,
					"maxAttempts", opts.Retries,
					"error", err,
				)
			}
		}),
	)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("publish failed: %w", err)
	}

	l.Info("completed package publish", "destination", r.Repo().Reference.String(),
		"duration", time.Since(start).Round(100*time.Millisecond))

	return publishedDesc, nil
}
