package test

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitopsExample(t *testing.T) {
	defer e2e.cleanupAfterTest(t)

	// run `zarf init`
	output, err := e2e.execZarfCommand("init", "--confirm", "--components=gitops-service")
	require.NoError(t, err, output)

	path := fmt.Sprintf("../../build/zarf-package-gitops-service-data-%s.tar.zst", e2e.arch)

	// Deploy the gitops example
	output, err = e2e.execZarfCommand("package", "deploy", path, "--confirm")
	require.NoError(t, err, output)

	// Create a tunnel to the git resources
	err = e2e.execZarfBackgroundCommand("connect", "git", "--cli-only")
	assert.NoError(t, err, "unable to establish tunnel to git")

	// Check for full git repo mirror (foo.git) from https://github.com/stefanprodan/podinfo.git
	adminPassword, err := e2e.execZarfCommand("tools", "get-admin-password")
	assert.NoError(t, err, "Unable to get admin password for gitea instance")

	cloneCommand := fmt.Sprintf("http://zarf-git-user:%s@127.0.0.1:45003/zarf-git-user/mirror__github.com__stefanprodan__podinfo.git", strings.TrimSpace(string(adminPassword)))
	gitOutput, err := exec.Command("git", "clone", cloneCommand).CombinedOutput()
	assert.NoError(t, err, string(gitOutput))
	e2e.filesToRemove = append(e2e.filesToRemove, "mirror__github.com__stefanprodan__podinfo")

	// Check for tagged git repo mirror (foo.git@1.2.3) from https://github.com/defenseunicorns/zarf.git@v0.15.0
	cloneCommand = fmt.Sprintf("http://zarf-git-user:%s@127.0.0.1:45003/zarf-git-user/mirror__github.com__defenseunicorns__zarf.git", strings.TrimSpace(string(adminPassword)))
	gitOutput, err = exec.Command("git", "clone", cloneCommand).CombinedOutput()
	assert.NoError(t, err, string(gitOutput))
	e2e.filesToRemove = append(e2e.filesToRemove, "mirror__github.com__defenseunicorns__zarf")

	// Check for correct tag
	expectedTag := "v0.15.0\n"
	err = os.Chdir("mirror__github.com__defenseunicorns__zarf")
	assert.NoError(t, err)
	gitOutput, _ = exec.Command("git", "tag").Output()
	assert.Equal(t, expectedTag, string(gitOutput), "Expected tag should match output")

	// Check for correct commits
	expectedCommits := "9eb207e\n7636dd0\ne02cec9"
	gitOutput, err = exec.Command("git", "log", "-3", "--oneline", "--pretty=format:%h").CombinedOutput()
	assert.NoError(t, err, string(gitOutput))
	assert.Equal(t, expectedCommits, string(gitOutput), "Expected commits should match output")

	// Check for existence of tags without specifying them, signifying that not using '@1.2.3' syntax brought over the whole repo
	expectedTag = "0.2.2"
	err = os.Chdir("../mirror__github.com__stefanprodan__podinfo")
	assert.NoError(t, err)
	gitOutput, err = exec.Command("git", "tag").CombinedOutput()
	assert.NoError(t, err, string(gitOutput))
	assert.Contains(t, string(gitOutput), expectedTag)

	err = os.Chdir("..")
	assert.NoError(t, err, "unable to change directories back to blah blah blah")
}
