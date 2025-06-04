// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/pkg/oci"

	layout2 "github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/logger"
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

	// applicable when output is an OCI registry
	RemoteOptions
}

// Create takes a path to a directory containing a ZarfPackageConfig and creates an archived Zarf package in the output directory
func Create(ctx context.Context, packagePath string, output string, opts CreateOptions) (err error) {
	if opts.SkipSBOM && opts.SBOMOut != "" {
		return fmt.Errorf("cannot skip SBOM creation and specify an SBOM output directory")
	}

	createOpts := layout2.CreateOptions{
		AssembleOptions: layout2.AssembleOptions{
			SkipSBOM:                opts.SkipSBOM,
			OCIConcurrency:          opts.OCIConcurrency,
			DifferentialPackagePath: opts.DifferentialPackagePath,
			Flavor:                  opts.Flavor,
			RegistryOverrides:       opts.RegistryOverrides,
			SigningKeyPath:          opts.SigningKeyPath,
			SigningKeyPassword:      opts.SigningKeyPassword,
		},
		SetVariables: opts.SetVariables,
	}
	pkgLayout, err := layout2.CreatePackage(ctx, packagePath, createOpts)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, pkgLayout.Cleanup())
	}()

	if helpers.IsOCIURL(output) {
		ref, err := layout2.ReferenceFromMetadata(output, pkgLayout.Pkg)
		if err != nil {
			return err
		}
		remote, err := layout2.NewRemote(ctx, ref, oci.PlatformForArch(pkgLayout.Pkg.Build.Architecture),
			oci.WithPlainHTTP(opts.PlainHTTP), oci.WithInsecureSkipVerify(opts.InsecureSkipTLSVerify))
		if err != nil {
			return err
		}
		err = remote.Push(ctx, pkgLayout, opts.OCIConcurrency)
		if err != nil {
			return err
		}
	} else {
		err = pkgLayout.Archive(ctx, output, opts.MaxPackageSizeMB)
		if err != nil {
			return err
		}
	}

	if opts.SBOMOut != "" {
		err := pkgLayout.GetSBOM(ctx, filepath.Join(opts.SBOMOut, pkgLayout.Pkg.Metadata.Name))
		// Don't fail package create if the package doesn't have an sbom
		var noSBOMErr *layout2.NoSBOMAvailableError
		if errors.As(err, &noSBOMErr) {
			logger.From(ctx).Error(fmt.Sprintf("cannot output sbom: %s", err.Error()))
			return nil
		}
		if err != nil {
			return err
		}
	}
	return nil
}
