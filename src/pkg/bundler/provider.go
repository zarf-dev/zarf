// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"context"
	"fmt"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Provider is an interface for processing bundles
//
// operations that are common no matter the source should be implemented on bundler
type Provider interface {
	// LoadBundle loads a bundle into the `dst` directory
	//
	// : if tarball
	// : : extracts the package(s) from the tarball
	//
	// : if OCI ref
	// : : pulls the package(s) from the OCI ref
	LoadBundle(requestedPackages []string) ([]ocispec.Descriptor, error)
	// LoadBundleMetadata loads a bundle's metadata (zarf-bundle.yaml) and signature (zarf-bundle.yaml.sig)
	//
	// these two files are placed in the `dst` directory
	//
	// : if tarball
	// : : extracts the metadata from the tarball
	//
	// : if OCI ref
	// : : pulls the metadata from the OCI ref
	LoadBundleMetadata() ([]ocispec.Descriptor, error)

	// ContentStore() *ocistore.Store

	getBundleManifest() error

	// DeployPackge/DeployBundle
	// ViewSBOMs/ExportSBOMs
	// RemovePackage/RemoveBundle
	// ListPackages/ListBundles?
}

// NewProvider returns a new bundler Provider based on the source type
func NewProvider(ctx context.Context, source, destination string) (Provider, error) {
	if utils.IsOCIURL(source) {
		provider := ociProvider{ctx: ctx, src: source, dst: destination}
		remote, err := oci.NewOrasRemote(source)
		if err != nil {
			return nil, err
		}
		provider.OrasRemote = remote
		return &provider, nil
	}
	if !IsValidTarballPath(source) {
		return nil, fmt.Errorf("invalid tarball path: %s", source)
	}
	return &tarballProvider{ctx: ctx, src: source, dst: destination}, nil
}

// ValidateBundleSignature validates the bundle signature
// TODO: implement
func validateBundleSignature(base string) error {
	message.Debugf("Validating bundle signature from %s/%s", base, config.ZarfYAMLSignature)
	return nil
	// err := utils.CosignVerifyBlob(bfs.tmp.ZarfBundleYaml, bfs.tmp.ZarfSig, <keypath>)
	// if err != nil {
	// 	return err
	// }
}
