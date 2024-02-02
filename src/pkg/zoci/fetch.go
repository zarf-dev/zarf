// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package zoci contains functions for interacting with Zarf packages stored in OCI registries.
package zoci

import (
	"context"

	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/types"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// FetchZarfYAML fetches the zarf.yaml file from the remote repository.
func (o *Remote) FetchZarfYAML(ctx context.Context) (pkg types.ZarfPackage, err error) {
	manifest, err := o.FetchRoot(ctx)
	if err != nil {
		return pkg, err
	}
	return oci.FetchYAMLFile[types.ZarfPackage](ctx, o.FetchLayer, manifest, layout.ZarfYAML)
}

// FetchImagesIndex fetches the images/index.json file from the remote repository.
func (o *Remote) FetchImagesIndex(ctx context.Context) (index *ocispec.Index, err error) {
	manifest, err := o.FetchRoot(ctx)
	if err != nil {
		return index, err
	}
	return oci.FetchJSONFile[*ocispec.Index](ctx, o.FetchLayer, manifest, ZarfPackageIndexPath)
}
