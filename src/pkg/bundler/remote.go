// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	goyaml "github.com/goccy/go-yaml"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content/file"
	ocistore "oras.land/oras-go/v2/content/oci"
)

type ociProvider struct {
	ctx context.Context
	src string
	dst string
	*oci.OrasRemote
	manifest *oci.ZarfOCIManifest
}

func (op *ociProvider) getBundleManifest() error {
	if op.manifest != nil {
		return nil
	}
	root, err := op.FetchRoot()
	if err != nil {
		return err
	}
	op.manifest = root
	return nil
}

// LoadPackage loads a package from a remote bundle
func (op *ociProvider) LoadPackage(sha, destinationDir string, concurrency int) (PathMap, error) {
	if destinationDir == op.dst {
		return nil, fmt.Errorf("destination directory cannot be the same as the bundle directory")
	}

	if err := op.getBundleManifest(); err != nil {
		return nil, err
	}
	pkgManifestDesc := op.manifest.Locate(sha)
	if oci.IsEmptyDescriptor(pkgManifestDesc) {
		return nil, fmt.Errorf("package %s does not exist in this bundle", sha)
	}
	pkgManifest, err := op.FetchManifest(pkgManifestDesc)
	if err != nil {
		return nil, err
	}
	// including the package manifest uses some ORAs FindSuccessors hackery to expand the manifest into all layers
	// as oras.Copy was designed for resolving layers via a manifest reference, not a manifest embedded inside of another
	// image
	layersToPull := []ocispec.Descriptor{pkgManifestDesc}
	for _, layer := range pkgManifest.Layers {
		// only fetch layers that exist
		// since optional-components exists, there will be layers that don't exist
		// as the package's preserved manifest will contain all layers for all components
		ok, _ := op.Repo().Blobs().Exists(op.ctx, layer)
		if ok {
			layersToPull = append(layersToPull, layer)
		}
	}

	store, err := file.New(destinationDir)
	if err != nil {
		return nil, err
	}
	defer store.Close()

	copyOpts := op.CopyOpts
	copyOpts.Concurrency = concurrency

	preCopy := func(_ context.Context, desc ocispec.Descriptor) error {
		message.Debug("Copying", message.JSONValue(desc), "to", destinationDir)
		return nil
	}

	copyOpts.PreCopy = preCopy

	if err := op.CopyWithProgress(layersToPull, store, &copyOpts, destinationDir); err != nil {
		return nil, err
	}

	loaded := make(PathMap)
	for _, layer := range layersToPull {
		rel := layer.Annotations[ocispec.AnnotationTitle]
		loaded[rel] = filepath.Join(destinationDir, rel)
	}
	message.Debug(message.JSONValue(loaded))
	return loaded, nil
}

// LoadBundleMetadata loads a remote bundle's metadata
func (op *ociProvider) LoadBundleMetadata() (PathMap, error) {
	if err := utils.CreateDirectory(filepath.Join(op.dst, blobsDir), 0700); err != nil {
		return nil, err
	}
	layers, err := op.PullMultipleFiles(BundleAlwaysPull, filepath.Join(op.dst, blobsDir))
	if err != nil {
		return nil, err
	}
	loaded := make(PathMap)
	for _, layer := range layers {
		rel := layer.Annotations[ocispec.AnnotationTitle]
		abs := filepath.Join(op.dst, blobsDir, rel)
		absSha := filepath.Join(op.dst, blobsDir, layer.Digest.Encoded())
		if err := os.Rename(abs, absSha); err != nil {
			return nil, err
		}
		loaded[rel] = absSha
	}
	return loaded, nil
}

// LoadBundle loads a bundle from a remote source
func (op *ociProvider) LoadBundle(concurrency int) (PathMap, error) {
	layersToPull := []ocispec.Descriptor{}

	if err := op.getBundleManifest(); err != nil {
		return nil, err
	}

	loaded, err := op.LoadBundleMetadata()
	if err != nil {
		return nil, err
	}

	b, err := os.ReadFile(loaded[BundleYAML])
	if err != nil {
		return nil, err
	}

	var bundle types.ZarfBundle

	if err := goyaml.Unmarshal(b, &bundle); err != nil {
		return nil, err
	}

	for _, pkg := range bundle.Packages {
		sha := strings.Split(pkg.Ref, "@sha256:")[1]
		manifestDesc := op.manifest.Locate(sha)
		manifestDesc.MediaType = ocispec.MediaTypeImageManifest
		if err != nil {
			return nil, err
		}
		manifest, err := op.FetchManifest(manifestDesc)
		if err != nil {
			return nil, err
		}
		layersToPull = append(layersToPull, manifestDesc)
		for _, layer := range manifest.Layers {
			ok, err := op.Repo().Blobs().Exists(op.ctx, layer)
			if err != nil {
				return nil, err
			}
			if ok {
				layersToPull = append(layersToPull, layer)
			}
		}
	}

	copyOpts := op.CopyOpts
	copyOpts.Concurrency = concurrency

	store, err := ocistore.NewWithContext(op.ctx, op.dst)
	if err != nil {
		return nil, err
	}

	rootDesc, err := op.ResolveRoot()
	if err != nil {
		return nil, err
	}
	layersToPull = append(layersToPull, rootDesc)

	if err := op.CopyWithProgress(layersToPull, store, &copyOpts, op.dst); err != nil {
		return nil, err
	}

	for _, layer := range layersToPull {
		sha := layer.Digest.Encoded()
		loaded[sha] = filepath.Join(op.dst, blobsDir, sha)
	}
	loaded["index.json"] = filepath.Join(op.dst, "index.json")

	return loaded, nil
}
