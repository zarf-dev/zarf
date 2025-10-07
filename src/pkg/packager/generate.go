// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/pkg/helpers/v2"
	goyaml "github.com/goccy/go-yaml"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

// GenerateOptions are the options for generating a Zarf package.
type GenerateOptions struct {
	// Path to the Helm chart in the git repository
	GitPath string
	// Kube version to provide to the Helm chart
	KubeVersion string
}

// Generate a Zarf package definition using information about a Helm chart.
func Generate(ctx context.Context, packageName, url, version string, opts GenerateOptions) (pkg v1alpha1.ZarfPackage, err error) {
	if packageName == "" {
		return v1alpha1.ZarfPackage{}, fmt.Errorf("must provide a package name")
	}
	if url == "" {
		return v1alpha1.ZarfPackage{}, fmt.Errorf("must provide a URL")
	}
	if version == "" {
		return v1alpha1.ZarfPackage{}, fmt.Errorf("must provide a version")
	}
	l := logger.From(ctx)
	generatedComponent := v1alpha1.ZarfComponent{
		Name:     packageName,
		Required: helpers.BoolPtr(true),
		Charts: []v1alpha1.ZarfChart{
			{
				Name:      packageName,
				Version:   version,
				Namespace: packageName,
				URL:       url,
				GitPath:   opts.GitPath,
			},
		},
	}

	pkg = v1alpha1.ZarfPackage{
		Kind: v1alpha1.ZarfPackageConfig,
		Metadata: v1alpha1.ZarfMetadata{
			Name:        packageName,
			Version:     version,
			Description: "auto-generated using `zarf dev generate`",
		},
		Components: []v1alpha1.ZarfComponent{
			generatedComponent,
		},
	}
	tmpGeneratePath, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	defer func(path string) {
		errRemove := os.RemoveAll(path)
		err = errors.Join(err, errRemove)
	}(tmpGeneratePath)
	b, err := goyaml.MarshalWithOptions(pkg)
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	if err := os.WriteFile(filepath.Join(tmpGeneratePath, layout.ZarfYAML), b, helpers.ReadAllWriteUser); err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	imagesScans, err := FindImages(ctx, tmpGeneratePath, FindImagesOptions{
		KubeVersionOverride: opts.KubeVersion,
	})
	if err != nil {
		// purposefully not returning error here, as we can still generate the package without images
		l.Error("failed to find images", "error", err.Error())
	}
	for i, imageScan := range imagesScans {
		pkg.Components[i].Images = append(pkg.Components[i].Images, imageScan.Matches...)
		pkg.Components[i].Images = append(pkg.Components[i].Images, imageScan.PotentialMatches...)
		pkg.Components[i].Images = append(pkg.Components[i].Images, imageScan.CosignArtifacts...)
	}

	if err := lint.ValidatePackage(pkg); err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	return pkg, nil
}
