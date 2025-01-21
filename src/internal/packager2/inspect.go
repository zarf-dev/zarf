// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager2 contains functions for inspecting packages.
package packager2

import (
	"context"
	"fmt"
	"os"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/packager/sbom"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/utils"
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
		err = handleSBOMOptions(ctx, opt)
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
	for _, component := range pkg.Components {
		imageList = append(imageList, component.Images...)
	}
	if imageList == nil {
		return nil, fmt.Errorf("failed listing images: 0 images found in package")
	}
	imageList = helpers.Unique(imageList)
	return imageList, nil
}

func getPackageMetadata(ctx context.Context, opt ZarfInspectOptions) (v1alpha1.ZarfPackage, error) {
	pkg, err := GetPackageFromSourceOrCluster(ctx, opt.Cluster, opt.Source, opt.SkipSignatureValidation, opt.PublicKeyPath)
	if err != nil {
		return pkg, err
	}

	return pkg, nil
}

func handleSBOMOptions(ctx context.Context, opt ZarfInspectOptions) error {
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

	sbomDirPath := opt.SBOMOutputDir
	if sbomDirPath == "" {
		tmpDir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)
		sbomDirPath = tmpDir
	}
	sbomPath, err := layout.GetSBOM(sbomDirPath)
	if err != nil {
		return err
	}
	if opt.ViewSBOM {
		err := sbom.ViewSBOMFiles(ctx, sbomPath)
		if err != nil {
			return err
		}
	}
	return nil
}

func getSBOM(viewSBOM bool, SBOMOutputDir string) bool {
	if viewSBOM || SBOMOutputDir != "" {
		return true
	}
	return false
}
