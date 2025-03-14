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
	_, err = w.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Email: "example@example.com",
		},
	})
	require.NoError(t, err)
	_, err = initRepo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{repoAddress},
	})
	require.NoError(t, err)
	err = initRepo.Push(&git.PushOptions{
		RemoteName: "origin",
	})
	require.NoError(t, err)

	// TODO: Is there a configuration that defines contents of HEAD that isn't read from ~/.gitconfig
	// Force-write refs/heads/main ref to HEAD to disk - Matching the above reference and decoupling from host gitconfig
	headFile := filepath.Join(cfg.Dir, "test.git", "HEAD")
	err = os.WriteFile(headFile, []byte("ref: refs/heads/main\n"), 0644)
	require.NoError(t, err, "Failed to write HEAD to disk")

	repo, err := Clone(ctx, rootPath, repoAddress, false)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(rootPath, expectedPath), repo.Path())

	repo, err = Open(rootPath, repoAddress)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(rootPath, expectedPath), repo.Path())
}
