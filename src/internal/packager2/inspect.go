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
	"github.com/zarf-dev/zarf/src/pkg/layout"
)

// ZarfInspectOptions tracks the user-defined preferences during a package inspection.
type ZarfInspectOptions struct {
	PackagePaths  *layout.PackagePaths
	Cluster       *cluster.Cluster
	ViewSBOM      bool
	SBOMOutputDir string
	ListImages    bool
}

// Inspect list the contents of a package.
func Inspect(ctx context.Context, options ZarfInspectOptions) (v1alpha1.ZarfPackage, error) {
	var err error
	pkg, err := getPackageMetadata(ctx, options.PackagePaths)
	if err != nil {
		return pkg, err
	}

	if getSBOM(options.ViewSBOM, options.SBOMOutputDir) {
		err = handleSBOMOptions(options.PackagePaths, pkg, options.ViewSBOM, options.SBOMOutputDir)
		if err != nil {
			return pkg, err
		}
		return pkg, nil
	}
	return pkg, nil
}

// InspectList lists the images in a component action
func InspectList(ctx context.Context, options ZarfInspectOptions) ([]string, error) {
	var imageList []string
	pkg, err := getPackageMetadata(ctx, options.PackagePaths)
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

func getPackageMetadata(_ context.Context, layout *layout.PackagePaths) (v1alpha1.ZarfPackage, error) {
	pkg, _, err := layout.ReadZarfYAML()
	if err != nil {
		return pkg, err
	}
	return pkg, nil
}

func handleSBOMOptions(layout *layout.PackagePaths, pkg v1alpha1.ZarfPackage, viewSBOM bool, SBOMOutputDir string) error {
	if SBOMOutputDir != "" {
		out, err := layout.SBOMs.OutputSBOMFiles(SBOMOutputDir, pkg.Metadata.Name)
		if err != nil {
			return err
		}
		if viewSBOM {
			err := sbom.ViewSBOMFiles(out)
			if err != nil {
				return err
			}
		}
	} else if viewSBOM {
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
