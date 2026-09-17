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
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/config"
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
// FIXME: Should still only generate a v1alpha1 package for now,
func Generate(ctx context.Context, packageName, url, version string, opts GenerateOptions) (pkg api.Package, err error) {
	if packageName == "" {
		return api.Package{}, fmt.Errorf("must provide a package name")
	}
	if url == "" {
		return api.Package{}, fmt.Errorf("must provide a URL")
	}
	if version == "" {
		return api.Package{}, fmt.Errorf("must provide a version")
	}
	l := logger.From(ctx)
	chart := api.Chart{Name: packageName, Version: version, Namespace: packageName}
	if opts.GitPath != "" {
		chart.Git = &api.GitSource{URL: url, Path: opts.GitPath}
	} else {
		chart.HelmRepository = &api.HelmRepositorySource{URL: url}
	}
	pkg = api.Package{
		Kind: api.ZarfPackageConfig,
		Metadata: api.PackageMetadata{
			Name:        packageName,
			Version:     version,
			Description: "auto-generated using `zarf dev generate`",
		},
		Components: []api.Component{
			{Name: packageName, Charts: []api.Chart{chart}},
		},
	}
	tmpGeneratePath, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return api.Package{}, err
	}
	defer func(path string) {
		errRemove := os.RemoveAll(path)
		err = errors.Join(err, errRemove)
	}(tmpGeneratePath)
	b, err := goyaml.MarshalWithOptions(pkg)
	if err != nil {
		return api.Package{}, err
	}
	if err := os.WriteFile(filepath.Join(tmpGeneratePath, layout.ZarfYAML), b, helpers.ReadAllWriteUser); err != nil {
		return api.Package{}, err
	}
	imagesScans, err := FindImages(ctx, tmpGeneratePath, FindImagesOptions{
		KubeVersionOverride: opts.KubeVersion,
		IsInteractive:       false,
	})
	if err != nil {
		// purposefully not returning error here, as we can still generate the package without images
		l.Error("failed to find images", "error", err.Error())
	}
	for i, imageScan := range imagesScans {
		for _, image := range append(append(imageScan.Matches, imageScan.PotentialMatches...), imageScan.CosignArtifacts...) {
			pkg.Components[i].Images = append(pkg.Components[i].Images, api.Image{Name: image})
		}
	}
	return pkg, nil
}
