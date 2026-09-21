// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package git

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/fluxcd/gitkit"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/require"

	"github.com/defenseunicorns/pkg/helpers/v2"

	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestRepository(t *testing.T) {
	t.Parallel()
	ctx := testutil.TestContext(t)

	cfg := gitkit.Config{
		Dir:        t.TempDir(),
		AutoCreate: true,
	}
	gitSrv := gitkit.New(cfg)
	err := gitSrv.Setup()
	require.NoError(t, err)
	srv := httptest.NewServer(http.HandlerFunc(gitSrv.ServeHTTP))
	t.Cleanup(func() {
		srv.Close()
	})

	rootPath := t.TempDir()
	repoName := "test"
	repoAddress := fmt.Sprintf("%s/%s.git", srv.URL, repoName)
	checksum := helpers.GetCRCHash(repoAddress)
	expectedPath := fmt.Sprintf("%s-%d", repoName, checksum)

	storer := memory.NewStorage()
	fs := memfs.New()
	options := git.InitOptions{
		DefaultBranch: plumbing.Main,
	}
	initRepo, err := git.InitWithOptions(storer, fs, options)
	require.NoError(t, err)
	w, err := initRepo.Worktree()
	require.NoError(t, err)
	filePath := "test.txt"
	newFile, err := fs.Create(filePath)
	require.NoError(t, err)
	_, err = newFile.Write([]byte("Hello World"))
	require.NoError(t, err)
	err = newFile.Close()
	require.NoError(t, err)
	_, err = w.Add(filePath)
	require.NoError(t, err)
	commit, err := w.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Email: "example@example.com",
		},
	})
	require.NoError(t, err)
	_, err = initRepo.CreateTag("v1.0.0", commit, nil)
	require.NoError(t, err)
	_, err = initRepo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{repoAddress},
	})
	require.NoError(t, err)
	err = initRepo.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs: []config.RefSpec{
			"refs/heads/*:refs/heads/*",
			"refs/tags/*:refs/tags/*",
		},
	})
	require.NoError(t, err)

	// TODO: Is there a configuration that defines contents of HEAD that isn't read from ~/.gitconfig
	// Force-write refs/heads/main ref to HEAD to disk - Matching the above reference and decoupling from host gitconfig
	headFile := filepath.Join(cfg.Dir, "test.git", "HEAD")
	err = os.WriteFile(headFile, []byte("ref: refs/heads/main\n"), 0644)
	require.NoError(t, err, "Failed to write HEAD to disk")

	source := api.Repository{URL: repoAddress}
	repo, err := Clone(ctx, rootPath, source, false)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(rootPath, expectedPath), repo.Path())

	repo, err = Open(rootPath, source)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(rootPath, expectedPath), repo.Path())

	tagSource := api.Repository{URL: repoAddress, Ref: &api.GitRef{Tag: "v1.0.0"}}
	tagRepo, err := Clone(ctx, rootPath, tagSource, false)
	require.NoError(t, err)
	tagChecksum := helpers.GetCRCHash(repoAddress + "@v1.0.0")
	require.Equal(t, filepath.Join(rootPath, fmt.Sprintf("%s-%d", repoName, tagChecksum)), tagRepo.Path())

	tagRepo, err = Open(rootPath, tagSource)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(rootPath, fmt.Sprintf("%s-%d", repoName, tagChecksum)), tagRepo.Path())

	_, err = Clone(ctx, rootPath, api.Repository{
		URL: repoAddress + "@legacy-tag",
		Ref: &api.GitRef{Tag: "v1.0.0"},
	}, false)
	require.ErrorContains(t, err, "defines a ref in both its URL and ref field")
}
