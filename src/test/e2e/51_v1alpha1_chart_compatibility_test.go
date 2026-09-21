// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/utils/exec"
	"github.com/zarf-dev/zarf/src/test/testutil"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/registry"
)

const legacyChartCompatibilityVersion = "v0.85.0"

// TestV1Alpha1ChartSourceCompatibility verifies that a package assembled by a
// released v1alpha1 CLI remains deployable by the current CLI. Each chart only
// creates a ConfigMap so source selection, package layout, and deployment are
// the variables under test.
func TestV1Alpha1ChartSourceCompatibility(t *testing.T) {
	t.Log("E2E: v1alpha1 chart source compatibility")

	packageDir := t.TempDir()
	localChartDir := filepath.Join(packageDir, "local-chart")
	writeLegacyConfigMapChart(t, localChartDir, "local-versionless", "local-versionless-config", "local-versionless")

	helmChartData := createLegacyChartArchive(t, "helm-source", "helm-renamed-config", "helm-renamed")
	helmRepositoryURL := serveLegacyHelmRepository(t, helmChartData, "helm-source", "0.1.0")

	registryAddress := testutil.SetupInMemoryRegistryDynamic(testutil.TestContext(t), t)
	registryClient, err := registry.NewClient(registry.ClientOptPlainHTTP())
	require.NoError(t, err)

	ociTagData := createLegacyChartArchive(t, "oci-tag-source", "oci-tag-config", "oci-tag")
	_, err = registryClient.Push(ociTagData, fmt.Sprintf("%s/charts/oci-tag-source:0.1.0", registryAddress))
	require.NoError(t, err)

	ociDigestData := createLegacyChartArchive(t, "oci-digest-source", "oci-digest-config", "oci-digest")
	ociDigest, err := registryClient.Push(ociDigestData, fmt.Sprintf("%s/charts/oci-digest-source:0.1.0", registryAddress))
	require.NoError(t, err)

	gitURL, gitCommit := createLegacyChartGitRepository(t)
	charts := []v1alpha1.ZarfChart{
		{Name: "local-versionless", Namespace: "legacy-local", LocalPath: "local-chart"},
		{Name: "helm-renamed", Namespace: "legacy-helm", Version: "0.1.0", URL: helmRepositoryURL, RepoName: "helm-source"},
		{Name: "oci-tag", Namespace: "legacy-oci-tag", Version: "0.1.0", URL: fmt.Sprintf("oci://%s/charts/oci-tag-source", registryAddress)},
		{Name: "oci-digest", Namespace: "legacy-oci-digest", Version: "digest-layout", URL: fmt.Sprintf("oci://%s/charts/oci-digest-source@%s", registryAddress, ociDigest.Manifest.Digest)},
		{Name: "git-version-tag", Namespace: "legacy-git-version", Version: "v1.0.0", URL: gitURL, GitPath: "git-charts/version-tag"},
		{Name: "git-version-commit", Namespace: "legacy-git-version-commit", Version: gitCommit, URL: gitURL, GitPath: "git-charts/version-commit"},
		{Name: "git-inline-tag", Namespace: "legacy-git-inline-tag", Version: "inline-tag-layout", URL: gitURL + "@v1.0.0", GitPath: "git-charts/inline-tag"},
		{Name: "git-inline-force-tag", Namespace: "legacy-git-inline-force-tag", Version: "inline-force-tag-layout", URL: gitURL + "@+v1.0.0", GitPath: "git-charts/inline-force-tag"},
		{Name: "git-refspec-tag", Namespace: "legacy-git-refspec-tag", Version: "refspec-tag-layout", URL: gitURL + "@refs/tags/v1.0.0", GitPath: "git-charts/refspec-tag"},
		{Name: "git-branch", Namespace: "legacy-git-branch", Version: "branch-layout", URL: gitURL + "@refs/heads/release", GitPath: "git-charts/branch"},
		{Name: "git-commit", Namespace: "legacy-git-commit", Version: "commit-layout", URL: gitURL + "@" + gitCommit, GitPath: "git-charts/commit"},
		{Name: "git-root", Namespace: "legacy-git-root", Version: "root-layout", URL: gitURL + "@v1.0.0"},
	}
	writeLegacyChartPackageDefinition(t, packageDir, charts)
	// FIXME: package remove is probably fine
	t.Cleanup(func() {
		cleanupLegacyChartNamespaces(t, charts)
	})

	legacyBinary := e2e.GetZarfAtVersion(t, legacyChartCompatibilityVersion)
	outputDir := t.TempDir()
	legacyTmpDir := t.TempDir()
	stdout, stderr, err := exec.CmdWithTesting(t, exec.Config{}, legacyBinary,
		"package", "create", packageDir,
		"-o", outputDir,
		"--plain-http",
		"--skip-sbom",
		"--confirm",
		"--no-color",
		"--tmpdir", legacyTmpDir,
	)
	require.NoError(t, err, stdout, stderr)

	packagePath := filepath.Join(outputDir, fmt.Sprintf("zarf-package-legacy-chart-source-compatibility-%s-0.0.1.tar.zst", e2e.Arch))
	stdout, stderr, err = e2e.Zarf(t, "package", "deploy", packagePath, "--confirm")
	require.NoError(t, err, stdout, stderr)

	for _, chart := range charts {
		stdout, stderr, err = e2e.Kubectl(t,
			"get", "configmap", chart.Name+"-config",
			"--namespace", chart.Namespace,
			"--output", "jsonpath={.data.source}",
		)
		require.NoError(t, err, stdout, stderr)
		require.Equal(t, chart.Name, stdout, "chart %q selected the wrong source revision", chart.Name)
	}

	stdout, stderr, err = e2e.Zarf(t, "package", "remove", packagePath, "--confirm")
	require.NoError(t, err, stdout, stderr)
}

func cleanupLegacyChartNamespaces(t *testing.T, charts []v1alpha1.ZarfChart) {
	t.Helper()

	args := []string{"delete", "namespace"}
	for _, chart := range charts {
		args = append(args, chart.Namespace)
	}
	args = append(args, "--ignore-not-found", "--wait=false")
	stdout, stderr, err := e2e.Kubectl(t, args...)
	if err != nil {
		t.Errorf("unable to remove legacy chart test namespaces: %s%s", stdout, stderr)
	}
}

func writeLegacyChartPackageDefinition(t *testing.T, packageDir string, charts []v1alpha1.ZarfChart) {
	t.Helper()

	required := true
	pkg := v1alpha1.ZarfPackage{
		Kind:     v1alpha1.ZarfPackageConfig,
		Metadata: v1alpha1.ZarfMetadata{Name: "legacy-chart-source-compatibility", Version: "0.0.1"},
		Components: []v1alpha1.ZarfComponent{{
			Name:     "charts",
			Required: &required,
			Charts:   charts,
		}},
	}
	require.NoError(t, utils.WriteYaml(filepath.Join(packageDir, "zarf.yaml"), pkg, 0o600))
}

func writeLegacyConfigMapChart(t *testing.T, chartDir, chartName, configMapName, marker string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Join(chartDir, "templates"), 0o700))
	chartYAML := fmt.Sprintf("apiVersion: v2\nname: %s\nversion: 0.1.0\n", chartName)
	configMapYAML := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: %s
  namespace: {{ .Release.Namespace }}
data:
  source: %q
`, configMapName, marker)
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(chartYAML), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "templates", "configmap.yaml"), []byte(configMapYAML), 0o600))
}

func createLegacyChartArchive(t *testing.T, chartName, configMapName, marker string) []byte {
	t.Helper()

	chartDir := filepath.Join(t.TempDir(), chartName)
	writeLegacyConfigMapChart(t, chartDir, chartName, configMapName, marker)
	chart, err := loader.LoadDir(chartDir)
	require.NoError(t, err)
	archivePath, err := chartutil.Save(chart, t.TempDir())
	require.NoError(t, err)
	chartData, err := os.ReadFile(archivePath)
	require.NoError(t, err)
	return chartData
}

func serveLegacyHelmRepository(t *testing.T, chartData []byte, chartName, version string) string {
	t.Helper()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.yaml":
			if _, err := fmt.Fprintf(w, `apiVersion: v1
entries:
  %s:
    - apiVersion: v2
      name: %s
      version: %s
      urls:
        - %s/charts/%s-%s.tgz
`, chartName, chartName, version, server.URL, chartName, version); err != nil {
				return
			}
		case fmt.Sprintf("/charts/%s-%s.tgz", chartName, version):
			if _, err := w.Write(chartData); err != nil {
				return
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func createLegacyChartGitRepository(t *testing.T) (string, string) {
	t.Helper()

	repositoryDir := t.TempDir()
	bareRepository := filepath.Join(repositoryDir, "legacy-charts.git")
	workingDirectory := filepath.Join(repositoryDir, "working")
	_, err := git.PlainInitWithOptions(bareRepository, &git.PlainInitOptions{
		Bare: true,
		InitOptions: git.InitOptions{
			DefaultBranch: plumbing.Main,
		},
	})
	require.NoError(t, err)
	repository, err := git.PlainInitWithOptions(workingDirectory, &git.PlainInitOptions{
		InitOptions: git.InitOptions{
			DefaultBranch: plumbing.Main,
		},
	})
	require.NoError(t, err)
	worktree, err := repository.Worktree()
	require.NoError(t, err)
	_, err = repository.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{bareRepository},
	})
	require.NoError(t, err)
	commitSignature := &object.Signature{
		Name:  "Zarf Test",
		Email: "zarf@example.com",
		When:  time.Unix(0, 0),
	}

	writeLegacyConfigMapChart(t, workingDirectory, "git-root", "git-root-config", "git-root")
	writeLegacyConfigMapChart(t, filepath.Join(workingDirectory, "git-charts", "version-tag"), "git-version-tag", "git-version-tag-config", "git-version-tag")
	writeLegacyConfigMapChart(t, filepath.Join(workingDirectory, "git-charts", "version-commit"), "git-version-commit", "git-version-commit-config", "git-version-commit-initial")
	writeLegacyConfigMapChart(t, filepath.Join(workingDirectory, "git-charts", "inline-tag"), "git-inline-tag", "git-inline-tag-config", "git-inline-tag")
	writeLegacyConfigMapChart(t, filepath.Join(workingDirectory, "git-charts", "inline-force-tag"), "git-inline-force-tag", "git-inline-force-tag-config", "git-inline-force-tag")
	writeLegacyConfigMapChart(t, filepath.Join(workingDirectory, "git-charts", "refspec-tag"), "git-refspec-tag", "git-refspec-tag-config", "git-refspec-tag")
	writeLegacyConfigMapChart(t, filepath.Join(workingDirectory, "git-charts", "commit"), "git-commit", "git-commit-config", "git-commit-initial")
	require.NoError(t, worktree.AddGlob("."))
	initialCommit, err := worktree.Commit("initial charts", &git.CommitOptions{Author: commitSignature})
	require.NoError(t, err)
	_, err = repository.CreateTag("v1.0.0", initialCommit, &git.CreateTagOptions{
		Tagger:  commitSignature,
		Message: "v1.0.0",
	})
	require.NoError(t, err)

	require.NoError(t, worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("release"),
		Create: true,
	}))
	writeLegacyConfigMapChart(t, filepath.Join(workingDirectory, "git-charts", "branch"), "git-branch", "git-branch-config", "git-branch")
	require.NoError(t, worktree.AddGlob("."))
	_, err = worktree.Commit("release chart", &git.CommitOptions{Author: commitSignature})
	require.NoError(t, err)

	require.NoError(t, worktree.Checkout(&git.CheckoutOptions{Branch: plumbing.Main}))
	writeLegacyConfigMapChart(t, filepath.Join(workingDirectory, "git-charts", "commit"), "git-commit", "git-commit-config", "git-commit")
	writeLegacyConfigMapChart(t, filepath.Join(workingDirectory, "git-charts", "version-commit"), "git-version-commit", "git-version-commit-config", "git-version-commit")
	require.NoError(t, worktree.AddGlob("."))
	commit, err := worktree.Commit("commit chart", &git.CommitOptions{Author: commitSignature})
	require.NoError(t, err)
	require.NoError(t, repository.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs: []config.RefSpec{
			"refs/heads/*:refs/heads/*",
			"refs/tags/*:refs/tags/*",
		},
	}))

	return "file://" + bareRepository, commit.String()
}
