// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package zoci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/file"
)

// DigestForLayout computes the OCI manifest digest for a local package layout
// without pushing to a registry. The result matches the digest that would be
// assigned to the package if published with PushPackage.
func DigestForLayout(ctx context.Context, pkgLayout *layout.PackageLayout) (string, error) {
	store, root, _, _, err := manifestForLayout(ctx, pkgLayout)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := store.Close(); err != nil {
			logger.From(ctx).Warn("failed to close file store", "error", err)
		}
	}()
	return root.Digest.String(), nil
}

// manifestForLayout opens a file store rooted at the package layout directory, stages all
// package files into it, packs the OCI manifest, and returns the store, manifest descriptor,
// marshaled config bytes (needed to push config to the remote), and total layer size in bytes.
// The caller is responsible for closing the store; on error the store is closed before returning.
func manifestForLayout(ctx context.Context, pkgLayout *layout.PackageLayout) (_ *file.Store, _ ocispec.Descriptor, _ []byte, _ int64, err error) {
	store, err := file.New(pkgLayout.DirPath())
	if err != nil {
		return nil, ocispec.Descriptor{}, nil, 0, err
	}
	defer func() {
		if err != nil {
			if closeErr := store.Close(); closeErr != nil {
				err = errors.Join(err, closeErr)
			}
		}
	}()

	files, err := pkgLayout.Files()
	if err != nil {
		return nil, ocispec.Descriptor{}, nil, 0, err
	}

	var (
		descs          []ocispec.Descriptor
		totalLayerSize int64
	)
	for path, name := range files {
		desc, err := store.Add(ctx, name, ZarfLayerMediaTypeBlob, path)
		if err != nil {
			return nil, ocispec.Descriptor{}, nil, 0, err
		}
		descs = append(descs, desc)
		totalLayerSize += desc.Size
	}

	// Sort by digest for deterministic ordering
	sort.Slice(descs, func(i, j int) bool {
		return descs[i].Digest < descs[j].Digest
	})
	annotations := annotationsFromMetadata(pkgLayout.Pkg.Metadata)
	if annotations[ocispec.AnnotationTitle] == "" {
		return nil, ocispec.Descriptor{}, nil, 0, fmt.Errorf("invalid annotations: please include value for %q", ocispec.AnnotationTitle)
	}

	// Back-compatible timestamp parsing → OCI format
	t, err := time.Parse(v1alpha1.BuildTimestampFormat, pkgLayout.Pkg.Build.Timestamp)
	if err != nil {
		return nil, ocispec.Descriptor{}, nil, 0, fmt.Errorf("unable to parse build timestamp: %w", err)
	}
	annotations[ocispec.AnnotationCreated] = t.Format(OCITimestampFormat)

	configBytes, err := json.Marshal(pkgLayout.Pkg)
	if err != nil {
		return nil, ocispec.Descriptor{}, nil, 0, err
	}
	configDesc := content.NewDescriptorFromBytes(ZarfConfigMediaType, configBytes)

	root, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, "", oras.PackManifestOptions{
		Layers:              descs,
		ConfigDescriptor:    &configDesc,
		ManifestAnnotations: annotations,
	})
	if err != nil {
		return nil, ocispec.Descriptor{}, nil, 0, fmt.Errorf("unable to pack manifest: %w", err)
	}

	return store, root, configBytes, totalLayerSize, nil
}
