// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci contains functions for interacting with Zarf packages stored in OCI registries.
package zoci

import (
	"context"
	"fmt"
	"time"

	"github.com/zarf-dev/zarf/src/internal/packager/images"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"oras.land/oras-go/v2"

	retry "github.com/avast/retry-go/v4"
)

// CopyPackage copies a zarf package from one OCI registry to another using ORAS with retry.
func CopyPackage(ctx context.Context, src *Remote, dst *Remote, retries int, concurrency int) (err error) {
	l := logger.From(ctx)
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}

	// Resolve the root digest of the source package (manifest or index)
	srcRoot, err := src.ResolveRoot(ctx)
	if err != nil {
		return err
	}
	srcRef := srcRoot.Digest.String()

	copyOpts := dst.OrasRemote.GetDefaultCopyOpts()
	copyOpts.Concurrency = concurrency

	tag := src.Repo().Reference.Reference // keep the source tag on the destination

	err = retry.Do(
		func() error {
			l.Info("copying package",
				"src", src.Repo().Reference.String(),
				"dst", dst.Repo().Reference.String(),
				"ref", srcRef,
			)
			source := src.Repo() // implements oras.ReadOnlyTarget
			trackedDst := images.NewTrackedTarget(
				dst.Repo(),
				0, // unknown total for registry→registry copy
				images.DefaultReport(dst.Log(), "package copy in progress", dst.Repo().Reference.String()),
			)
			trackedDst.StartReporting(ctx)
			defer trackedDst.StopReporting()

			// 1) Copy by digest from source → destination
			publishedDesc, copyErr := oras.Copy(ctx, source, srcRef, trackedDst, "", copyOpts)
			if copyErr != nil {
				return copyErr
			}

			// 2) Update/tag the destination index to the source tag
			return dst.OrasRemote.UpdateIndex(ctx, tag, publishedDesc)
		},
		retry.Attempts(uint(retries)),
		retry.Delay(500*time.Millisecond),
		retry.MaxDelay(8*time.Second),
		retry.DelayType(retry.BackOffDelay),
		retry.LastErrorOnly(true),
		retry.Context(ctx),
		retry.OnRetry(func(n uint, err error) {
			l.Warn("retrying package copy",
				"attempt", n+1,
				"max_attempts", retries,
				"error", err,
			)
		}),
	)
	if err != nil {
		return fmt.Errorf("copy failed after retries: %w", err)
	}

	l.Info("package copied successfully",
		"source", src.Repo().Reference.String(),
		"destination", dst.Repo().Reference.String(),
		"tag", tag,
	)
	return nil
}
