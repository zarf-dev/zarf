// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager2 contains functions for inspecting packages.
package packager2

import (
	"context"
	"fmt"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/packager/sbom"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
)

// ZarfInspectOptions tracks the user-defined preferences during a package inspection.
type ZarfInspectOptions struct {
	Source                  string
	Cluster                 *cluster.Cluster
	ViewSBOM                bool
	SBOMOutputDir           string
	ListImages              bool
	SkipSignatureValidation bool
	PublicKeyPath           string
}

// Inspect list the contents of a package.
func Inspect(ctx context.Context, opt ZarfInspectOptions) (v1alpha1.ZarfPackage, error) {
	var err error
	pkg, err := getPackageMetadata(ctx, opt)
	if err != nil {
		return pkg, err
	}

	if getSBOM(opt.ViewSBOM, opt.SBOMOutputDir) {
		err = handleSBOMOptions(ctx, pkg, opt)
		if err != nil {
			return pkg, err
		}
		return pkg, nil
	}
	return pkg, nil
}

// InspectList lists the images in a component action
func InspectList(ctx context.Context, opt ZarfInspectOptions) ([]string, error) {
	var imageList []string
	pkg, err := getPackageMetadata(ctx, opt)
	if err != nil {
		return nil, err
	}
	// Only list images if we have have components
	if len(pkg.Components) > 0 {
		for _, component := range pkg.Components {
			imageList = append(imageList, component.Images...)
		}
		if len(imageList) > 0 {
			imageList = helpers.Unique(imageList)
			return imageList, nil
		}
		return nil, fmt.Errorf("failed listing images: list of images found in components: %d", len(imageList))
	}

	return imageList, err
}

func getPackageMetadata(ctx context.Context, opt ZarfInspectOptions) (v1alpha1.ZarfPackage, error) {
	pkg, err := packageFromSourceOrCluster(ctx, opt.Cluster, opt.Source, opt.SkipSignatureValidation, opt.PublicKeyPath)
	if err != nil {
		return pkg, err
	}

	return pkg, nil
}

func handleSBOMOptions(ctx context.Context, pkg v1alpha1.ZarfPackage, opt ZarfInspectOptions) error {
	loadOpt := LoadOptions{
		Source:                  opt.Source,
		SkipSignatureValidation: opt.SkipSignatureValidation,
		Filter:                  filters.Empty(),
		PublicKeyPath:           opt.PublicKeyPath,
	}
	layout, err := LoadPackage(ctx, loadOpt)
	if err != nil {
		return err
	}
	if opt.SBOMOutputDir != "" {
		out, err := layout.SBOMs.OutputSBOMFiles(opt.SBOMOutputDir, pkg.Metadata.Name)
		if err != nil {
			return err
		}
		if opt.ViewSBOM {
			err := sbom.ViewSBOMFiles(out)
			if err != nil {
				return err
			}
		}
	} else if opt.ViewSBOM {
		err := sbom.ViewSBOMFiles(layout.SBOMs.Path)
		if err != nil {
			return err
		}
		return err
	}
	return nil
}

func getSBOM(viewSBOM bool, SBOMOutputDir string) bool {
	if viewSBOM || SBOMOutputDir != "" {
		return true
	}
	return false
}
