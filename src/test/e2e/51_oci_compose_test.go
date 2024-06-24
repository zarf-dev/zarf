// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	"oras.land/oras-go/v2/registry"
)

type SkeletonSuite struct {
	suite.Suite
	*require.Assertions
	Reference registry.Reference
}

var (
	importEverything     = filepath.Join("src", "test", "packages", "51-import-everything")
	importEverythingPath string
	importception        = filepath.Join("src", "test", "packages", "51-import-everything", "inception")
	importceptionPath    string
)

func (suite *SkeletonSuite) SetupSuite() {
	suite.Assertions = require.New(suite.T())

	e2e.SetupDockerRegistry(suite.T(), 555)
	suite.Reference.Registry = "localhost:555"

	// Setup the package paths after e2e has been initialized
	importEverythingPath = filepath.Join("build", fmt.Sprintf("zarf-package-import-everything-%s-0.0.1.tar.zst", e2e.Arch))
	importceptionPath = filepath.Join("build", fmt.Sprintf("zarf-package-importception-%s-0.0.1.tar.zst", e2e.Arch))
}

func (suite *SkeletonSuite) TearDownSuite() {
	e2e.TeardownRegistry(suite.T(), 555)

	err := os.RemoveAll(filepath.Join("src", "test", "packages", "51-import-everything", "charts", "local"))
	suite.NoError(err)
	err = os.RemoveAll(importEverythingPath)
	suite.NoError(err)
	err = os.RemoveAll(importceptionPath)
	suite.NoError(err)
}

func (suite *SkeletonSuite) Test_0_Publish_Skeletons() {
	suite.T().Log("E2E: Skeleton Package Publish oci://")
	ref := suite.Reference.String()

	helmCharts := filepath.Join("examples", "helm-charts")
	_, stdErr, err := e2e.Zarf("package", "publish", helmCharts, "oci://"+ref, "--insecure")
	suite.NoError(err)
	suite.Contains(stdErr, "Published "+ref)

	bigBang := filepath.Join("src", "test", "packages", "51-import-everything", "big-bang-min")
	_, stdErr, err = e2e.Zarf("package", "publish", bigBang, "oci://"+ref, "--insecure")
	suite.NoError(err)
	suite.Contains(stdErr, "Published "+ref)

	composable := filepath.Join("src", "test", "packages", "09-composable-packages")
	_, stdErr, err = e2e.Zarf("package", "publish", composable, "oci://"+ref, "--insecure")
	suite.NoError(err)
	suite.Contains(stdErr, "Published "+ref)

	_, stdErr, err = e2e.Zarf("package", "publish", importEverything, "oci://"+ref, "--insecure")
	suite.NoError(err)
	suite.Contains(stdErr, "Published "+ref)

	_, _, err = e2e.Zarf("package", "inspect", "oci://"+ref+"/import-everything:0.0.1", "--insecure", "-a", "skeleton")
	suite.NoError(err)

	_, _, err = e2e.Zarf("package", "pull", "oci://"+ref+"/import-everything:0.0.1", "-o", "build", "--insecure", "-a", "skeleton")
	suite.NoError(err)

	_, _, err = e2e.Zarf("package", "pull", "oci://"+ref+"/helm-charts:0.0.1", "-o", "build", "--insecure", "-a", "skeleton")
	suite.NoError(err)

	_, _, err = e2e.Zarf("package", "pull", "oci://"+ref+"/big-bang-min:2.10.0", "-o", "build", "--insecure", "-a", "skeleton")
	suite.NoError(err)

	_, _, err = e2e.Zarf("package", "pull", "oci://"+ref+"/test-compose-package:0.0.1", "-o", "build", "--insecure", "-a", "skeleton")
	suite.NoError(err)
}

func (suite *SkeletonSuite) Test_1_Compose_Everything_Inception() {
	suite.T().Log("E2E: Skeleton Package Compose oci://")

	_, _, err := e2e.Zarf("package", "create", importEverything, "-o", "build", "--insecure", "--confirm")
	suite.NoError(err)

	_, _, err = e2e.Zarf("package", "create", importception, "-o", "build", "--insecure", "--confirm")
	suite.NoError(err)

	_, stdErr, err := e2e.Zarf("package", "inspect", importEverythingPath)
	suite.NoError(err)

	targets := []string{
		"import-component-local == import-component-local",
		"import-component-oci == import-component-oci",
		"import-big-bang == import-big-bang",
		"file-imports == file-imports",
		"local-chart-import == local-chart-import",
	}

	for _, target := range targets {
		suite.Contains(stdErr, target)
	}
}

func (suite *SkeletonSuite) Test_2_FilePaths() {
	suite.T().Log("E2E: Skeleton + Package File Paths")

	pkgTars := []string{
		filepath.Join("build", fmt.Sprintf("zarf-package-import-everything-%s-0.0.1.tar.zst", e2e.Arch)),
		filepath.Join("build", "zarf-package-import-everything-skeleton-0.0.1.tar.zst"),
		filepath.Join("build", fmt.Sprintf("zarf-package-importception-%s-0.0.1.tar.zst", e2e.Arch)),
		filepath.Join("build", "zarf-package-helm-charts-skeleton-0.0.1.tar.zst"),
		filepath.Join("build", "zarf-package-big-bang-min-skeleton-2.10.0.tar.zst"),
		filepath.Join("build", "zarf-package-test-compose-package-skeleton-0.0.1.tar.zst"),
	}

	for _, pkgTar := range pkgTars {
		var pkg types.ZarfPackage

		unpacked := strings.TrimSuffix(pkgTar, ".tar.zst")
		defer os.RemoveAll(unpacked)
		defer os.RemoveAll(pkgTar)
		_, _, err := e2e.Zarf("tools", "archiver", "decompress", pkgTar, unpacked, "--unarchive-all")
		suite.NoError(err)
		suite.DirExists(unpacked)

		// Verify skeleton contains kustomize-generated manifests.
		if strings.HasSuffix(pkgTar, "zarf-package-test-compose-package-skeleton-0.0.1.tar.zst") {
			kustomizeGeneratedManifests := []string{
				"kustomization-connect-service-0.yaml",
				"kustomization-connect-service-1.yaml",
				"kustomization-connect-service-two-0.yaml",
			}
			manifestDir := filepath.Join(unpacked, "components", "test-compose-package", "manifests")
			for _, manifest := range kustomizeGeneratedManifests {
				manifestPath := filepath.Join(manifestDir, manifest)
				suite.FileExists(manifestPath, "expected to find kustomize-generated manifest: %q", manifestPath)
				var configMap corev1.ConfigMap
				err := utils.ReadYaml(manifestPath, &configMap)
				suite.NoError(err)
				suite.Equal("ConfigMap", configMap.Kind, "expected manifest %q to be of kind ConfigMap", manifestPath)
			}
		}

		err = utils.ReadYaml(filepath.Join(unpacked, layout.ZarfYAML), &pkg)
		suite.NoError(err)
		suite.NotNil(pkg)

		components := pkg.Components
		suite.NotNil(components)

		isSkeleton := false
		if strings.Contains(pkgTar, "-skeleton-") {
			isSkeleton = true
		}
		suite.verifyComponentPaths(unpacked, components, isSkeleton)
	}
}

func (suite *SkeletonSuite) DirOrFileExists(path string) {
	suite.T().Helper()

	invalid := helpers.InvalidPath(path)
	suite.Falsef(invalid, "path specified does not exist: %s", path)
}

func (suite *SkeletonSuite) verifyComponentPaths(unpackedPath string, components []types.ZarfComponent, isSkeleton bool) {

	if isSkeleton {
		suite.NoDirExists(filepath.Join(unpackedPath, "images"))
		suite.NoDirExists(filepath.Join(unpackedPath, "sboms"))
	}

	for _, component := range components {
		if len(component.Charts) == 0 && len(component.Files) == 0 && len(component.Manifests) == 0 && len(component.DataInjections) == 0 && len(component.Repos) == 0 {
			// component has no files to check
			continue
		}

		base := filepath.Join(unpackedPath, "components", component.Name)
		componentPaths := layout.ComponentPaths{
			Base:           base,
			Temp:           filepath.Join(base, layout.TempDir),
			Files:          filepath.Join(base, layout.FilesDir),
			Charts:         filepath.Join(base, layout.ChartsDir),
			Repos:          filepath.Join(base, layout.ReposDir),
			Manifests:      filepath.Join(base, layout.ManifestsDir),
			DataInjections: filepath.Join(base, layout.DataInjectionsDir),
			Values:         filepath.Join(base, layout.ValuesDir),
		}

		if isSkeleton && component.DeprecatedCosignKeyPath != "" {
			suite.FileExists(filepath.Join(base, component.DeprecatedCosignKeyPath))
		}

		if isSkeleton && component.Extensions.BigBang != nil {
			for _, valuesFile := range component.Extensions.BigBang.ValuesFiles {
				suite.FileExists(filepath.Join(base, valuesFile))
			}
		}

		for chartIdx, chart := range component.Charts {
			if isSkeleton && chart.URL != "" {
				continue
			} else if isSkeleton {
				dir := fmt.Sprintf("%s-%d", chart.Name, chartIdx)
				suite.DirExists(filepath.Join(componentPaths.Charts, dir))
				continue
			}
			tgz := fmt.Sprintf("%s-%s.tgz", chart.Name, chart.Version)
			suite.FileExists(filepath.Join(componentPaths.Charts, tgz))
		}

		for filesIdx, file := range component.Files {
			if isSkeleton && helpers.IsURL(file.Source) {
				continue
			} else if isSkeleton {
				suite.FileExists(filepath.Join(base, file.Source))
				continue
			}
			path := filepath.Join(componentPaths.Files, strconv.Itoa(filesIdx), filepath.Base(file.Target))
			suite.DirOrFileExists(path)
		}

		for dataIdx, data := range component.DataInjections {
			if isSkeleton && helpers.IsURL(data.Source) {
				continue
			} else if isSkeleton {
				suite.DirOrFileExists(filepath.Join(base, data.Source))
				continue
			}
			path := filepath.Join(componentPaths.DataInjections, strconv.Itoa(dataIdx), filepath.Base(data.Target.Path))
			suite.DirOrFileExists(path)
		}

		for _, manifest := range component.Manifests {
			if isSkeleton {
				suite.Nil(manifest.Kustomizations)
			}
			for filesIdx, path := range manifest.Files {
				if isSkeleton && helpers.IsURL(path) {
					continue
				} else if isSkeleton {
					suite.FileExists(filepath.Join(base, path))
					continue
				}
				suite.FileExists(filepath.Join(componentPaths.Manifests, fmt.Sprintf("%s-%d.yaml", manifest.Name, filesIdx)))
			}
			for kustomizeIdx := range manifest.Kustomizations {
				path := filepath.Join(componentPaths.Manifests, fmt.Sprintf("kustomization-%s-%d.yaml", manifest.Name, kustomizeIdx))
				suite.FileExists(path)
			}
		}

		if !isSkeleton {
			for _, repo := range component.Repos {
				dir, err := transform.GitURLtoFolderName(repo)
				suite.NoError(err)
				suite.DirExists(filepath.Join(componentPaths.Repos, dir))
			}
		}
	}

}

func TestSkeletonSuite(t *testing.T) {
	e2e.SetupWithCluster(t)

	suite.Run(t, new(SkeletonSuite))
}
