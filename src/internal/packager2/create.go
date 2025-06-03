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
	"github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/internal/packager2/load"
	"github.com/zarf-dev/zarf/src/pkg/logger"
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
	Output                  string
	DifferentialPackagePath string
	OCIConcurrency          int
}

// Create takes a path to a directory containing a ZarfPackageConfig and produces an archived Zarf package
func Create(ctx context.Context, packagePath string, opt CreateOptions) (err error) {
	if opt.SkipSBOM && opt.SBOMOut != "" {
		return fmt.Errorf("cannot skip SBOM creation and specify an SBOM output directory")
	}

	loadOpts := load.DefinitionOpts{
		Flavor:       opt.Flavor,
		SetVariables: opt.SetVariables,
	}
	pkg, err := load.PackageDefinition(ctx, packagePath, loadOpts)
	if err != nil {
		return err
	}

	assembleOpt := layout.AssembleOptions{
		SkipSBOM:                opt.SkipSBOM,
		OCIConcurrency:          opt.OCIConcurrency,
		DifferentialPackagePath: opt.DifferentialPackagePath,
		Flavor:                  opt.Flavor,
		RegistryOverrides:       opt.RegistryOverrides,
		SigningKeyPath:          opt.SigningKeyPath,
		SigningKeyPassword:      opt.SigningKeyPassword,
	}
	pkgLayout, err := layout.AssemblePackage(ctx, pkg, packagePath, assembleOpt)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, pkgLayout.Cleanup())
	}()

	if helpers.IsOCIURL(opt.Output) {
		ref, err := zoci.ReferenceFromMetadata(opt.Output, pkgLayout.Pkg)
		if err != nil {
			return err
		}
		remote, err := zoci.NewRemote(ctx, ref, oci.PlatformForArch(pkgLayout.Pkg.Build.Architecture))
		if err != nil {
			return err
		}
		err = remote.PushPackage(ctx, pkgLayout, config.CommonOptions.OCIConcurrency)
		if err != nil {
			return err
		}
	} else {
		err = pkgLayout.Archive(ctx, opt.Output, opt.MaxPackageSizeMB)
		if err != nil {
			return err
		}
	}

	if opt.SBOMOut != "" {
		err := pkgLayout.GetSBOM(ctx, filepath.Join(opt.SBOMOut, pkgLayout.Pkg.Metadata.Name))
		// Don't fail package create if the package doesn't have an sbom
		var noSBOMErr *layout.NoSBOMAvailableError
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
