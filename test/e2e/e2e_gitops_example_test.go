package test

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitopsExample(t *testing.T) {
	// run `zarf init`
	output, err := exec.Command(e2e.zarfBinPath, "init", "--confirm", "--components=gitops-service").CombinedOutput()
	require.NoError(t, err, string(output))

	// Deploy the gitops example
	output, err = exec.Command(e2e.zarfBinPath, "package", "deploy", "../../build/zarf-package-gitops-service-data.tar.zst", "--confirm").CombinedOutput()
	require.NoError(t, err, string(output))

	time.Sleep(5 * time.Second)

	// Create a tunnel to the git resources
	tunnelCmd := exec.Command(e2e.zarfBinPath, "connect", "git")
	err = tunnelCmd.Start()
	require.NoError(t, err, "unable to establish tunnel to git")
	e2e.cmdsToKill = append(e2e.cmdsToKill, tunnelCmd)
	time.Sleep(1 * time.Second)

	// Check for full git repo mirror (foo.git) from https://github.com/stefanprodan/podinfo.git
	adminPassword, err := exec.Command(e2e.zarfBinPath, "tools", "get-admin-password").Output()
	require.NoError(t, err, "Unable to get admin password for gitea instance")

	cloneCommand := fmt.Sprintf("http://zarf-git-user:%s@127.0.0.1:45003/zarf-git-user/mirror__github.com__stefanprodan__podinfo.git", strings.TrimSpace(string(adminPassword)))
	output, err = exec.Command("git", "clone", cloneCommand).CombinedOutput()
	require.NoError(t, err, string(output))
	e2e.filesToRemove = append(e2e.filesToRemove, "mirror__github.com__stefanprodan__podinfo")

	// Check for tagged git repo mirror (foo.git@1.2.3) from https://github.com/defenseunicorns/zarf.git@v0.15.0
	cloneCommand = fmt.Sprintf("http://zarf-git-user:%s@127.0.0.1:45003/zarf-git-user/mirror__github.com__defenseunicorns__zarf.git", strings.TrimSpace(string(adminPassword)))
	output, err = exec.Command("git", "clone", cloneCommand).CombinedOutput()
	require.NoError(t, err, string(output))
	e2e.filesToRemove = append(e2e.filesToRemove, "mirror__github.com__defenseunicorns__zarf")

	// Check for correct tag
	expectedTag := "v0.15.0\n"
	err = os.Chdir("mirror__github.com__defenseunicorns__zarf")
	require.NoError(t, err)
	output, err = exec.Command("git", "tag").Output()
	assert.Equal(t, expectedTag, string(output), "Expected tag should match output")

	// Check for correct commits
	expectedCommits := "9eb207e\n7636dd0\ne02cec9"
	output, err = exec.Command("git", "log", "-3", "--oneline", "--pretty=format:%h").CombinedOutput()
	require.NoError(t, err, string(output))
	assert.Equal(t, expectedCommits, string(output), "Expected commits should match output")

	// Check for existence of tags without specifying them, signifying that not using '@1.2.3' syntax brought over the whole repo
	expectedTag = "0.2.2"
	err = os.Chdir("../mirror__github.com__stefanprodan__podinfo")
	require.NoError(t, err)
	output, err = exec.Command("git", "tag").CombinedOutput()
	require.NoError(t, err, string(output))
	assert.Contains(t, string(output), expectedTag)

	err = os.Chdir("..")
	require.NoError(t, err, "unable to change directories back to blah blah blah")

	e2e.cleanupAfterTest(t)
}
