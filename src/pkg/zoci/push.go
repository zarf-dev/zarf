// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci contains functions for interacting with Zarf packages stored in OCI registries.
package zoci

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"time"

	"github.com/avast/retry-go/v4"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
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

	src, root, manifestConfigBytes, totalSize, err := manifestForLayout(ctx, pkgLayout)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	defer func() {
		err2 := src.Close()
		err = errors.Join(err, err2)
	}()

	copyOpts := r.OrasRemote.GetDefaultCopyOpts()
	copyOpts.Concurrency = opts.OCIConcurrency

	var publishedDesc ocispec.Descriptor
	err = retry.Do(
		func() error {
			l.Info("pushing package to registry", "destination", r.Repo().Reference.String(),
				"architecture", pkgLayout.Pkg.Build.Architecture, "size", utils.ByteFormat(float64(totalSize), 2))

			manifestConfigDesc, err := r.PushLayer(ctx, manifestConfigBytes, ZarfConfigMediaType)
			if err != nil {
				return err
			}

			if err = src.Tag(ctx, root, root.Digest.String()); err != nil {
				return err
			}

			// Update the total with manifest + config for better progress (optional)
			attemptTotal := totalSize + root.Size + manifestConfigDesc.Size

			trackedRemote := images.NewTrackedTarget(
				r.Repo(),
				attemptTotal,
				images.DefaultReport(r.Log(), "package publish in progress", r.Repo().Reference.String()),
			)
			trackedRemote.StartReporting(ctx)
			defer trackedRemote.StopReporting()

			var copyErr error
			publishedDesc, copyErr = oras.Copy(ctx, src, root.Digest.String(), trackedRemote, "", copyOpts)
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

func annotationsFromMetadata(metadata v1alpha1.ZarfMetadata) map[string]string {
	annotations := map[string]string{
		ocispec.AnnotationTitle:       metadata.Name,
		ocispec.AnnotationDescription: metadata.Description,
	}

	if url := metadata.URL; url != "" {
		annotations[ocispec.AnnotationURL] = url
	}
	if authors := metadata.Authors; authors != "" {
		annotations[ocispec.AnnotationAuthors] = authors
	}
	if documentation := metadata.Documentation; documentation != "" {
		annotations[ocispec.AnnotationDocumentation] = documentation
	}
	if source := metadata.Source; source != "" {
		annotations[ocispec.AnnotationSource] = source
	}
	if vendor := metadata.Vendor; vendor != "" {
		annotations[ocispec.AnnotationVendor] = vendor
	}
	// annotations explicitly defined in `metadata.annotations` take precedence over legacy fields
	maps.Copy(annotations, metadata.Annotations)
	return annotations
}
