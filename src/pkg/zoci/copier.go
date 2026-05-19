// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci contains functions for interacting with Zarf packages stored in OCI registries.
package zoci

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/util/wait"
	"oras.land/oras-go/v2"

	"github.com/zarf-dev/zarf/src/pkg/logger"
)

// CopyPackage copies a zarf package from one OCI registry to another using ORAS with retry.
func CopyPackage(ctx context.Context, src *Remote, dst *Remote, opts PublishOptions) (err error) {
	l := logger.From(ctx)
	if opts.OCIConcurrency <= 0 {
		opts.OCIConcurrency = DefaultConcurrency
	}
	// disallow infinite or negative
	if opts.Retries <= 0 {
		if opts.Retries < 0 {
			return fmt.Errorf("retries cannot be negative")
		}
		l.Debug("retries set to default", "retries", DefaultRetries)
		opts.Retries = DefaultRetries
	}

	// Resolve the root digest of the source package (manifest or index)
	srcRoot, err := src.ResolveRoot(ctx)
	if err != nil {
		return err
	}
	srcRef := srcRoot.Digest.String()

	copyOpts := dst.OrasRemote.GetDefaultCopyOpts()
	copyOpts.Concurrency = opts.OCIConcurrency

	tag := src.Repo().Reference.Reference // keep the source tag on the destination
	if opts.Tag != "" {
		tag = opts.Tag
	}

	var (
		lastErr  error
		attempts int
	)
	err = wait.ExponentialBackoffWithContext(ctx, wait.Backoff{
		Duration: defaultDelayTime,
		Factor:   2.0,
		Steps:    opts.Retries,
		Cap:      defaultMaxDelayTime,
	}, func(ctx context.Context) (bool, error) {
		l.Info("copying package",
			"src", src.Repo().Reference.String(),
			"dst", dst.Repo().Reference.String(),
			"ref", srcRef,
		)
		defer func() {
			if lastErr == nil {
				return
			}
			l.Warn("retrying package copy",
				"attempt", attempts,
				"maxAttempts", opts.Retries,
				"error", lastErr,
			)
		}()

		source := src.Repo()      // implements oras.ReadOnlyTarget
		destination := dst.Repo() // implements oras.Target

		attempts++

		// 1) Copy by digest from source → destination
		publishedDesc, copyErr := oras.Copy(ctx, source, srcRef, destination, "", copyOpts)
		if copyErr != nil {
			lastErr = copyErr
			return false, nil
		}

		// 2) Update/tag the destination index to the source tag
		if err := dst.OrasRemote.UpdateIndex(ctx, tag, publishedDesc); err != nil {
			lastErr = err
			return false, nil
		}

		lastErr = nil
		return true, nil
	})
	if err != nil {
		if lastErr != nil {
			return fmt.Errorf("copy failed after retries: %w", lastErr)
		}
		return fmt.Errorf("copy failed after retries: %w", err)
	}

	l.Info("package copied successfully",
		"source", src.Repo().Reference.String(),
		"destination", dst.Repo().Reference.String(),
		"tag", tag,
	)
	return nil
}
