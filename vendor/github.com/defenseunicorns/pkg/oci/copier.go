// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2024-Present Defense Unicorns

package oci

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"github.com/defenseunicorns/pkg/helpers/v2"
)

// Copy copies an artifact from one OCI registry to another
func Copy(ctx context.Context, src *OrasRemote, dst *OrasRemote,
	include func(d ocispec.Descriptor) bool, concurrency int, progressBar helpers.ProgressWriter) (err error) {
	if progressBar == nil {
		progressBar = helpers.DiscardProgressWriter{}
	}
	// create a new semaphore to limit concurrency
	sem := semaphore.NewWeighted(int64(concurrency))

	// fetch the source root manifest
	srcRoot, err := src.FetchRoot(ctx)
	if err != nil {
		return err
	}

	layers := helpers.Filter(srcRoot.Layers, include)
	layers = append(layers, srcRoot.Config)

	start := time.Now()

	for idx, layer := range layers {
		b, err := json.MarshalIndent(layer, "", "  ")
		if err != nil {
			src.log.Debug("failed to marshal json", "error", err.Error())
		}
		src.log.Debug("Copying layer", "layer", string(b))
		if err := sem.Acquire(ctx, 1); err != nil {
			return err
		}

		// check if the layer already exists in the destination
		exists, err := dst.repo.Exists(ctx, layer)
		if err != nil {
			return err
		}
		if exists {
			src.log.Debug("layer already exists in destination, skipping")
			b := make([]byte, layer.Size)
			_, _ = progressBar.Write(b)
			progressBar.Updatef("[%d/%d] layers copied", idx+1, len(layers))
			sem.Release(1)
			continue
		}

		eg, ectx := errgroup.WithContext(ctx)
		eg.SetLimit(2)

		// fetch the layer from the source
		rc, err := src.repo.Fetch(ectx, layer)
		if err != nil {
			return err
		}

		// create a new pipe so we can write to both the progressbar and the destination at the same time
		pr, pw := io.Pipe()

		// TeeReader gets the data from the fetching layer and writes it to the PipeWriter
		tr := io.TeeReader(rc, pw)

		// this goroutine is responsible for pushing the layer to the destination
		eg.Go(func() error {
			defer pw.Close()

			// get data from the TeeReader and push it to the destination
			// push the layer to the destination
			err = dst.repo.Push(ectx, layer, tr)
			if err != nil {
				return fmt.Errorf("failed to push layer %s to %s: %w", layer.Digest, dst.repo.Reference, err)
			}
			return nil
		})

		// this goroutine is responsible for updating the progressbar
		eg.Go(func() error {
			// read from the PipeReader to the progressbar
			if _, err := io.Copy(progressBar, pr); err != nil {
				return fmt.Errorf("failed to update progress on layer %s: %w", layer.Digest, err)
			}
			return nil
		})

		// wait for the goroutines to finish
		if err := eg.Wait(); err != nil {
			return err
		}
		sem.Release(1)
		progressBar.Updatef("[%d/%d] layers copied", idx+1, len(layers))
	}

	duration := time.Since(start)
	src.log.Debug("copy successful", "source", src.repo.Reference, "destination", dst.repo.Reference, "concurrency", concurrency, "duration", duration)

	return nil
}
