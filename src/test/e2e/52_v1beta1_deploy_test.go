// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"archive/tar"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/internal/gitea"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/test"
)

func TestV1Beta1Deploy(t *testing.T) {
	fixture := filepath.Join("src", "test", "packages", "52-v1beta1-deploy")
	workDir := t.TempDir()
	source := filepath.Join(workDir, "source")
	require.NoError(t, os.CopyFS(source, os.DirFS(fixture)))

	chartRepo := filepath.Join(workDir, "chart.git")
	require.NoError(t, os.CopyFS(chartRepo, os.DirFS(filepath.Join(source, "chart"))))
	repo, err := git.PlainInit(chartRepo, false)
	require.NoError(t, err)
	worktree, err := repo.Worktree()
	require.NoError(t, err)
	require.NoError(t, worktree.AddGlob("."))
	commitOptions := &git.CommitOptions{Author: &object.Signature{Name: "test", Email: "test@example.com"}}
	chartCommit, err := worktree.Commit("original chart", commitOptions)
	require.NoError(t, err)
	// The pinned revision must be older than the default branch's tip.
	chartValues := filepath.Join(chartRepo, "values.yaml")
	require.NoError(t, os.WriteFile(chartValues, []byte("mapped:\n  excluded: newer-commit\n"), 0600))
	require.NoError(t, worktree.AddGlob("."))
	_, err = worktree.Commit("change chart defaults", commitOptions)
	require.NoError(t, err)

	archive, err := os.Create(filepath.Join(source, "archive.tar"))
	require.NoError(t, err)
	writer := tar.NewWriter(archive)
	payload := []byte("from-archive\n")
	require.NoError(t, writer.WriteHeader(&tar.Header{Name: "archive/inside.txt", Mode: 0644, Size: int64(len(payload))}))
	_, err = writer.Write(payload)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	require.NoError(t, archive.Close())
	var networkChecks atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		networkChecks.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	stdOut, stdErr, err := e2e.ZarfInDir(t, source, "dev", "template",
		"--set", "architecture="+e2e.Arch,
		"--set", "networkAddress="+strings.TrimPrefix(server.URL, "http://"),
		"--set", "chartGitURL=file://"+filepath.ToSlash(chartRepo),
		"--set", "chartGitCommit="+chartCommit.String(),
	)
	require.NoError(t, err, "%s\n%s", stdOut, stdErr)

	stdOut, stdErr, err = e2e.ZarfInDir(t, workDir, "package", "create", source, "-o", workDir, "--skip-sbom", "--confirm")
	require.NoError(t, err, "%s\n%s", stdOut, stdErr)
	require.Contains(t, stdOut+stdErr, "create-before")
	require.Contains(t, stdOut+stdErr, "create-success")
	packagePath := filepath.Join(workDir, fmt.Sprintf("zarf-package-v1beta1-deploy-%s.tar.zst", e2e.Arch))
	require.FileExists(t, packagePath)
	docsDir := filepath.Join(workDir, "docs")
	stdOut, stdErr, err = e2e.ZarfInDir(t, workDir, "package", "inspect", "documentation", packagePath, "--output", docsDir)
	require.NoError(t, err, "%s\n%s", stdOut, stdErr)
	docs, err := os.ReadFile(filepath.Join(docsDir, "v1beta1-deploy-documentation", "README.md"))
	require.NoError(t, err)
	require.Contains(t, string(docs), "package documentation inclusion")

	stdOut, stdErr, err = e2e.ZarfInDir(t, workDir, "package", "deploy", packagePath, "--components=resources,optional-selected", "--confirm", "--set-values", "message=from-cli")
	require.NoError(t, err, "%s\n%s", stdOut, stdErr)
	deployed := true
	t.Cleanup(func() {
		if deployed {
			stdOut, stdErr, err := e2e.ZarfInDir(t, workDir, "package", "remove", "v1beta1-deploy", "--confirm")
			require.NoError(t, err, "%s\n%s", stdOut, stdErr)
		}
	})
	require.Positive(t, networkChecks.Load())
	ctx := logger.WithContext(t.Context(), test.GetLogger(t))
	c, err := cluster.New(ctx)
	require.NoError(t, err)
	state, err := c.LoadState(ctx)
	require.NoError(t, err)

	image := state.RegistryInfo.Address + "/zarf-dev/images/hello-world:latest"
	stdOut, stdErr, err = e2e.ZarfInDir(t, workDir, "tools", "registry", "digest", image)
	require.NoError(t, err, "%s\n%s", stdOut, stdErr)
	require.Regexp(t, `^sha256:[a-f0-9]{64}$`, strings.TrimSpace(stdOut))

	gitTunnel, err := c.Connect(ctx, cluster.ZarfGit)
	require.NoError(t, err)
	defer gitTunnel.Close()
	require.Len(t, gitTunnel.HTTPEndpoints(), 1)
	gitClient, err := gitea.NewClient(gitTunnel.HTTPEndpoints()[0], state.GitServer.PullUsername, state.GitServer.PullPassword)
	require.NoError(t, err)
	repoName, err := transform.GitURLtoRepoName("https://github.com/zarf-dev/zarf-public-test.git")
	require.NoError(t, err)
	_, status, err := gitClient.DoRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v1/repos/%s/%s/tags/v0.0.1", state.GitServer.PushUsername, repoName), nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	content, err := os.ReadFile(filepath.Join(workDir, "deployed", "payload.txt"))
	require.NoError(t, err)
	require.Equal(t, "v1beta1 payload\n", string(content))
	link, err := os.Readlink(filepath.Join(workDir, "payload-link.txt"))
	require.NoError(t, err)
	require.NotEmpty(t, link)
	linked, err := os.ReadFile(filepath.Join(workDir, "payload-link.txt"))
	require.NoError(t, err)
	require.Equal(t, content, linked)
	content, err = os.ReadFile(filepath.Join(workDir, "deployed", "template.txt"))
	require.NoError(t, err)
	require.Equal(t, "message=from-cli\n", string(content))
	content, err = os.ReadFile(filepath.Join(workDir, "deployed", "extracted.txt"))
	require.NoError(t, err)
	require.Equal(t, "from-archive\n", string(content))
	info, err := os.Stat(filepath.Join(workDir, "deployed", "executable.txt"))
	require.NoError(t, err)
	require.NotZero(t, info.Mode()&0111)

	for name, expected := range map[string]map[string]string{
		"v1beta1-manifest":   {"message": "from-cli", "action": "action-value", "env": "from-default"},
		"v1beta1-kustomized": {"source": "kustomize"},
		"v1beta1-chart":      {"message": "from-cli", "included": "from-mapping", "excluded": "chart-default"},
		"v1beta1-optional":   {"selected": "true"},
	} {
		for key, value := range expected {
			out, stderr, err := e2e.Kubectl(t, "get", "configmap", name, "-n", "default", "-o", fmt.Sprintf("jsonpath={.data.%s}", key))
			require.NoError(t, err, stderr)
			require.Equal(t, value, out, "%s.data.%s", name, key)
		}
	}

	stdOut, stdErr, err = e2e.ZarfInDir(t, workDir, "package", "remove", "v1beta1-deploy", "--confirm")
	require.NoError(t, err, "%s\n%s", stdOut, stdErr)
	deployed = false
	require.Contains(t, stdOut+stdErr, "remove-before")
	require.Contains(t, stdOut+stdErr, "remove-success")
	_, _, err = e2e.Kubectl(t, "get", "configmap", "v1beta1-manifest", "-n", "default")
	require.Error(t, err)
}
