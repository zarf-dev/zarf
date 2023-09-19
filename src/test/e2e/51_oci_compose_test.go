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

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/transform"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/exec"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"oras.land/oras-go/v2/registry"
)

type SkeletonSuite struct {
	suite.Suite
	*require.Assertions
	Reference registry.Reference
}

var (
	composeExample       = filepath.Join("examples", "composable-packages")
	composeExamplePath   string
	importEverything     = filepath.Join("src", "test", "packages", "51-import-everything")
	importEverythingPath string
	importception        = filepath.Join("src", "test", "packages", "51-import-everything", "inception")
	importceptionPath    string
	everythingExternal   = filepath.Join("src", "test", "packages", "everything-external")
	absNoCode            = filepath.Join("/", "tmp", "nocode")
)

func (suite *SkeletonSuite) SetupSuite() {
	suite.Assertions = require.New(suite.T())
	err := os.MkdirAll(filepath.Join("src", "test", "packages", "51-import-everything", "charts"), 0755)
	suite.NoError(err)
	err = utils.CreatePathAndCopy(filepath.Join("examples", "helm-charts", "chart"), filepath.Join("src", "test", "packages", "51-import-everything", "charts", "local"))
	suite.NoError(err)
	suite.DirExists(filepath.Join("src", "test", "packages", "51-import-everything", "charts", "local"))

	err = utils.CreatePathAndCopy(importEverything, everythingExternal)
	suite.NoError(err)
	suite.DirExists(everythingExternal)

	err = exec.CmdWithPrint("git", "clone", "https://github.com/kelseyhightower/nocode", absNoCode)
	suite.NoError(err)
	suite.DirExists(absNoCode)

	e2e.SetupDockerRegistry(suite.T(), 555)
	suite.Reference.Registry = "localhost:555"

	// Setup the package paths after e2e has been initialized
	composeExamplePath = filepath.Join("build", fmt.Sprintf("zarf-package-composable-packages-%s.tar.zst", e2e.Arch))
	importEverythingPath = filepath.Join("build", fmt.Sprintf("zarf-package-import-everything-%s-0.0.1.tar.zst", e2e.Arch))
	importceptionPath = filepath.Join("build", fmt.Sprintf("zarf-package-importception-%s-0.0.1.tar.zst", e2e.Arch))
}

func (suite *SkeletonSuite) TearDownSuite() {
	e2e.TeardownRegistry(suite.T(), 555)
	err := os.RemoveAll(everythingExternal)
	suite.NoError(err)
	err = os.RemoveAll(absNoCode)
	suite.NoError(err)
	err = os.RemoveAll(filepath.Join("src", "test", "packages", "51-import-everything", "charts", "local"))
	suite.NoError(err)
	err = os.RemoveAll("files")
	suite.NoError(err)
	err = os.RemoveAll(composeExamplePath)
	suite.NoError(err)
	err = os.RemoveAll(importEverythingPath)
	suite.NoError(err)
	err = os.RemoveAll(importceptionPath)
	suite.NoError(err)
}

func (suite *SkeletonSuite) Test_0_Publish_Skeletons() {
	suite.T().Log("E2E: Skeleton Package Publish oci://")
	ref := suite.Reference.String()

	wordpress := filepath.Join("examples", "wordpress")
	_, stdErr, err := e2e.Zarf("package", "publish", wordpress, "oci://"+ref, "--insecure")
	suite.NoError(err)
	suite.Contains(stdErr, "Published "+ref)

	helmCharts := filepath.Join("examples", "helm-charts")
	_, stdErr, err = e2e.Zarf("package", "publish", helmCharts, "oci://"+ref, "--insecure")
	suite.NoError(err)
	suite.Contains(stdErr, "Published "+ref)

	bigBang := filepath.Join("examples", "big-bang")
	_, stdErr, err = e2e.Zarf("package", "publish", bigBang, "oci://"+ref, "--insecure")
	suite.NoError(err)
	suite.Contains(stdErr, "Published "+ref)

	_, stdErr, err = e2e.Zarf("package", "publish", importEverything, "oci://"+ref, "--insecure")
	suite.NoError(err)
	suite.Contains(stdErr, "Published "+ref)

	_, _, err = e2e.Zarf("package", "inspect", "oci://"+ref+"/import-everything:0.0.1-skeleton", "--insecure")
	suite.NoError(err)

	_, _, err = e2e.Zarf("package", "pull", "oci://"+ref+"/import-everything:0.0.1-skeleton", "-o", "build", "--insecure")
	suite.NoError(err)

	_, _, err = e2e.Zarf("package", "pull", "oci://"+ref+"/helm-charts:0.0.1-skeleton", "-o", "build", "--insecure")
	suite.NoError(err)

	_, _, err = e2e.Zarf("package", "pull", "oci://"+ref+"/big-bang-example:2.10.0-skeleton", "-o", "build", "--insecure")
	suite.NoError(err)
}

func (suite *SkeletonSuite) Test_1_Compose_Example() {
	suite.T().Log("E2E: Skeleton Package Compose oci://")

	_, stdErr, err := e2e.Zarf("package", "create", composeExample, "-o", "build", "--insecure", "--no-color", "--confirm")
	suite.NoError(err)

	// Ensure that common names merge
	suite.Contains(stdErr, `
  manifests:
  - name: multi-games
    namespace: dos-games
    files:
    - ../dos-games/manifests/deployment.yaml
    - ../dos-games/manifests/service.yaml
    - quake-service.yaml`)

	// Ensure that the action was appended
	suite.Contains(stdErr, `
  - docker.io/bitnami/wordpress:6.2.0-debian-11-r18
  actions:
    onDeploy:
      before:
      - cmd: ./zarf tools kubectl get -n dos-games deployment -o jsonpath={.items[0].metadata.creationTimestamp}
        setVariables:
        - name: WORDPRESS_BLOG_NAME`)

	// Ensure that the variables were merged
	suite.Contains(stdErr, `
- name: WORDPRESS_BLOG_NAME
  description: The blog name that is used for the WordPress admin account
  default: The Zarf Blog
  prompt: true`)
}

func (suite *SkeletonSuite) Test_2_Compose_Everything_Inception() {
	suite.T().Log("E2E: Skeleton Package Compose oci://")

	_, _, err := e2e.Zarf("package", "create", importEverything, "-o", "build", "--insecure", "--confirm")
	suite.NoError(err)

	_, _, err = e2e.Zarf("package", "create", importception, "-o", "build", "--insecure", "--confirm")
	suite.NoError(err)

	_, stdErr, err := e2e.Zarf("package", "inspect", importEverythingPath)
	suite.NoError(err)

	targets := []string{
		"import-component-local == import-component-local",
		"import-component-local-relative == import-component-local-relative",
		"import-component-wordpress == import-component-wordpress",
		"import-component-oci == import-component-oci",
		"file-imports == file-imports",
		"import-helm-local == import-helm-local",
		"import-helm-local-relative == import-helm-local-relative",
		"import-helm-oci == import-helm-oci",
		"import-repos == import-repos",
		"import-images == import-images",
	}

	for _, target := range targets {
		suite.Contains(stdErr, target)
	}
}

func (suite *SkeletonSuite) Test_3_FilePaths() {
	suite.T().Log("E2E: Skeleton Package File Paths")

	pkgTars := []string{
		filepath.Join("build", fmt.Sprintf("zarf-package-import-everything-%s-0.0.1.tar.zst", e2e.Arch)),
		filepath.Join("build", "zarf-package-import-everything-skeleton-0.0.1.tar.zst"),
		filepath.Join("build", fmt.Sprintf("zarf-package-importception-%s-0.0.1.tar.zst", e2e.Arch)),
		filepath.Join("build", "zarf-package-helm-charts-skeleton-0.0.1.tar.zst"),
		filepath.Join("build", "zarf-package-big-bang-example-skeleton-2.10.0.tar.zst"),
	}

	for _, pkgTar := range pkgTars {
		var pkg types.ZarfPackage

		unpacked := strings.TrimSuffix(pkgTar, ".tar.zst")
		defer os.RemoveAll(unpacked)
		defer os.RemoveAll(pkgTar)
		_, _, err := e2e.Zarf("tools", "archiver", "decompress", pkgTar, unpacked, "--unarchive-all")
		suite.NoError(err)
		suite.DirExists(unpacked)

		err = utils.ReadYaml(filepath.Join(unpacked, config.ZarfYAML), &pkg)
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
	invalid := utils.InvalidPath(path)
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
		componentPaths := types.ComponentPaths{
			Base:           base,
			Temp:           filepath.Join(base, types.TempFolder),
			Files:          filepath.Join(base, types.FilesFolder),
			Charts:         filepath.Join(base, types.ChartsFolder),
			Repos:          filepath.Join(base, types.ReposFolder),
			Manifests:      filepath.Join(base, types.ManifestsFolder),
			DataInjections: filepath.Join(base, types.DataInjectionsFolder),
			Values:         filepath.Join(base, types.ValuesFolder),
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
