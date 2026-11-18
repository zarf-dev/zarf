// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/oras-project/oras-go/v3/errdef"
	internalv1alpha1 "github.com/zarf-dev/zarf/src/internal/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/helpers"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
	"github.com/zarf-dev/zarf/src/types"

	"github.com/zarf-dev/zarf/src/pkg/oci"
	"github.com/zarf-dev/zarf/src/pkg/packager/assemble"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	legacyerrdef "oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/registry"
)

const defaultPublishRetries = 1

// PublishFromOCI copies a package between registry references.
//
// Deprecated: Use CopyPackage with oci.Reference values.
func PublishFromOCI(ctx context.Context, src registry.Reference, dst registry.Reference, opts PublishFromOCIOptions) error {
	return legacyReferenceError(CopyPackage(ctx, referenceFromLegacy(src), referenceFromLegacy(dst), opts))
}

// PublishPackage pushes a package to a registry namespace and returns its full reference.
//
// Deprecated: Use PublishPackageToOCI with an oci.Reference destination.
func PublishPackage(ctx context.Context, pkgLayout *layout.PackageLayout, dst registry.Reference, opts PublishPackageOptions) (registry.Reference, error) {
	return legacyReferenceResult(PublishPackageToOCI(ctx, pkgLayout, referenceFromLegacy(dst), opts))
}

// PublishSkeleton publishes a legacy skeleton package to a registry namespace.
//
// Deprecated: Use PublishSkeletonToOCI with an oci.Reference destination.
// Only supported by deprecated v1alpha1 packages.
func PublishSkeleton(ctx context.Context, path string, ref registry.Reference, opts PublishSkeletonOptions) (registry.Reference, error) {
	return legacyReferenceResult(PublishSkeletonToOCI(ctx, path, referenceFromLegacy(ref), opts))
}

func referenceFromLegacy(ref registry.Reference) oci.Reference {
	result := oci.Reference{Registry: ref.Registry, Repository: ref.Repository}
	if strings.Contains(ref.Reference, ":") {
		result.Digest = ref.Reference
	} else {
		result.Tag = ref.Reference
	}
	return result
}

func legacyReferenceResult(ref oci.Reference, err error) (registry.Reference, error) {
	if err != nil {
		return registry.Reference{}, legacyReferenceError(err)
	}
	return registry.Reference{Registry: ref.Registry, Repository: ref.Repository, Reference: ref.GetReference()}, nil
}

func legacyReferenceError(err error) error {
	if errors.Is(err, errdef.ErrInvalidReference) {
		return fmt.Errorf("%w: %w", legacyerrdef.ErrInvalidReference, err)
	}
	return err
}

// PublishFromOCIOptions declares the parameters to copy a package.
//
// Deprecated: Use CopyPackageOptions.
type PublishFromOCIOptions = CopyPackageOptions

// CopyPackageOptions configures copying a package between OCI registries.
type CopyPackageOptions struct {
	// OCIConcurrency configures the amount of layers to push in parallel
	OCIConcurrency int
	// Architecture is the architecture we are publishing to
	Architecture string
	// Retries is the number of times to retry a failed push
	Retries int
	types.RemoteOptions
}

// CopyPackage copies a package between OCI registries.
// References identify the package repositories, including any source digest and destination tag.
func CopyPackage(ctx context.Context, src oci.Reference, dst oci.Reference, opts CopyPackageOptions) (err error) {
	l := logger.From(ctx)
	start := time.Now()

	// disallow infinite or negative
	if opts.Retries <= 0 {
		if opts.Retries < 0 {
			return fmt.Errorf("retries cannot be negative")
		}
		l.Debug("retries set to default", "retries", defaultPublishRetries)
		opts.Retries = defaultPublishRetries
	}

	if err := src.Validate(); err != nil {
		return fmt.Errorf("failed to validate source registry: %w", err)
	}

	if err := dst.Validate(); err != nil {
		return fmt.Errorf("failed to validate destination registry: %w", err)
	}

	srcParts := strings.Split(src.Repository, "/")
	srcPackageName := srcParts[len(srcParts)-1]

	dstParts := strings.Split(dst.Repository, "/")
	dstPackageName := dstParts[len(dstParts)-1]

	if srcPackageName != dstPackageName {
		return fmt.Errorf("source and destination repositories must have the same name")
	}

	arch := helpers.ResolveArch(opts.Architecture)
	p := oci.PlatformForArch(arch)

	// Set up remote repo clients.
	remoteOptions := zoci.RemoteClientOptions{
		RemoteOptions: opts.RemoteOptions,
	}
	srcRemote, err := zoci.NewRemote(ctx, src.String(), p, remoteOptions)
	if err != nil {
		return fmt.Errorf("could not instantiate source remote: %w", err)
	}
	dstRemote, err := zoci.NewRemote(ctx, dst.String(), p, remoteOptions)
	if err != nil {
		return fmt.Errorf("could not instantiate destination remote: %w", err)
	}

	publishOptions := zoci.PublishOptions{
		OCIConcurrency: opts.OCIConcurrency,
		Retries:        opts.Retries,
		Tag:            dst.Tag,
	}
	if publishOptions.Tag == "" {
		publishOptions.Tag = dst.Digest
	}

	// Execute copy
	err = zoci.CopyPackage(ctx, srcRemote, dstRemote, publishOptions)
	if err != nil {
		return fmt.Errorf("could not copy package: %w", err)
	}

	l.Debug("publisher2.PublishOCI done", "duration", time.Since(start))
	return nil
}

// PublishPackageOptions declares the parameters to publish a package.
type PublishPackageOptions struct {
	// OCIConcurrency configures the amount of layers to push in parallel
	OCIConcurrency int
	// Retries specifies the number of retries to use
	Retries int
	types.RemoteOptions
	// Tag is an optional tag for the OCI reference separate from the package metadata.version
	Tag string
}

// PublishPackageToOCI pushes a package to a registry namespace.
// It appends the package name to dst and returns the full published reference.
func PublishPackageToOCI(ctx context.Context, pkgLayout *layout.PackageLayout, dst oci.Reference, opts PublishPackageOptions) (oci.Reference, error) {
	l := logger.From(ctx)

	// disallow infinite or negative
	if opts.Retries <= 0 {
		if opts.Retries < 0 {
			return oci.Reference{}, fmt.Errorf("retries cannot be negative")
		}
		l.Debug("retries set to default", "retries", defaultPublishRetries)
		opts.Retries = defaultPublishRetries
	}

	// Validate inputs
	l.Debug("validating PublishOpts")
	if err := dst.ValidateRegistry(); err != nil {
		return oci.Reference{}, fmt.Errorf("invalid registry: %w", err)
	}
	if pkgLayout == nil {
		return oci.Reference{}, fmt.Errorf("package layout must be specified")
	}

	internalv1alpha1.WarnIfDeprecated(ctx, pkgLayout.Definition())

	referenceOptions := zoci.ReferenceFromMetadataOptions{
		Tag: opts.Tag,
	}
	// Build Reference for remote from registry location and pkg
	pkgRef, err := zoci.OCIReferenceFromMetadataWithOptions(dst.String(), pkgLayout.Definition(), referenceOptions)
	if err != nil {
		return oci.Reference{}, err
	}

	if err := pushToRemote(ctx, pkgLayout, pkgRef, opts.OCIConcurrency, opts.Retries, opts.RemoteOptions); err != nil {
		return oci.Reference{}, err
	}

	return pkgRef, nil
}

// PublishSkeletonOptions declares the parameters to publish a skeleton package.
type PublishSkeletonOptions struct {
	// Architecture selects package components, falling back to metadata and then the host.
	Architecture string
	// OCIConcurrency configures the amount of layers to push in parallel
	OCIConcurrency int
	// CachePath is used to cache layers from skeleton package pulls
	CachePath string
	// Flavor specifies the flavor to use
	Flavor string
	// Retries specifies the number of retries to use
	Retries int
	// SkipVersionCheck skips version requirement validation
	SkipVersionCheck bool
	// WithBuildMachineInfo controls whether to include build machine information (hostname and username) in the package metadata
	WithBuildMachineInfo bool
	types.RemoteOptions
	// Tag is an optional tag for the OCI reference separate from the package metadata.version
	Tag string
}

// PublishSkeletonToOCI publishes a skeleton package to a registry namespace.
// It appends the package name to ref and returns the full published reference.
//
// Deprecated: Only supported by deprecated v1alpha1 packages.
func PublishSkeletonToOCI(ctx context.Context, path string, ref oci.Reference, opts PublishSkeletonOptions) (_ oci.Reference, err error) {
	l := logger.From(ctx)

	// disallow infinite or negative
	if opts.Retries <= 0 {
		if opts.Retries < 0 {
			return oci.Reference{}, fmt.Errorf("retries cannot be negative")
		}
		l.Debug("retries set to default", "retries", defaultPublishRetries)
		opts.Retries = defaultPublishRetries
	}

	cachePath, err := utils.ResolveCachePath(opts.CachePath)
	if err != nil {
		return oci.Reference{}, err
	}
	opts.CachePath = cachePath

	// Validate inputs
	l.Debug("validating PublishOpts")
	if err := ref.ValidateRegistry(); err != nil {
		return oci.Reference{}, fmt.Errorf("invalid registry: %w", err)
	}
	if path == "" {
		return oci.Reference{}, fmt.Errorf("path must be specified")
	}

	// Load package layout
	l.Info("loading skeleton package", "path", path)
	loaded, err := load.Package(ctx, path, load.PackageOptions{
		DefinitionOptions: load.DefinitionOptions{
			Architecture:     opts.Architecture,
			CachePath:        opts.CachePath,
			Flavor:           opts.Flavor,
			SkipVersionCheck: opts.SkipVersionCheck,
			RemoteOptions:    opts.RemoteOptions,
		},
	})
	if err != nil {
		return oci.Reference{}, err
	}
	defer func() {
		err = errors.Join(err, loaded.Close())
	}()
	pkg := loaded.Definition
	internalv1alpha1.WarnAuthorIfDeprecated(ctx, pkg)
	for _, comp := range pkg.Components {
		if comp.ImageArchives != nil {
			return oci.Reference{}, fmt.Errorf("cannot publish skeleton package with image archives")
		}
	}
	// Create skeleton buildpath
	createOpts := assemble.AssembleSkeletonOptions{
		Flavor:               opts.Flavor,
		WithBuildMachineInfo: opts.WithBuildMachineInfo,
	}
	pkgLayout, err := assemble.AssembleSkeleton(ctx, loaded, createOpts)
	if err != nil {
		return oci.Reference{}, fmt.Errorf("unable to create skeleton: %w", err)
	}
	referenceOptions := zoci.ReferenceFromMetadataOptions{
		Tag: opts.Tag,
	}
	// Build Reference for remote from registry location and pkg
	pkgRef, err := zoci.OCIReferenceFromMetadataWithOptions(ref.String(), pkgLayout.Definition(), referenceOptions)
	if err != nil {
		return oci.Reference{}, err
	}
	err = pushToRemote(ctx, pkgLayout, pkgRef, opts.OCIConcurrency, opts.Retries, opts.RemoteOptions)
	if err != nil {
		return oci.Reference{}, err
	}
	return pkgRef, nil
}

// pushToRemote pushes a package to the given reference
func pushToRemote(ctx context.Context, layout *layout.PackageLayout, ref oci.Reference, concurrency int, retries int, remoteOpts types.RemoteOptions) error {
	arch := layout.Definition().Metadata.Architecture
	// Set platform
	platform := oci.PlatformForArch(arch)

	remote, err := zoci.NewRemote(ctx, ref.String(), platform, zoci.RemoteClientOptions{
		RemoteOptions: remoteOpts,
	})
	if err != nil {
		return fmt.Errorf("could not instantiate remote: %w", err)
	}

	publishOptions := zoci.PublishOptions{
		OCIConcurrency: concurrency,
		Retries:        retries,
	}

	_, err = remote.PushPackage(ctx, layout, publishOptions)
	if err != nil {
		return fmt.Errorf("could not push package: %w", err)
	}

	return nil
}
