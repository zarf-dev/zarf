// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
)

// CreateOptions are the optional parameters to create
type CreateOptions struct {
	Flavor                  string
	RegistryOverrides       map[string]string
	SigningKeyPath          string
	SigningKeyPassword      string
	SetVariables            map[string]string
	MaxPackageSizeMB        int
	SBOMOut                 string
	SkipSBOM                bool
	DifferentialPackagePath string
	OCIConcurrency          int
	CachePath               string
	// applicable when output is an OCI registry
	RemoteOptions
}

// Create takes a path to a directory containing a ZarfPackageConfig and returns the path to the created package
func Create(ctx context.Context, packagePath string, output string, opts CreateOptions) (_ string, err error) {
	if opts.SkipSBOM && opts.SBOMOut != "" {
		return "", fmt.Errorf("cannot skip SBOM creation and specify an SBOM output directory")
	}

	loadOpts := load.DefinitionOptions{
		Flavor:       opts.Flavor,
		SetVariables: opts.SetVariables,
		CachePath:    opts.CachePath,
	}
	pkg, err := load.PackageDefinition(ctx, packagePath, loadOpts)
	if err != nil {
		return "", err
	}

	var differentialPkg v1alpha1.ZarfPackage
	if opts.DifferentialPackagePath != "" {
		pkgLayout, err := LoadPackage(ctx, opts.DifferentialPackagePath, LoadOptions{
			Architecture:            pkg.Metadata.Architecture,
			RemoteOptions:           opts.RemoteOptions,
			LayersSelector:          zoci.MetadataLayers,
			OCIConcurrency:          opts.OCIConcurrency,
			CachePath:               opts.CachePath,
			SkipSignatureValidation: true,
		})
		if err != nil {
			return "", fmt.Errorf("failed to load differential package: %w", err)
		}
		differentialPkg = pkgLayout.Pkg
	}

	assembleOpt := layout.AssembleOptions{
		SkipSBOM:            opts.SkipSBOM,
		OCIConcurrency:      opts.OCIConcurrency,
		DifferentialPackage: differentialPkg,
		Flavor:              opts.Flavor,
		RegistryOverrides:   opts.RegistryOverrides,
		SigningKeyPath:      opts.SigningKeyPath,
		SigningKeyPassword:  opts.SigningKeyPassword,
		CachePath:           opts.CachePath,
	}
	pkgLayout, err := layout.AssemblePackage(ctx, pkg, packagePath, assembleOpt)
	if err != nil {
		return "", err
	}
	defer func() {
		err = errors.Join(err, pkgLayout.Cleanup())
	}()

	var packageLocation string
	if helpers.IsOCIURL(output) {
		ref, err := zoci.ReferenceFromMetadata(output, pkgLayout.Pkg)
		if err != nil {
			return "", err
		}
		remote, err := zoci.NewRemote(ctx, ref.String(), oci.PlatformForArch(pkgLayout.Pkg.Build.Architecture),
			oci.WithPlainHTTP(opts.PlainHTTP), oci.WithInsecureSkipVerify(opts.InsecureSkipTLSVerify))
		if err != nil {
			return "", err
		}
		// Setting a default here for retries as the flag is less intuitive during create
		publishOptions := zoci.PublishOptions{
			Retries:        1,
			OCIConcurrency: opts.OCIConcurrency,
		}
		_, err = remote.PushPackage(ctx, pkgLayout, publishOptions)
		if err != nil {
			return "", err
		}
		packageLocation = ref.String()
	} else {
		packageLocation, err = pkgLayout.Archive(ctx, output, opts.MaxPackageSizeMB)
		if err != nil {
			return "", err
		}
	}

	if opts.SBOMOut != "" {
		err := pkgLayout.GetSBOM(ctx, filepath.Join(opts.SBOMOut, pkgLayout.Pkg.Metadata.Name))
		// Don't fail package create if the package doesn't have an sbom
		var noSBOMErr *layout.NoSBOMAvailableError
		if errors.As(err, &noSBOMErr) {
			logger.From(ctx).Error(fmt.Sprintf("SBOM not available in package: %s", err.Error()))
			return packageLocation, nil
		}
		if err != nil {
			return "", err
		}
	}
	return packageLocation, nil
}
