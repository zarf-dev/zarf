// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/zoci"

	"github.com/defenseunicorns/pkg/oci"
	layout2 "github.com/zarf-dev/zarf/src/internal/packager2/layout"

	"oras.land/oras-go/v2/registry"
)

// PublishFromOCIOpts declares the parameters to publish a package.
type PublishFromOCIOpts struct {
	// Concurrency configures the zoci push concurrency if empty defaults to 3.
	Concurrency int
	// SigningKeyPath points to a signing key on the local disk.
	SigningKeyPath string
	// SigningKeyPassword holds a password to use the key at SigningKeyPath.
	SigningKeyPassword string
	// SkipSignatureValidation flags whether Publish should skip validating the signature.
	SkipSignatureValidation bool
	// WithPlainHTTP falls back to plain HTTP for the registry calls instead of TLS.
	WithPlainHTTP bool
	// PublicKeyPath validates the create time signage of a package.
	PublicKeyPath string
	// Architecture is the architecture we are publishing to
	Architecture string
}

// PublishFromOCI takes a source and destination registry reference and a PublishFromOCIOpts and copies the package from the source to the destination.
func PublishFromOCI(ctx context.Context, src registry.Reference, dst registry.Reference, opts PublishFromOCIOpts) error {
	l := logger.From(ctx)
	start := time.Now()

	if err := src.Validate(); err != nil {
		return err
	}

	if err := dst.Validate(); err != nil {
		return err
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
	srcRemote, err := zoci.NewRemote(ctx, src.String(), p, oci.WithPlainHTTP(opts.WithPlainHTTP))
	if err != nil {
		return fmt.Errorf("could not instantiate remote: %w", err)
	}
	dstRemote, err := zoci.NewRemote(ctx, dst.String(), p, oci.WithPlainHTTP(opts.WithPlainHTTP))
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

// PublishPackageOpts declares the parameters to publish a package.
type PublishPackageOpts struct {
	// Concurrency configures the zoci push concurrency if empty defaults to 3.
	Concurrency int
	// SigningKeyPath points to a signing key on the local disk.
	SigningKeyPath string
	// SigningKeyPassword holds a password to use the key at SigningKeyPath.
	SigningKeyPassword string
	// SkipSignatureValidation flags whether Publish should skip validating the signature.
	SkipSignatureValidation bool
	// WithPlainHTTP falls back to plain HTTP for the registry calls instead of TLS.
	WithPlainHTTP bool
	// PublicKeyPath validates the create time signage of a package.
	PublicKeyPath string
	// Architecture is the architecture we are publishing to
	Architecture string
}

// PublishPackage takes a Path to the location of the built package, a ref to a registry, and a PublishOpts and uploads to the target OCI registry.
func PublishPackage(ctx context.Context, path string, dst registry.Reference, opts PublishPackageOpts) error {
	l := logger.From(ctx)

	// Validate inputs
	l.Debug("validating PublishOpts")
	if err := dst.ValidateRegistry(); err != nil {
		return fmt.Errorf("invalid registry: %w", err)
	}
	if path == "" {
		return fmt.Errorf("path must be specified")
	}

	// Load package layout
	l.Info("loading package", "path", path)
	layoutOpts := layout2.PackageLayoutOptions{
		PublicKeyPath:           opts.PublicKeyPath,
		SkipSignatureValidation: opts.SkipSignatureValidation,
	}
	pkgLayout, err := layout2.LoadFromTar(ctx, path, layoutOpts)
	if err != nil {
		return fmt.Errorf("unable to load package: %w", err)
	}

	return pushToRemote(ctx, pkgLayout, dst, opts.Concurrency, opts.WithPlainHTTP)
}

// PublishSkeletonOpts declares the parameters to publish a skeleton package.
type PublishSkeletonOpts struct {
	// Concurrency configures the zoci push concurrency if empty defaults to 3.
	Concurrency int
	// SigningKeyPath points to a signing key on the local disk.
	SigningKeyPath string
	// SigningKeyPassword holds a password to use the key at SigningKeyPath.
	SigningKeyPassword string
	// WithPlainHTTP falls back to plain HTTP for the registry calls instead of TLS.
	WithPlainHTTP bool
}

// PublishSkeleton takes a Path to the location of the build package, a ref to a registry, and a PublishOpts and uploads the skeleton package to the target OCI registry.
func PublishSkeleton(ctx context.Context, path string, ref registry.Reference, opts PublishSkeletonOpts) error {
	l := logger.From(ctx)

	// Validate inputs
	l.Debug("validating PublishOpts")
	if err := ref.ValidateRegistry(); err != nil {
		return fmt.Errorf("invalid registry: %w", err)
	}
	if path == "" {
		return fmt.Errorf("path must be specified")
	}

	// Load package layout
	l.Info("loading skeleton package", "path", path)
	// Create skeleton buildpath
	createOpts := layout2.CreateOptions{
		SigningKeyPath:     opts.SigningKeyPath,
		SigningKeyPassword: opts.SigningKeyPassword,
		SetVariables:       map[string]string{},
	}
	buildPath, err := layout2.CreateSkeleton(ctx, path, createOpts)
	if err != nil {
		return fmt.Errorf("unable to create skeleton: %w", err)
	}

	layoutOpts := layout2.PackageLayoutOptions{
		SkipSignatureValidation: true,
		IsPartial:               false,
	}
	pkgLayout, err := layout2.LoadFromDir(ctx, buildPath, layoutOpts)
	if err != nil {
		return fmt.Errorf("unable to load skeleton: %w", err)
	}

	return pushToRemote(ctx, pkgLayout, ref, opts.Concurrency, opts.WithPlainHTTP)
}

// pushToRemote pushes a package to a remote at ref.
func pushToRemote(ctx context.Context, layout *layout2.PackageLayout, ref registry.Reference, concurrency int, plainHTTP bool) error {
	// Build Reference for remote from registry location and pkg
	r, err := layout2.ReferenceFromMetadata(ref.String(), layout.Pkg)
	if err != nil {
		return err
	}

	arch := layout.Pkg.Metadata.Architecture
	// Set platform
	p := oci.PlatformForArch(arch)

	// Set up remote repo client
	rem, err := layout2.NewRemote(ctx, r, p, oci.WithPlainHTTP(plainHTTP))
	if err != nil {
		return fmt.Errorf("could not instantiate remote: %w", err)
	}

	return rem.Push(ctx, layout, concurrency)
}
