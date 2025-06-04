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

	"github.com/zarf-dev/zarf/src/config"
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
}

// Create takes a path to a directory containing a ZarfPackageConfig and creates an archived Zarf package in the output directory
func Create(ctx context.Context, packagePath string, output string, opt CreateOptions) (err error) {
	if opt.SkipSBOM && opt.SBOMOut != "" {
		return fmt.Errorf("cannot skip SBOM creation and specify an SBOM output directory")
	}

	createOpt := layout2.CreateOptions{
		AssembleOptions: layout2.AssembleOptions{
			SkipSBOM:                opt.SkipSBOM,
			OCIConcurrency:          opt.OCIConcurrency,
			DifferentialPackagePath: opt.DifferentialPackagePath,
			Flavor:                  opt.Flavor,
			RegistryOverrides:       opt.RegistryOverrides,
			SigningKeyPath:          opt.SigningKeyPath,
			SigningKeyPassword:      opt.SigningKeyPassword,
		},
		SetVariables: opt.SetVariables,
	}
	pkgLayout, err := layout2.CreatePackage(ctx, packagePath, createOpt)
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
		remote, err := layout2.NewRemote(ctx, ref, oci.PlatformForArch(pkgLayout.Pkg.Build.Architecture))
		if err != nil {
			return err
		}
		err = remote.Push(ctx, pkgLayout, config.CommonOptions.OCIConcurrency)
		if err != nil {
			return err
		}
	} else {
		err = pkgLayout.Archive(ctx, output, opt.MaxPackageSizeMB)
		if err != nil {
			return err
		}
	}

	if opt.SBOMOut != "" {
		err := pkgLayout.GetSBOM(ctx, filepath.Join(opt.SBOMOut, pkgLayout.Pkg.Metadata.Name))
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
