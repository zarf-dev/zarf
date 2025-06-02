// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/defenseunicorns/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/file"
)

// Push pushes the given package layout to the remote registry.
func (r *Remote) Push(ctx context.Context, pkgLayout *PackageLayout, concurrency int) (err error) {
	logger.From(ctx).Info("pushing package to registry",
		"destination", r.Repo().Reference.String(),
		"architecture", pkgLayout.Pkg.Build.Architecture)

	src, err := file.New("")
	if err != nil {
		return err
	}
	defer func(src *file.Store) {
		err2 := src.Close()
		err = errors.Join(err, err2)
	}(src)

	descs := []ocispec.Descriptor{}
	files, err := pkgLayout.Files()
	if err != nil {
		return err
	}
	for path, name := range files {
		desc, err := src.Add(ctx, name, ZarfLayerMediaTypeBlob, path)
		if err != nil {
			return err
		}
		descs = append(descs, desc)
	}

	// Sort by Digest string
	sort.Slice(descs, func(i, j int) bool {
		return descs[i].Digest < descs[j].Digest
	})

	annotations := annotationsFromMetadata(pkgLayout.Pkg.Metadata)

	// Perform the conversion of the string timestamp to the appropriate format in order to maintain backwards compatibility

	t, err := time.Parse(CreateTimestampFormat, pkgLayout.Pkg.Build.Timestamp)
	if err != nil {
		// if we change the format of the timestamp, we need to update the conversion here
		// and also account for an error state for mismatch with older formats
		return fmt.Errorf("unable to parse timestamp: %w", err)
	}
	annotations[ocispec.AnnotationCreated] = t.Format(OCITimestampFormat)

	manifestConfigDesc, err := r.CreateAndPushManifestConfig(ctx, annotations, ZarfConfigMediaType)
	if err != nil {
		return err
	}
	// here is where the manifest is created and written to the filesystem given the file.store Push() functionality
	root, err := r.PackAndTagManifest(ctx, src, descs, manifestConfigDesc, annotations)
	if err != nil {
		return err
	}

	defer func() {
		// remove the dangling manifest file created by the PackAndTagManifest
		// should this behavior change, we should expect this to begin producing an error
		err2 := os.Remove(pkgLayout.Pkg.Metadata.Name)
		err = errors.Join(err, err2)
	}()

	copyOpts := r.GetDefaultCopyOpts()
	copyOpts.Concurrency = concurrency
	publishedDesc, err := oras.Copy(ctx, src, root.Digest.String(), r.Repo(), "", copyOpts)
	if err != nil {
		return err
	}

	err = r.UpdateIndex(ctx, r.Repo().Reference.Reference, publishedDesc)
	if err != nil {
		return err
	}

	return nil
}

// CopyPackage copies a zarf package from one OCI registry to another
func CopyPackage(ctx context.Context, src *Remote, dst *Remote, concurrency int) (err error) {
	l := logger.From(ctx)
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}

	srcManifest, err := src.FetchRoot(ctx)
	if err != nil {
		return err
	}
	l.Info("copying package",
		"src", src.Repo().Reference.String(),
		"dst", dst.Repo().Reference.String())
	if err := oci.Copy(ctx, src.OrasRemote, dst.OrasRemote, nil, concurrency, nil); err != nil {
		return err
	}

	srcRoot, err := src.ResolveRoot(ctx)
	if err != nil {
		return err
	}

	b, err := srcManifest.MarshalJSON()
	if err != nil {
		return err
	}
	expected := content.NewDescriptorFromBytes(ocispec.MediaTypeImageManifest, b)

	if err := dst.Repo().Manifests().PushReference(ctx, expected, bytes.NewReader(b), srcRoot.Digest.String()); err != nil {
		return err
	}

	tag := src.Repo().Reference.Reference
	if err := dst.UpdateIndex(ctx, tag, expected); err != nil {
		return err
	}

	src.Log().Info(fmt.Sprintf("Published %s to %s", src.Repo().Reference, dst.Repo().Reference))
	return nil
}
