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

// FetchZarfYAML fetches the zarf.yaml file from the remote repository.
func (r *Remote) FetchZarfYAML(ctx context.Context) (v1alpha1.ZarfPackage, error) {
	manifest, err := r.FetchRoot(ctx)
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	descriptor := manifest.Locate(layout.ZarfYAML)
	if oci.IsEmptyDescriptor(descriptor) {
		return v1alpha1.ZarfPackage{}, fmt.Errorf("unable to find %s in the manifest", layout.ZarfYAML)
	}
	b, err := r.FetchLayer(ctx, descriptor)
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	return pkgcfg.ParseBuiltPackageDefinition(ctx, b)
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
