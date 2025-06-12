// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/zoci"

	"github.com/defenseunicorns/pkg/oci"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"

	"oras.land/oras-go/v2/registry"
)

// PublishFromOCIOptions declares the parameters to publish a package.
type PublishFromOCIOptions struct {
	// Concurrency configures the amount of layers to push in parallel
	Concurrency int
	// Architecture is the architecture we are publishing to
	Architecture string
	RemoteOptions
}

// PublishFromOCI takes a source and destination registry reference and a PublishFromOCIOpts and copies the package from the source to the destination.
func PublishFromOCI(ctx context.Context, src registry.Reference, dst registry.Reference, opts PublishFromOCIOptions) (err error) {
	l := logger.From(ctx)
	start := time.Now()

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

	arch := config.GetArch(opts.Architecture)
	p := oci.PlatformForArch(arch)

	// Set up remote repo client
	srcRemote, err := zoci.NewRemote(ctx, src.String(), p, oci.WithPlainHTTP(opts.PlainHTTP), oci.WithInsecureSkipVerify(opts.InsecureSkipTLSVerify))
	if err != nil {
		return fmt.Errorf("could not instantiate remote: %w", err)
	}
	dstRemote, err := zoci.NewRemote(ctx, dst.String(), p, oci.WithPlainHTTP(opts.PlainHTTP), oci.WithInsecureSkipVerify(opts.InsecureSkipTLSVerify))
	if err != nil {
		return fmt.Errorf("could not instantiate remote: %w", err)
	}

	// Execute copy
	err = zoci.CopyPackage(ctx, srcRemote, dstRemote, opts.Concurrency)
	if err != nil {
		return fmt.Errorf("could not copy package: %w", err)
	}

	l.Debug("publisher2.PublishOCI done", "duration", time.Since(start))
	return nil
}

// PublishPackageOptions declares the parameters to publish a package.
type PublishPackageOptions struct {
	// Concurrency configures the amount of layers to push in parallel
	Concurrency int
	// SigningKeyPath points to a signing key on the local disk.
	SigningKeyPath string
	// SigningKeyPassword holds a password to use the key at SigningKeyPath.
	SigningKeyPassword string
	RemoteOptions
}

// PublishPackage takes a package layout and pushes the package to the given registry. It returns the ref to the pushed package
func PublishPackage(ctx context.Context, pkgLayout *layout.PackageLayout, dst registry.Reference, opts PublishPackageOptions) (string, error) {
	l := logger.From(ctx)

	// Validate inputs
	l.Debug("validating PublishOpts")
	if err := dst.ValidateRegistry(); err != nil {
		return "", fmt.Errorf("invalid registry: %w", err)
	}
	if pkgLayout == nil {
		return "", fmt.Errorf("package layout must be specified")
	}

	if err := pkgLayout.SignPackage(opts.SigningKeyPath, opts.SigningKeyPassword); err != nil {
		return "", fmt.Errorf("unable to sign package: %w", err)
	}

	// Build Reference for remote from registry location and pkg
	pkgRef, err := zoci.ReferenceFromMetadata(dst.String(), pkgLayout.Pkg)
	if err != nil {
		return "", err
	}

	if err := pushToRemote(ctx, pkgLayout, pkgRef, opts.Concurrency, opts.RemoteOptions); err != nil {
		return "", err
	}

	return pkgRef, nil
}

// PublishSkeletonOptions declares the parameters to publish a skeleton package.
type PublishSkeletonOptions struct {
	// Concurrency configures the amount of layers to push in parallel
	Concurrency int
	// SigningKeyPath points to a signing key on the local disk.
	SigningKeyPath string
	// SigningKeyPassword holds a password to use the key at SigningKeyPath.
	SigningKeyPassword string
	// CachePath is used to cache layers from skeleton package pulls
	CachePath string
	RemoteOptions
}

// PublishSkeleton takes a Path to the package definition and uploads a skeleton package to the given a registry. It returns a ref to the skeleton package.
func PublishSkeleton(ctx context.Context, path string, ref registry.Reference, opts PublishSkeletonOptions) (string, error) {
	l := logger.From(ctx)

	// Validate inputs
	l.Debug("validating PublishOpts")
	if err := ref.ValidateRegistry(); err != nil {
		return "", fmt.Errorf("invalid registry: %w", err)
	}
	if path == "" {
		return "", fmt.Errorf("path must be specified")
	}

	// Load package layout
	l.Info("loading skeleton package", "path", path)
	pkg, err := load.PackageDefinition(ctx, path, load.DefinitionOptions{
		CachePath: opts.CachePath,
	})
	if err != nil {
		return "", err
	}
	// Create skeleton buildpath
	createOpts := layout.AssembleSkeletonOptions{
		SigningKeyPath:     opts.SigningKeyPath,
		SigningKeyPassword: opts.SigningKeyPassword,
	}
	pkgLayout, err := layout.AssembleSkeleton(ctx, pkg, path, createOpts)
	if err != nil {
		return "", fmt.Errorf("unable to create skeleton: %w", err)
	}
	// Build Reference for remote from registry location and pkg
	pkgRef, err := zoci.ReferenceFromMetadata(ref.String(), pkgLayout.Pkg)
	if err != nil {
		return "", err
	}
	err = pushToRemote(ctx, pkgLayout, pkgRef, opts.Concurrency, opts.RemoteOptions)
	if err != nil {
		return "", err
	}
	return pkgRef, nil
}

// pushToRemote pushes a package to the given reference
func pushToRemote(ctx context.Context, layout *layout.PackageLayout, ref string, concurrency int, remoteOpts RemoteOptions) error {
	arch := layout.Pkg.Metadata.Architecture
	// Set platform
	platform := oci.PlatformForArch(arch)

	remote, err := zoci.NewRemote(ctx, ref, platform, oci.WithPlainHTTP(remoteOpts.PlainHTTP), oci.WithInsecureSkipVerify(remoteOpts.InsecureSkipTLSVerify))
	if err != nil {
		return fmt.Errorf("could not instantiate remote: %w", err)
	}

	return remote.PushPackage(ctx, layout, concurrency)
}
