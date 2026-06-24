// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package zoci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/utils"
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

// fileStoreCloser wraps a file.Store and removes its working directory on Close.
//
// oras.PackManifest copies ManifestAnnotations (including AnnotationTitle) onto the
// descriptor it pushes, so the file store will write the manifest JSON to
// {workingDir}/{title} as a named file. Named files are NOT tracked in tmpFiles,
// so file.Store.Close alone does not delete them. This wrapper ensures the temp
// directory is removed when the store is no longer needed (without the caller needing
// to worry about it), preventing the manifest file from being picked up by
// pkgLayout.Files() on subsequent calls.
type fileStoreCloser struct {
	*file.Store
	dir string
}

func (f *fileStoreCloser) Close() error {
	err := f.Store.Close()
	if rmErr := os.RemoveAll(f.dir); rmErr != nil {
		err = errors.Join(err, rmErr)
	}
	return err
}

// manifestForLayout stages all package files into a temporary OCI file store,
// packs the OCI manifest, and returns the store (caller must Close it), manifest
// descriptor, marshaled config bytes, and total layer size. On error the store
// is closed and its temp directory removed before returning.
func manifestForLayout(ctx context.Context, pkgLayout *layout.PackageLayout) (_ *fileStoreCloser, _ ocispec.Descriptor, _ []byte, _ int64, err error) {
	storeDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, ocispec.Descriptor{}, nil, 0, err
	}
	store, err := file.New(storeDir)
	if err != nil {
		_ = os.RemoveAll(storeDir)
		return nil, ocispec.Descriptor{}, nil, 0, err
	}
	wrapped := &fileStoreCloser{Store: store, dir: storeDir}
	defer func() {
		if err != nil {
			if closeErr := wrapped.Close(); closeErr != nil {
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

	return wrapped, root, configBytes, totalLayerSize, nil
}
