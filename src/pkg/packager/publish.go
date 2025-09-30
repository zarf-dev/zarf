// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/zoci"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"

	"oras.land/oras-go/v2/registry"
)

const defaultPublishRetries = 1

// PublishFromOCIOptions declares the parameters to publish a package.
type PublishFromOCIOptions struct {
	// OCIConcurrency configures the amount of layers to push in parallel
	OCIConcurrency int
	// Architecture is the architecture we are publishing to
	Architecture string
	// Retries is the number of times to retry a failed push
	Retries int
	RemoteOptions
}

// PublishFromOCI takes a source and destination registry reference and a PublishFromOCIOpts and copies the package from the source to the destination.
// src and dst are references to the full package ref, e.g. my-registry.com/my-namespace/my-package:0.0.1
func PublishFromOCI(ctx context.Context, src registry.Reference, dst registry.Reference, opts PublishFromOCIOptions) (err error) {
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

	publishOptions := zoci.PublishOptions{
		OCIConcurrency: opts.OCIConcurrency,
		Retries:        opts.Retries,
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
	// SigningKeyPath points to a signing key on the local disk.
	SigningKeyPath string
	// SigningKeyPassword holds a password to use the key at SigningKeyPath.
	SigningKeyPassword string
	// Retries specifies the number of retries to use
	Retries int
	RemoteOptions
}

// PublishPackage takes a package layout and pushes the package to the given registry.
// dst is the path to the registry namespace, e.g. my-registry.com/my-namespace. The full package ref is created using the package name and returned
func PublishPackage(ctx context.Context, pkgLayout *layout.PackageLayout, dst registry.Reference, opts PublishPackageOptions) (registry.Reference, error) {
	l := logger.From(ctx)

	// disallow infinite or negative
	if opts.Retries <= 0 {
		if opts.Retries < 0 {
			return registry.Reference{}, fmt.Errorf("retries cannot be negative")
		}
		l.Debug("retries set to default", "retries", defaultPublishRetries)
		opts.Retries = defaultPublishRetries
	}

	// Validate inputs
	l.Debug("validating PublishOpts")
	if err := dst.ValidateRegistry(); err != nil {
		return registry.Reference{}, fmt.Errorf("invalid registry: %w", err)
	}
	if pkgLayout == nil {
		return registry.Reference{}, fmt.Errorf("package layout must be specified")
	}

	if err := pkgLayout.SignPackage(opts.SigningKeyPath, opts.SigningKeyPassword); err != nil {
		return registry.Reference{}, fmt.Errorf("unable to sign package: %w", err)
	}

	// Build Reference for remote from registry location and pkg
	pkgRef, err := zoci.ReferenceFromMetadata(dst.String(), pkgLayout.Pkg)
	if err != nil {
		return registry.Reference{}, err
	}

	if err := pushToRemote(ctx, pkgLayout, pkgRef, opts.OCIConcurrency, opts.Retries, opts.RemoteOptions); err != nil {
		return registry.Reference{}, err
	}

	return pkgRef, nil
}

// PublishSkeletonOptions declares the parameters to publish a skeleton package.
type PublishSkeletonOptions struct {
	// OCIConcurrency configures the amount of layers to push in parallel
	OCIConcurrency int
	// SigningKeyPath points to a signing key on the local disk.
	SigningKeyPath string
	// SigningKeyPassword holds a password to use the key at SigningKeyPath.
	SigningKeyPassword string
	// CachePath is used to cache layers from skeleton package pulls
	CachePath string
	// Flavor specifies the flavor to use
	Flavor string
	// Retries specifies the number of retries to use
	Retries int
	RemoteOptions
}

// PublishSkeleton takes a Path to the package definition and uploads a skeleton package to the given a registry.
// dst is the path to the registry namespace, e.g. my-registry.com/my-namespace. The full package ref is created using the package name and returned
func PublishSkeleton(ctx context.Context, path string, ref registry.Reference, opts PublishSkeletonOptions) (registry.Reference, error) {
	l := logger.From(ctx)

	// disallow infinite or negative
	if opts.Retries <= 0 {
		if opts.Retries < 0 {
			return registry.Reference{}, fmt.Errorf("retries cannot be negative")
		}
		l.Debug("retries set to default", "retries", defaultPublishRetries)
		opts.Retries = defaultPublishRetries
	}

	// Validate inputs
	l.Debug("validating PublishOpts")
	if err := ref.ValidateRegistry(); err != nil {
		return registry.Reference{}, fmt.Errorf("invalid registry: %w", err)
	}
	if path == "" {
		return registry.Reference{}, fmt.Errorf("path must be specified")
	}

	// Load package layout
	l.Info("loading skeleton package", "path", path)
	pkg, err := load.PackageDefinition(ctx, path, load.DefinitionOptions{
		CachePath: opts.CachePath,
		Flavor:    opts.Flavor,
	})
	if err != nil {
		return registry.Reference{}, err
	}
	// Create skeleton buildpath
	createOpts := layout.AssembleSkeletonOptions{
		SigningKeyPath:     opts.SigningKeyPath,
		SigningKeyPassword: opts.SigningKeyPassword,
		Flavor:             opts.Flavor,
	}
	pkgLayout, err := layout.AssembleSkeleton(ctx, pkg, path, createOpts)
	if err != nil {
		return registry.Reference{}, fmt.Errorf("unable to create skeleton: %w", err)
	}
	// Build Reference for remote from registry location and pkg
	pkgRef, err := zoci.ReferenceFromMetadata(ref.String(), pkgLayout.Pkg)
	if err != nil {
		return registry.Reference{}, err
	}
	err = pushToRemote(ctx, pkgLayout, pkgRef, opts.OCIConcurrency, opts.Retries, opts.RemoteOptions)
	if err != nil {
		return registry.Reference{}, err
	}
	l.Info("skeleton packages contain metadata and local resources to allow for remote component imports")
	ex := []v1alpha1.ZarfComponent{}
	for _, c := range pkgLayout.Pkg.Components {
		ex = append(ex, v1alpha1.ZarfComponent{
			Name: fmt.Sprintf("import-%s", c.Name),
			Import: v1alpha1.ZarfComponentImport{
				Name: c.Name,
				URL:  helpers.OCIURLPrefix + pkgRef.String(),
			},
		})
	}
	err = utils.ColorPrintYAML(ex, nil, false)
	if err != nil {
		return registry.Reference{}, err
	}
	l.Info("find more info on skeleton packages at https://docs.zarf.dev/faq/#what-is-a-skeleton-zarf-package")
	return pkgRef, nil
}

// pushToRemote pushes a package to the given reference
func pushToRemote(ctx context.Context, layout *layout.PackageLayout, ref registry.Reference, concurrency int, retries int, remoteOpts RemoteOptions) error {
	arch := layout.Pkg.Metadata.Architecture
	// Set platform
	platform := oci.PlatformForArch(arch)

	remote, err := zoci.NewRemote(ctx, ref.String(), platform, oci.WithPlainHTTP(remoteOpts.PlainHTTP), oci.WithInsecureSkipVerify(remoteOpts.InsecureSkipTLSVerify))
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
