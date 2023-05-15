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
	"github.com/defenseunicorns/zarf/src/types"
	dconfig "github.com/docker/cli/cli/config"
	"github.com/stretchr/testify/suite"
	"oras.land/oras-go/v2/registry"
)

type SkeletonSuite struct {
	suite.Suite
	Remote    *utils.OrasRemote
	Reference registry.Reference
}

var (
	importEverything   = filepath.Join("src", "test", "test-packages", "51-import-everything")
	importception      = filepath.Join("src", "test", "test-packages", "51-import-everything", "inception")
	everythingExternal = filepath.Join("src", "test", "test-packages", "everything-external")
	absNoCode          = filepath.Join("/", "tmp", "nocode")
)

func (suite *SkeletonSuite) SetupSuite() {
	err := os.MkdirAll(filepath.Join("src", "test", "test-packages", "51-import-everything", "charts"), 0755)
	suite.NoError(err)
	err = utils.CreatePathAndCopy(filepath.Join("examples", "helm-local-chart", "chart"), filepath.Join("src", "test", "test-packages", "51-import-everything", "charts", "local"))
	suite.NoError(err)
	suite.DirExists(filepath.Join("src", "test", "test-packages", "51-import-everything", "charts", "local"))

	err = utils.CreatePathAndCopy(importEverything, everythingExternal)
	suite.NoError(err)
	suite.DirExists(everythingExternal)

	err = exec.CmdWithPrint("git", "clone", "https://github.com/kelseyhightower/nocode", absNoCode)
	suite.NoError(err)
	suite.DirExists(absNoCode)

	// TODO: (@RAZZLE) how do we want to handle this?
	image := "registry:2.8.2"

	// spin up a local registry
	err = exec.CmdWithPrint("docker", "run", "-d", "--restart=always", "-p", "555:5000", "--name", "registry", image)
	suite.NoError(err)

	// docker config folder
	cfg, err := dconfig.Load(dconfig.Dir())
	suite.NoError(err)
	if !cfg.ContainsAuth() {
		// make a docker config file w/ some blank creds
		_, _, err := e2e.ExecZarfCommand("tools", "registry", "login", "--username", "zarf", "-p", "zarf", "localhost:6000")
		suite.NoError(err)
	}

	suite.Reference.Registry = "localhost:555"

	// re-add gitea
	stdOut, stdErr, err := e2e.ExecZarfCommand("init", "--components=git-server", "--confirm")
	suite.NoError(err, stdOut, stdErr)
}

func (suite *SkeletonSuite) TearDownSuite() {
	_, _, err := exec.Cmd("docker", "rm", "-f", "registry")
	suite.NoError(err)
	err = os.RemoveAll(everythingExternal)
	suite.NoError(err)
	err = os.RemoveAll(absNoCode)
	suite.NoError(err)
	err = os.RemoveAll(filepath.Join("src", "test", "test-packages", "51-import-everything", "charts", "local"))
	suite.NoError(err)
	err = os.RemoveAll(filepath.Join("files"))
	suite.NoError(err)
	stdOut, stdErr, err := e2e.ExecZarfCommand("package", "remove", "init", "--components=git-server", "--confirm")
	suite.NoError(err, stdOut, stdErr)
}

func (suite *SkeletonSuite) Test_0_Publish_Skeletons() {
	suite.T().Log("E2E: Skeleton Package Publish oci://")

	helmLocal := filepath.Join("examples", "helm-local-chart")
	ref := suite.Reference.String()
	_, stdErr, err := e2e.ExecZarfCommand("package", "publish", helmLocal, "oci://"+ref, "--insecure")
	suite.NoError(err)
	suite.Contains(stdErr, "Published "+ref)

	_, stdErr, err = e2e.ExecZarfCommand("package", "publish", importEverything, "oci://"+ref, "--insecure")
	suite.NoError(err)
	suite.Contains(stdErr, "Published "+ref)

	_, _, err = e2e.ExecZarfCommand("package", "inspect", "oci://"+ref+"/import-everything:0.0.1-skeleton", "--insecure")
	suite.NoError(err)

	_, _, err = e2e.ExecZarfCommand("package", "pull", "oci://"+ref+"/helm-local-chart:0.0.1-skeleton", "--insecure")
	suite.NoError(err)
}

func (suite *SkeletonSuite) Test_1_Compose() {
	suite.T().Log("E2E: Skeleton Package Compose oci://")

	_, _, err := e2e.ExecZarfCommand("package", "create", importEverything, "--confirm", "-o", "build", "--insecure")
	suite.NoError(err)

	_, _, err = e2e.ExecZarfCommand("package", "create", importception, "--confirm", "-o", "build", "--insecure")
	suite.NoError(err)
}

func (suite *SkeletonSuite) Test_3_FilePaths() {
	suite.T().Log("E2E: Skeleton Package File Paths")

	localSkeletonPkgTar := "zarf-package-helm-local-chart-skeleton-0.0.1.tar.zst"

	pkgTars := []string{
		filepath.Join("build", fmt.Sprintf("zarf-package-import-everything-%s-0.0.1.tar.zst", e2e.Arch)),
		filepath.Join("build", fmt.Sprintf("zarf-package-importception-%s-0.0.1.tar.zst", e2e.Arch)),
		localSkeletonPkgTar,
	}

	for _, pkgTar := range pkgTars {
		var pkg types.ZarfPackage

		unpacked := strings.TrimSuffix(pkgTar, ".tar.zst")
		defer suite.NoError(os.RemoveAll(unpacked))
		_, _, err := e2e.ExecZarfCommand("tools", "archiver", "decompress", pkgTar, unpacked, "--unarchive-all")
		suite.NoError(err)
		suite.DirExists(unpacked)

		err = utils.ReadYaml(filepath.Join(unpacked, config.ZarfYAML), &pkg)
		suite.NoError(err)
		suite.NotNil(pkg)

		components := pkg.Components
		suite.NotNil(components)

		isSkeleton := false
		if pkgTar == localSkeletonPkgTar {
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
			Temp:           filepath.Join(base, "temp"),
			Files:          filepath.Join(base, "files"),
			Charts:         filepath.Join(base, "charts"),
			Repos:          filepath.Join(base, "repos"),
			Manifests:      filepath.Join(base, "manifests"),
			DataInjections: filepath.Join(base, "data"),
			Values:         filepath.Join(base, "values"),
		}

		suite.DirExists(filepath.Join(base, component.Name))
		if isSkeleton && component.CosignKeyPath != "" {
			suite.FileExists(filepath.Join(base, component.CosignKeyPath))
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
			if isSkeleton && utils.IsURL(file.Source) {
				continue
			} else if isSkeleton {
				suite.FileExists(filepath.Join(componentPaths.Files, file.Source))
				continue
			}
			suite.DirOrFileExists(filepath.Join(componentPaths.Files, strconv.Itoa(filesIdx)))
		}

		for dataIdx, data := range component.DataInjections {
			if isSkeleton && utils.IsURL(data.Source) {
				continue
			} else if isSkeleton {
				suite.DirOrFileExists(filepath.Join(componentPaths.DataInjections, data.Source))
				continue
			}
			path := filepath.Join(componentPaths.DataInjections, fmt.Sprintf("injection-%d", dataIdx))
			suite.DirOrFileExists(path)
		}

		for _, manifest := range component.Manifests {
			if isSkeleton {
				suite.Nil(manifest.Kustomizations)
			}
			for filesIdx, path := range manifest.Files {
				if isSkeleton && utils.IsURL(path) {
					continue
				} else if isSkeleton {
					suite.FileExists(filepath.Join(componentPaths.Manifests, path))
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
				dir, err := transform.GitTransformURLtoFolderName(repo)
				suite.NoError(err)
				suite.DirExists(filepath.Join(componentPaths.Repos, dir))
			}
		}
	}

}

func TestSkeletonSuite(t *testing.T) {
	e2e.SetupWithCluster(t)
	defer e2e.Teardown(t)
	suite.Run(t, new(SkeletonSuite))
}
