// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci contains functions for interacting with Zarf packages stored in OCI registries.
package zoci

import (
	"context"
	"fmt"

	"github.com/defenseunicorns/pkg/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/pkgcfg"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
)

// FetchPackageMetadata resolves the remote reference and fetches its parsed package
// definition from the same platform-selected OCI manifest. It does not download
// unrelated package layers.
func (r *Remote) FetchPackageMetadata(ctx context.Context) (ocispec.Descriptor, v1alpha1.ZarfPackage, error) {
	if r.root != nil {
		pkg, err := r.FetchZarfYAML(ctx)
		if err != nil {
			return ocispec.Descriptor{}, v1alpha1.ZarfPackage{}, err
		}
		return r.rootDescriptor, pkg, nil
	}

	descriptor, err := r.ResolveRoot(ctx)
	if err != nil {
		return ocispec.Descriptor{}, v1alpha1.ZarfPackage{}, err
	}
	pkg, err := r.FetchZarfYAMLFromDescriptor(ctx, descriptor)
	if err != nil {
		return ocispec.Descriptor{}, v1alpha1.ZarfPackage{}, err
	}
	return descriptor, pkg, nil
}

// FetchZarfYAML fetches the zarf.yaml file from the remote repository.
func (r *Remote) FetchZarfYAML(ctx context.Context) (v1alpha1.ZarfPackage, error) {
	manifest, err := r.FetchRoot(ctx)
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	return r.fetchZarfYAMLFromManifest(ctx, manifest)
}

// FetchZarfYAMLFromDescriptor fetches the zarf.yaml file from the manifest identified by descriptor.
// It pins this Remote to the descriptor so subsequent reads use the same package.
func (r *Remote) FetchZarfYAMLFromDescriptor(ctx context.Context, descriptor ocispec.Descriptor) (v1alpha1.ZarfPackage, error) {
	manifest, err := r.FetchRootFromDescriptor(ctx, descriptor)
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	return r.fetchZarfYAMLFromManifest(ctx, manifest)
}

func (r *Remote) fetchZarfYAMLFromManifest(ctx context.Context, manifest *oci.Manifest) (v1alpha1.ZarfPackage, error) {
	descriptor := manifest.Locate(layout.ZarfYAML)
	if oci.IsEmptyDescriptor(descriptor) {
		return v1alpha1.ZarfPackage{}, fmt.Errorf("unable to find %s in the manifest", layout.ZarfYAML)
	}
	b, err := r.FetchLayer(ctx, descriptor)
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	return pkgcfg.ParseMultiDoc(ctx, b)
}

// FetchImagesIndex fetches the images/index.json file from the remote repository.
func (r *Remote) FetchImagesIndex(ctx context.Context) (*ocispec.Index, error) {
	manifest, err := r.FetchRoot(ctx)
	if err != nil {
		return nil, err
	}
	result, err := oci.FetchJSONFile[*ocispec.Index](ctx, r.FetchLayer, manifest, layout.IndexPath)
	if err != nil {
		return nil, err
	}
	return result, nil
}
