// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package oci

import (
	"fmt"
	"io"
	"sync"

	"github.com/defenseunicorns/zarf/src/pkg/message"
)

// CopyPackage copies a package from one OCI registry to another
func CopyPackage(src *OrasRemote, dst *OrasRemote, concurrency int) error {
	srcRoot, err := src.FetchRoot()
	if err != nil {
		return err
	}
	layers := srcRoot.Layers
	layers = append(layers, srcRoot.Config)

	size := int64(0)
	for _, layer := range layers {
		size += layer.Size
	}

	title := fmt.Sprintf("Copying from %s to %s", src.Reference, dst.Reference)
	progressBar := message.NewProgressBar(size, title)
	defer progressBar.Successf("%s into %s", src.Reference, dst.Reference)

	// TODO: goroutine this w/ semaphores
	for _, layer := range layers {
		pr, pw := io.Pipe()

		wg := sync.WaitGroup{}
		wg.Add(2)

		// fetch the layer from the source
		rc, err := src.Fetch(src.Context, layer)
		if err != nil {
			return err
		}
		// TeeReader gets the data from the fetching layer and writes it to the PipeWriter
		tr := io.TeeReader(rc, pw)

		// this goroutine is responsible for pushing the layer to the destination
		go func() {
			defer wg.Done()
			defer pw.Close()

			// get data from the TeeReader and push it to the destination
			// push the layer to the destination
			err = dst.Push(dst.Context, layer, tr)
			if err != nil {
				message.Fatal(err, "failed to push layer")
			}
		}()

		// this goroutine is responsible for updating the progressbar
		go func() {
			defer wg.Done()

			// read from the PipeReader to the progressbar
			if _, err := io.Copy(progressBar, pr); err != nil {
				message.Fatal(err, "failed to copy layer")
			}
		}()

		// wait for the goroutines to finish
		wg.Wait()
	}

	return nil
}
