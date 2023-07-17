// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/mholt/archiver/v3"
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
	LoadBundle(dst string, requestedPackages []string) ([]ocispec.Descriptor, error)
	// LoadBundleMetadata loads a bundle's metadata (zarf-bundle.yaml) and signature (zarf-bundle.yaml.sig)
	//
	// these two files are placed in the `dst` directory
	//
	// : if tarball
	// : : extracts the metadata from the tarball
	//
	// : if OCI ref
	// : : pulls the metadata from the OCI ref
	LoadBundleMetadata(dst string) error

	ValidateBundleSignature(base string) error
	// DeployPackge/DeployBundle
	// ViewSBOMs/ExportSBOMs
	// RemovePackage/RemoveBundle
	// ListPackages/ListBundles?
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

// tarballProvider is a Processor that works with tarballs
type tarballProvider struct {
	src string
}

// LoadBundle loads a bundle from a tarball
func (tp *tarballProvider) LoadBundle(dst string, requestedPackages []string) ([]ocispec.Descriptor, error) {
	if len(requestedPackages) == 0 {
		if err := archiver.Unarchive(tp.src, dst); err != nil {
			return nil, fmt.Errorf("failed to extract %s to %s: %w", tp.src, dst, err)
		}
		// var desc []ocispec.Descriptor
		// var bundle types.ZarfBundle
		// ctx := context.Background()
		// store, err := ocistore.NewWithContext(ctx, dst)
		// if err != nil {
		// 	return nil, err
		// }
		// for _, pkg := range bundle.Packages {
		// 	manifestSha256 := strings.Split(pkg.Ref, "@sha256:")[1]
		// }

		// if err := utils.ReadYaml(filepath.Join(dst, config.ZarfBundleYAML), &bundle); err != nil {
		// 	return nil, err
		// }

		// TODO: finish me
	}

	return nil, nil
}

// LoadBundleMetadata loads a bundle's metadata from a tarball
func (tp *tarballProvider) LoadBundleMetadata(dst string) error {
	pathsToExtract := oci.BundleAlwaysPull

	for _, path := range pathsToExtract {
		if err := archiver.Extract(tp.src, path, dst); err != nil {
			return fmt.Errorf("failed to extract %s from %s: %w", path, tp.src, err)
		}
	}
	return nil
}

// ValidateBundleSignature validates the bundle signature
func (tp *tarballProvider) ValidateBundleSignature(base string) error {
	return validateBundleSignature(base)
}

// ociProvider is a Processor that works with OCI images
type ociProvider struct {
	src string
	*oci.OrasRemote
}

// LoadBundle loads a bundle from an OCI image
func (op *ociProvider) LoadBundle(dst string, requestedPackages []string) ([]ocispec.Descriptor, error) {
	return op.PullBundle(dst, config.CommonOptions.OCIConcurrency, requestedPackages)
}

// LoadBundleMetadata loads a bundle's metadata from an OCI image
func (op *ociProvider) LoadBundleMetadata(dst string) error {
	return op.PullBundleMetadata(dst)
}

// ValidateBundleSignature validates the bundle signature
func (op *ociProvider) ValidateBundleSignature(base string) error {
	return validateBundleSignature(base)
}

// NewProvider returns a new bundler Provider based on the source type
func NewProvider(source string) (Provider, error) {
	if utils.IsOCIURL(source) {
		provider := ociProvider{src: source}
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
	return &tarballProvider{src: source}, nil
}
