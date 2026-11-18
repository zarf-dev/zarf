// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci contains functions for interacting with Zarf packages stored in OCI registries.
package zoci

import (
	"context"
	"fmt"

	"github.com/oras-project/oras-go/v3"
	"github.com/zarf-dev/zarf/src/pkg/logger"
)

// CopyPackage copies a zarf package from one OCI registry to another using ORAS with retry.
func CopyPackage(ctx context.Context, src *Remote, dst *Remote, opts PublishOptions) (err error) {
	if opts.OCIConcurrency <= 0 {
		opts.OCIConcurrency = DefaultConcurrency
	}

	// Resolve the root digest of the source package (manifest or index)
	srcRoot, err := src.ResolveRoot(ctx)
	if err != nil {
		return err
	}
	srcRef := srcRoot.Digest.String()

	copyOpts := dst.OrasRemote.GetDefaultCopyOpts()
	copyOpts.Concurrency = opts.OCIConcurrency

	// A combined reference pins the source digest but publishes under its tag.
	tag := src.Reference().Tag
	if tag == "" {
		tag = src.Reference().GetReference()
	}
	if opts.Tag != "" {
		tag = opts.Tag
	}

	err = Retry(ctx, opts.Retries,
		func() error {
			logger.From(ctx).Info("copying package",
				"src", src.Reference().String(),
				"dst", dst.Reference().String(),
				"ref", srcRef,
			)

			source := src.Repo()      // implements oras.ReadOnlyTarget
			destination := dst.Repo() // implements oras.Target

			// 1) Copy by digest from source → destination
			publishedDesc, copyErr := oras.Copy(ctx, source, srcRef, destination, "", copyOpts)
			if copyErr != nil {
				return copyErr
			}

			// 2) Update/tag the destination index to the source tag
			return dst.OrasRemote.UpdateIndex(ctx, tag, publishedDesc)
		},
	)
	if err != nil {
		return fmt.Errorf("copy failed after retries: %w", err)
	}

	logger.From(ctx).Info("package copied successfully",
		"source", src.Reference().String(),
		"destination", dst.Reference().String(),
		"tag", tag,
	)
	return nil
}
