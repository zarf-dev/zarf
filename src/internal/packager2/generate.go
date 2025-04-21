// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/pkg/helpers/v2"
	goyaml "github.com/goccy/go-yaml"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/lint"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

// GenerateOptions are the options for generating a Zarf package.
type GenerateOptions struct {
	// Name of the package
	PackageName string
	// Version of the Helm chart
	Version string
	// URL to the Helm chart
	URL string
	// Path to the Helm chart in the git repository
	GitPath string
	// Kube version to provide to the Helm chart
	KubeVersion string
}

// Generate a Zarf package definition using information about a Helm chart.
func Generate(ctx context.Context, opts *GenerateOptions) (pkg v1alpha1.ZarfPackage, err error) {
	l := logger.From(ctx)
	generatedComponent := v1alpha1.ZarfComponent{
		Name:     opts.PackageName,
		Required: helpers.BoolPtr(true),
		Charts: []v1alpha1.ZarfChart{
			{
				Name:      opts.PackageName,
				Version:   opts.Version,
				Namespace: opts.PackageName,
				URL:       opts.URL,
				GitPath:   opts.GitPath,
			},
		},
	}

	pkg = v1alpha1.ZarfPackage{
		Kind: v1alpha1.ZarfPackageConfig,
		Metadata: v1alpha1.ZarfMetadata{
			Name:        opts.PackageName,
			Version:     opts.Version,
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
	results, err := FindImages(ctx, tmpGeneratePath, FindImagesOptions{
		KubeVersionOverride: opts.KubeVersion,
	})
	if err != nil {
		// purposefully not returning error here, as we can still generate the package without images
		l.Error("failed to find images", "error", err.Error())
	}
	for i, component := range results.ComponentImageScans {
		pkg.Components[i].Images = append(pkg.Components[i].Images, component.Matches...)
		pkg.Components[i].Images = append(pkg.Components[i].Images, component.PotentialMatches...)
		pkg.Components[i].Images = append(pkg.Components[i].Images, component.CosignArtifacts...)
	}

	if err := lint.ValidatePackage(pkg); err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	return pkg, nil
}
