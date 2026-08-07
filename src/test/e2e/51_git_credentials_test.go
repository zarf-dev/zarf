// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fluxcd/gitkit"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

// TestCreateGitSSHDefaultIdentityFallback locks down SSH identity-file auth and known_hosts through the host git fallback.
func TestCreateGitSSHDefaultIdentityFallback(t *testing.T) {
	t.Log("E2E: Git SSH default identity via host git fallback")

	privateKey := newE2ESSHPrivateKey(t)
	repoURL, keyDir := newE2ESSHGitServer(t, "identity.git", privateKey)
	homePath := t.TempDir()
	t.Setenv("HOME", homePath)
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("SSH_KNOWN_HOSTS", "")
	identityPath := writeE2ESSHIdentity(t, homePath, privateKey)
	knownHostsPath := writeE2EKnownHosts(t, homePath, keyDir, repoURL)
	t.Setenv("GIT_SSH_COMMAND", fmt.Sprintf("ssh -o IdentitiesOnly=yes -o IdentityFile=%s -o UserKnownHostsFile=%s", identityPath, knownHostsPath))
	preflight := exec.Command("git", "ls-remote", repoURL)
	preflight.Env = os.Environ()
	preflightOutput, err := preflight.CombinedOutput()
	require.NoError(t, err, string(preflightOutput))

	packageDir := t.TempDir()
	zarfYAML := fmt.Sprintf(`kind: ZarfPackageConfig
metadata:
  name: git-ssh-identity
  version: 0.0.1
components:
  - name: repo
    required: true
    repos:
      - %s
`, repoURL)
	require.NoError(t, os.WriteFile(filepath.Join(packageDir, "zarf.yaml"), []byte(zarfYAML), 0o600))

	outputDir := t.TempDir()
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", packageDir, "-o", outputDir, "--confirm", "--skip-sbom")
	require.NoError(t, err, stdOut, stdErr)

	packagePath := filepath.Join(outputDir, fmt.Sprintf("zarf-package-git-ssh-identity-%s-0.0.1.tar.zst", e2e.Arch))
	require.FileExists(t, packagePath)
}

func newE2ESSHGitServer(t *testing.T, repoPath string, privateKey *rsa.PrivateKey) (string, string) {
	t.Helper()

	gitPath, err := exec.LookPath("git")
	require.NoError(t, err)
	cfg := gitkit.Config{
		Dir:        t.TempDir(),
		AutoCreate: true,
		GitPath:    gitPath,
	}
	gitSrv := gitkit.New(cfg)
	require.NoError(t, gitSrv.Setup())
	httpSrv := httptest.NewServer(http.HandlerFunc(gitSrv.ServeHTTP))
	t.Cleanup(httpSrv.Close)

	repoURL := fmt.Sprintf("%s/%s", httpSrv.URL, repoPath)
	pushE2EFixtureRepository(t, repoURL)
	require.NoError(t, os.WriteFile(filepath.Join(cfg.Dir, filepath.FromSlash(repoPath), "HEAD"), []byte("ref: refs/heads/main\n"), 0o600))

	keyDir := t.TempDir()
	sshSrv := gitkit.NewSSH(gitkit.Config{
		Dir:     cfg.Dir,
		KeyDir:  keyDir,
		GitPath: gitPath,
		Auth:    true,
		GitUser: "git",
	})
	authorizedKey := e2eAuthorizedKey(t, privateKey)
	sshSrv.PublicKeyLookupFunc = func(candidate string) (*gitkit.PublicKey, error) {
		if candidate != authorizedKey {
			return nil, nil
		}
		return &gitkit.PublicKey{Id: "test-key"}, nil
	}
	require.NoError(t, sshSrv.Listen("127.0.0.1:0"))
	t.Cleanup(func() {
		require.NoError(t, sshSrv.Stop())
	})
	go func() {
		_ = sshSrv.Serve()
	}()
	require.Eventually(t, func() bool {
		conn, err := net.Dial("tcp", sshSrv.Address())
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 5*time.Second, 100*time.Millisecond)

	return fmt.Sprintf("ssh://git@%s/%s", sshSrv.Address(), repoPath), keyDir
}

func pushE2EFixtureRepository(t *testing.T, repoURL string) {
	t.Helper()

	storer := memory.NewStorage()
	fs := memfs.New()
	repo, err := git.InitWithOptions(storer, fs, git.InitOptions{DefaultBranch: plumbing.Main})
	require.NoError(t, err)
	worktree, err := repo.Worktree()
	require.NoError(t, err)
	commitE2EFixtureFile(t, fs, worktree)
	_, err = repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{repoURL}})
	require.NoError(t, err)
	require.NoError(t, repo.Push(&git.PushOptions{RemoteName: "origin"}))
}

func commitE2EFixtureFile(t *testing.T, fs billy.Filesystem, worktree *git.Worktree) {
	t.Helper()

	file, err := fs.Create("README.md")
	require.NoError(t, err)
	_, err = file.Write([]byte("fixture\n"))
	require.NoError(t, err)
	require.NoError(t, file.Close())
	_, err = worktree.Add("README.md")
	require.NoError(t, err)
	_, err = worktree.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{Name: "Zarf Test", Email: "zarf@example.com", When: time.Unix(1, 0)},
	})
	require.NoError(t, err)
}

func newE2ESSHPrivateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return privateKey
}

func e2eAuthorizedKey(t *testing.T, privateKey *rsa.PrivateKey) string {
	t.Helper()

	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(publicKey)))
}

func writeE2ESSHIdentity(t *testing.T, homePath string, privateKey *rsa.PrivateKey) string {
	t.Helper()

	sshPath := filepath.Join(homePath, ".ssh")
	require.NoError(t, os.MkdirAll(sshPath, 0o700))
	identityPath := filepath.Join(sshPath, "id_rsa")
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	require.NoError(t, os.WriteFile(identityPath, privateKeyPEM, 0o600))
	return identityPath
}

func writeE2EKnownHosts(t *testing.T, homePath, keyDir, repoURL string) string {
	t.Helper()

	sshPath := filepath.Join(homePath, ".ssh")
	require.NoError(t, os.MkdirAll(sshPath, 0o700))
	publicKey, err := os.ReadFile(filepath.Join(keyDir, "gitkit.rsa.pub"))
	require.NoError(t, err)
	u, err := url.Parse(repoURL)
	require.NoError(t, err)
	host, port, err := net.SplitHostPort(u.Host)
	require.NoError(t, err)
	entry := fmt.Sprintf("[%s]:%s %s\n", host, port, strings.TrimSpace(string(publicKey)))
	knownHostsPath := filepath.Join(sshPath, "known_hosts")
	require.NoError(t, os.WriteFile(knownHostsPath, []byte(entry), 0o600))
	return knownHostsPath
}
