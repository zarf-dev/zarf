// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package git

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
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
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/defenseunicorns/pkg/helpers/v2"

	"github.com/zarf-dev/zarf/src/pkg/transform"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestRepository(t *testing.T) {
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

func TestCloneAllRefs(t *testing.T) {
	ctx := testutil.TestContext(t)
	fixture := newGitFixture(t, "all-refs.git")

	repository, err := cloneWithoutHostGit(ctx, t, t.TempDir(), fixture.address, false)
	require.NoError(t, err)

	localRepo := openClonedRepository(t, repository)
	assertHeadHash(t, localRepo, fixture.refs.main)

	remote, err := localRepo.Remote(onlineRemoteName)
	require.NoError(t, err)
	require.Equal(t, []string{fixture.address}, remote.Config().URLs)

	refs := repositoryReferenceNames(t, localRepo)
	require.Contains(t, refs, plumbing.NewBranchReferenceName("main"))
	require.Contains(t, refs, plumbing.NewBranchReferenceName("feature"))
	require.Contains(t, refs, plumbing.NewTagReferenceName("lightweight-v1"))
	require.Contains(t, refs, plumbing.NewTagReferenceName("annotated-v1"))
}

func TestCloneBranchRefShallow(t *testing.T) {
	ctx := testutil.TestContext(t)
	fixture := newGitFixture(t, "branch-ref.git")

	repository, err := cloneWithoutHostGit(ctx, t, t.TempDir(), fixture.address+"@refs/heads/feature", true)
	require.NoError(t, err)

	localRepo := openClonedRepository(t, repository)
	assertHeadHash(t, localRepo, fixture.refs.feature)

	refs := repositoryReferenceNames(t, localRepo)
	require.Contains(t, refs, plumbing.NewRemoteReferenceName(onlineRemoteName, "feature"))
	require.NotContains(t, refs, plumbing.NewRemoteReferenceName(onlineRemoteName, "main"))
	require.NotContains(t, refs, plumbing.NewTagReferenceName("lightweight-v1"))
	require.NotContains(t, refs, plumbing.NewTagReferenceName("annotated-v1"))
}

func TestCloneLightweightTagRef(t *testing.T) {
	ctx := testutil.TestContext(t)
	fixture := newGitFixture(t, "lightweight-tag.git")

	repository, err := cloneWithoutHostGit(ctx, t, t.TempDir(), fixture.address+"@lightweight-v1", true)
	require.NoError(t, err)

	localRepo := openClonedRepository(t, repository)
	assertHeadHash(t, localRepo, fixture.refs.main)

	branch, err := localRepo.Reference(plumbing.NewBranchReferenceName("zarf-ref-lightweight-v1"), true)
	require.NoError(t, err)
	require.Equal(t, fixture.refs.main, branch.Hash())
}

func TestCloneAnnotatedTagRef(t *testing.T) {
	ctx := testutil.TestContext(t)
	fixture := newGitFixture(t, "annotated-tag.git")

	repository, err := cloneWithoutHostGit(ctx, t, t.TempDir(), fixture.address+"@annotated-v1", true)
	require.NoError(t, err)

	localRepo := openClonedRepository(t, repository)
	assertHeadHash(t, localRepo, fixture.refs.main)

	branch, err := localRepo.Reference(plumbing.NewBranchReferenceName("zarf-ref-annotated-v1"), true)
	require.NoError(t, err)
	require.Equal(t, fixture.refs.main, branch.Hash())
}

func TestCloneCommitSHARef(t *testing.T) {
	ctx := testutil.TestContext(t)
	fixture := newGitFixture(t, "commit-sha.git")
	ref := fixture.refs.main.String()

	repository, err := cloneWithoutHostGit(ctx, t, t.TempDir(), fixture.address+"@"+ref, false)
	require.NoError(t, err)

	localRepo := openClonedRepository(t, repository)
	assertHeadHash(t, localRepo, fixture.refs.main)

	branch, err := localRepo.Reference(plumbing.NewBranchReferenceName("zarf-ref-"+ref), true)
	require.NoError(t, err)
	require.Equal(t, fixture.refs.main, branch.Hash())
}

func TestCloneAzureStyleURL(t *testing.T) {
	ctx := testutil.TestContext(t)
	fixture := newAzureGitFixture(t)

	repository, err := cloneWithoutHostGit(ctx, t, t.TempDir(), fixture.address, false)
	require.NoError(t, err)

	localRepo := openClonedRepository(t, repository)
	assertHeadHash(t, localRepo, fixture.refs.main)

	remote, err := localRepo.Remote(onlineRemoteName)
	require.NoError(t, err)
	require.Equal(t, []string{fixture.address}, remote.Config().URLs)
}

func TestCloneDoesNotInvokeHostGit(t *testing.T) {
	ctx := testutil.TestContext(t)
	fixture := newGitFixture(t, "no-host-git.git")

	repository, err := cloneWithoutHostGit(ctx, t, t.TempDir(), fixture.address, false)
	require.NoError(t, err)
	require.NotEmpty(t, repository.Path())
}

// TestCloneHTTPURLUserinfoAuth locks down HTTP Basic auth sourced from URL userinfo.
func TestCloneHTTPURLUserinfoAuth(t *testing.T) {
	ctx := testutil.TestContext(t)
	cred := testHTTPCredential()
	fixture := newAuthenticatedHTTPGitFixture(t, "url-userinfo.git", cred, func(serverURL string) string {
		u, err := url.Parse(serverURL)
		require.NoError(t, err)
		u.User = url.UserPassword(cred.username, cred.password)
		u.Path = "/url-userinfo.git"
		return u.String()
	})

	repository, err := cloneWithoutHostGit(ctx, t, t.TempDir(), fixture.address, false)
	require.NoError(t, err)

	localRepo := openClonedRepository(t, repository)
	assertHeadHash(t, localRepo, fixture.refs.main)
}

// TestCloneHTTPGitCredentialsAuth locks down HTTP Basic auth sourced from ~/.git-credentials.
func TestCloneHTTPGitCredentialsAuth(t *testing.T) {
	ctx := testutil.TestContext(t)
	cred := testHTTPCredential()
	fixture := newAuthenticatedHTTPGitFixture(t, "git-credentials.git", cred, func(serverURL string) string {
		return serverURL + "/git-credentials.git"
	})
	homePath := t.TempDir()
	t.Setenv("HOME", homePath)
	u, err := url.Parse(fixture.address)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(homePath, ".git-credentials"), []byte(fmt.Sprintf("https://%s:%s@%s\n", url.QueryEscape(cred.username), url.QueryEscape(cred.password), u.Host)), 0o600)
	require.NoError(t, err)

	repository, err := cloneWithoutHostGit(ctx, t, t.TempDir(), fixture.address, false)
	require.NoError(t, err)

	localRepo := openClonedRepository(t, repository)
	assertHeadHash(t, localRepo, fixture.refs.main)
}

// TestCloneHTTPNetrcAuth locks down HTTP Basic auth sourced from ~/.netrc.
func TestCloneHTTPNetrcAuth(t *testing.T) {
	ctx := testutil.TestContext(t)
	cred := testHTTPCredential()
	fixture := newAuthenticatedHTTPGitFixture(t, "netrc.git", cred, func(serverURL string) string {
		return serverURL + "/netrc.git"
	})
	homePath := t.TempDir()
	t.Setenv("HOME", homePath)
	u, err := url.Parse(fixture.address)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(homePath, ".netrc"), []byte(fmt.Sprintf("machine %s login %s password %s\n", u.Host, cred.username, cred.password)), 0o600)
	require.NoError(t, err)

	repository, err := cloneWithoutHostGit(ctx, t, t.TempDir(), fixture.address, false)
	require.NoError(t, err)

	localRepo := openClonedRepository(t, repository)
	assertHeadHash(t, localRepo, fixture.refs.main)
}

// TestCloneSSHAgentAuth locks down SSH public key auth sourced from SSH_AUTH_SOCK and verified by known_hosts.
func TestCloneSSHAgentAuth(t *testing.T) {
	ctx := testutil.TestContext(t)
	privateKey := newTestPrivateKey(t)
	fixture := newSSHGitFixture(t, "ssh-agent.git", privateKey)
	homePath := t.TempDir()
	t.Setenv("HOME", homePath)
	t.Setenv("SSH_KNOWN_HOSTS", "")
	writeKnownHosts(t, fixture)
	startTestSSHAgent(t, privateKey)

	repository, err := cloneWithoutHostGit(ctx, t, t.TempDir(), fixture.address, false)
	require.NoError(t, err)

	localRepo := openClonedRepository(t, repository)
	assertHeadHash(t, localRepo, fixture.refs.main)
}

func TestOpenLegacyRepoPath(t *testing.T) {
	rootPath := t.TempDir()
	address := "https://github.com/zarf-dev/zarf.git@v1.2.3"
	legacyFolder, err := transform.GitURLtoRepoName(address)
	require.NoError(t, err)
	legacyPath := filepath.Join(rootPath, legacyFolder)
	err = os.MkdirAll(legacyPath, 0o700)
	require.NoError(t, err)

	repository, err := Open(rootPath, address)
	require.NoError(t, err)
	require.Equal(t, legacyPath, repository.Path())
}

type gitFixture struct {
	cfg       gitkit.Config
	address   string
	repoPath  string
	sshKeyDir string
	refs      gitFixtureRefs
}

type gitFixtureRefs struct {
	main    plumbing.Hash
	feature plumbing.Hash
}

type httpCredential struct {
	username string
	password string
}

func testHTTPCredential() httpCredential {
	return httpCredential{
		username: "zarf-user",
		password: "zarf-password",
	}
}

func newGitFixture(t *testing.T, repoPath string) gitFixture {
	t.Helper()
	return newGitFixtureWithAddress(t, repoPath, func(serverURL string) string {
		return fmt.Sprintf("%s/%s", serverURL, repoPath)
	}, nil)
}

func newAzureGitFixture(t *testing.T) gitFixture {
	t.Helper()
	const repoPath = "me0515/zarf-public-test/_git/zarf-public-test"
	return newGitFixtureWithAddress(t, repoPath, func(serverURL string) string {
		u, err := url.Parse(serverURL)
		require.NoError(t, err)
		u.User = url.User("me0515")
		u.Path = "/" + repoPath
		return u.String()
	}, nil)
}

func newAuthenticatedHTTPGitFixture(t *testing.T, repoPath string, cred httpCredential, address func(string) string) gitFixture {
	t.Helper()
	auth := &githttp.BasicAuth{
		Username: cred.username,
		Password: cred.password,
	}
	return newGitFixtureWithAddress(t, repoPath, address, auth, func(gitSrv *gitkit.Server) {
		gitSrv.AuthFunc = func(candidate gitkit.Credential, _ *gitkit.Request) (bool, error) {
			return candidate.Username == cred.username && candidate.Password == cred.password, nil
		}
	})
}

func newGitFixtureWithAddress(t *testing.T, repoPath string, address func(string) string, auth transport.AuthMethod, configure ...func(*gitkit.Server)) gitFixture {
	t.Helper()
	gitPath, err := exec.LookPath("git")
	require.NoError(t, err)

	cfg := gitkit.Config{
		Dir:        t.TempDir(),
		AutoCreate: true,
		GitPath:    gitPath,
		Auth:       len(configure) > 0,
	}
	gitSrv := gitkit.New(cfg)
	for _, configureServer := range configure {
		configureServer(gitSrv)
	}
	err = gitSrv.Setup()
	require.NoError(t, err)
	srv := httptest.NewServer(http.HandlerFunc(gitSrv.ServeHTTP))
	t.Cleanup(srv.Close)

	repoAddress := address(srv.URL)
	refs := pushFixtureRepository(t, repoAddress, auth)
	writeRemoteHEAD(t, cfg.Dir, repoPath, "main")

	return gitFixture{
		cfg:      cfg,
		address:  repoAddress,
		repoPath: repoPath,
		refs:     refs,
	}
}

func newSSHGitFixture(t *testing.T, repoPath string, privateKey *rsa.PrivateKey) gitFixture {
	t.Helper()
	fixture := newGitFixture(t, repoPath)
	keyDir := t.TempDir()
	server := gitkit.NewSSH(gitkit.Config{
		Dir:     fixture.cfg.Dir,
		KeyDir:  keyDir,
		GitPath: fixture.cfg.GitPath,
		Auth:    true,
		GitUser: "git",
	})
	authorizedKey := testAuthorizedKey(t, privateKey)
	server.PublicKeyLookupFunc = func(candidate string) (*gitkit.PublicKey, error) {
		if candidate != authorizedKey {
			return nil, nil
		}
		return &gitkit.PublicKey{Id: "test-key"}, nil
	}
	err := server.Listen("127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, server.Stop())
	})
	go func() {
		_ = server.Serve()
	}()

	fixture.address = fmt.Sprintf("ssh://git@%s/%s", server.Address(), repoPath)
	fixture.sshKeyDir = keyDir
	return fixture
}

func pushFixtureRepository(t *testing.T, repoAddress string, auth transport.AuthMethod) gitFixtureRefs {
	t.Helper()

	storer := memory.NewStorage()
	fs := memfs.New()
	initRepo, err := git.InitWithOptions(storer, fs, git.InitOptions{
		DefaultBranch: plumbing.Main,
	})
	require.NoError(t, err)
	w, err := initRepo.Worktree()
	require.NoError(t, err)

	mainHash := commitFixtureFile(t, fs, w, "main.txt", "main\n", "main commit")
	_, err = initRepo.CreateTag("lightweight-v1", mainHash, nil)
	require.NoError(t, err)
	_, err = initRepo.CreateTag("annotated-v1", mainHash, &git.CreateTagOptions{
		Tagger:  testSignature(),
		Message: "annotated v1",
	})
	require.NoError(t, err)

	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("feature"),
		Create: true,
	})
	require.NoError(t, err)
	featureHash := commitFixtureFile(t, fs, w, "feature.txt", "feature\n", "feature commit")

	_, err = initRepo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{repoAddress},
	})
	require.NoError(t, err)
	err = initRepo.Push(&git.PushOptions{
		RemoteName: "origin",
		Auth:       auth,
		RefSpecs: []config.RefSpec{
			"refs/heads/*:refs/heads/*",
			"refs/tags/*:refs/tags/*",
		},
	})
	require.NoError(t, err)

	return gitFixtureRefs{
		main:    mainHash,
		feature: featureHash,
	}
}

func commitFixtureFile(t *testing.T, fs billy.Filesystem, w *git.Worktree, filePath, content, message string) plumbing.Hash {
	t.Helper()

	file, err := fs.Create(filePath)
	require.NoError(t, err)
	_, err = file.Write([]byte(content))
	require.NoError(t, err)
	err = file.Close()
	require.NoError(t, err)
	_, err = w.Add(filePath)
	require.NoError(t, err)
	hash, err := w.Commit(message, &git.CommitOptions{
		Author: testSignature(),
	})
	require.NoError(t, err)
	return hash
}

func testSignature() *object.Signature {
	return &object.Signature{
		Name:  "Zarf Test",
		Email: "zarf@example.com",
		When:  time.Unix(1, 0),
	}
}

func writeRemoteHEAD(t *testing.T, repoRoot, repoPath, branch string) {
	t.Helper()

	headFile := filepath.Join(repoRoot, filepath.FromSlash(repoPath), "HEAD")
	err := os.WriteFile(headFile, []byte(fmt.Sprintf("ref: refs/heads/%s\n", branch)), 0o600)
	require.NoError(t, err)
}

func newTestPrivateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return privateKey
}

func testAuthorizedKey(t *testing.T, privateKey *rsa.PrivateKey) string {
	t.Helper()

	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(publicKey)))
}

func writeKnownHosts(t *testing.T, fixture gitFixture) {
	t.Helper()

	homePath, err := os.UserHomeDir()
	require.NoError(t, err)
	sshPath := filepath.Join(homePath, ".ssh")
	err = os.MkdirAll(sshPath, 0o700)
	require.NoError(t, err)

	u, err := url.Parse(fixture.address)
	require.NoError(t, err)
	host, port, err := net.SplitHostPort(u.Host)
	require.NoError(t, err)
	publicKey, err := os.ReadFile(filepath.Join(fixture.sshKeyDir, "gitkit.rsa.pub"))
	require.NoError(t, err)
	entry := fmt.Sprintf("[%s]:%s %s\n", host, port, strings.TrimSpace(string(publicKey)))
	err = os.WriteFile(filepath.Join(sshPath, "known_hosts"), []byte(entry), 0o600)
	require.NoError(t, err)
}

func startTestSSHAgent(t *testing.T, privateKey *rsa.PrivateKey) {
	t.Helper()

	keyring := agent.NewKeyring()
	err := keyring.Add(agent.AddedKey{PrivateKey: privateKey})
	require.NoError(t, err)

	socketPath := filepath.Join(t.TempDir(), "agent.sock")
	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, listener.Close())
	})
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				_ = agent.ServeAgent(keyring, conn)
				_ = conn.Close()
			}()
		}
	}()
	t.Setenv("SSH_AUTH_SOCK", socketPath)
}

func cloneWithoutHostGit(ctx context.Context, t *testing.T, rootPath, address string, shallow bool) (*Repository, error) {
	t.Helper()

	fakeBin := t.TempDir()
	marker := filepath.Join(fakeBin, "git-called")
	fakeGit := filepath.Join(fakeBin, "git")
	err := os.WriteFile(fakeGit, []byte(fmt.Sprintf("#!/bin/sh\necho called > %q\nexit 1\n", marker)), 0o700)
	require.NoError(t, err)
	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+originalPath)

	repository, cloneErr := Clone(ctx, rootPath, address, shallow)
	_, err = os.Stat(marker)
	require.ErrorIs(t, err, os.ErrNotExist)
	return repository, cloneErr
}

func openClonedRepository(t *testing.T, repository *Repository) *git.Repository {
	t.Helper()

	localRepo, err := git.PlainOpen(repository.Path())
	require.NoError(t, err)
	return localRepo
}

func assertHeadHash(t *testing.T, repository *git.Repository, want plumbing.Hash) {
	t.Helper()

	head, err := repository.Head()
	require.NoError(t, err)
	require.Equal(t, want, head.Hash())
}

func repositoryReferenceNames(t *testing.T, repository *git.Repository) map[plumbing.ReferenceName]bool {
	t.Helper()

	iter, err := repository.References()
	require.NoError(t, err)
	defer iter.Close()

	refs := map[plumbing.ReferenceName]bool{}
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		refs[ref.Name()] = true
		return nil
	})
	require.NoError(t, err)
	return refs
}
