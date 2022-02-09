package test

import (
	"fmt"
	"testing"

	teststructure "github.com/gruntwork-io/terratest/modules/test-structure"
	"github.com/stretchr/testify/require"
)

func TestGitopsExample(t *testing.T) {
	e2e := NewE2ETest(t)

	// At the end of the test, run `terraform destroy` to clean up any resources that were created
	defer teststructure.RunTestStage(e2e.testing, "TEARDOWN", e2e.teardown)

	// Upload the Zarf artifacts
	teststructure.RunTestStage(e2e.testing, "UPLOAD", func() {
		e2e.syncFileToRemoteServer("../../build/zarf", fmt.Sprintf("/home/%s/build/zarf", e2e.username), "0700")
		e2e.syncFileToRemoteServer("../../build/zarf-init.tar.zst", fmt.Sprintf("/home/%s/build/zarf-init.tar.zst", e2e.username), "0600")
		e2e.syncFileToRemoteServer("../../build/zarf-package-gitops-service-data.tar.zst", fmt.Sprintf("/home/%s/build/zarf-package-gitops-service-data.tar.zst", e2e.username), "0600")
	})

	teststructure.RunTestStage(t, "TEST", func() {
		// run `zarf init`
		output, err := e2e.runSSHCommand("sudo bash -c 'cd /home/%s/build && ./zarf init --confirm --components k3s,logging,gitops-service --host 127.0.0.1'", e2e.username)
		require.NoError(t, err, output)

		// Deploy the gitops example
		output, err = e2e.runSSHCommand("sudo bash -c 'cd /home/%s/build && ./zarf package deploy zarf-package-gitops-service-data.tar.zst --confirm'", e2e.username)
		require.NoError(t, err, output)

		// Create a tunnel to the git resources
		output, err = e2e.runSSHCommand("sudo bash -c '(/home/%s/build/zarf connect git &> /dev/nul &)'", e2e.username)
		require.NoError(t, err, output)

		// Check for full git repo mirror(foo.git) from https://github.com/stefanprodan/podinfo.git
		output, err = e2e.runSSHCommand("sudo bash -c 'cd /home/%s/build && git clone http://zarf-git-user:$(./zarf tools get-admin-password)@127.0.0.1:45003/zarf-git-user/mirror__github.com__stefanprodan__podinfo.git'", e2e.username)
		require.NoError(t, err, output)

		// Check for tagged git repo mirror (foo.git@1.2.3) from https://github.com/defenseunicorns/zarf.git@v0.15.0
		output, err = e2e.runSSHCommand("sudo bash -c 'cd /home/%s/build && git clone http://zarf-git-user:$(./zarf tools get-admin-password)@127.0.0.1:45003/zarf-git-user/mirror__github.com__defenseunicorns__zarf.git'", e2e.username)
		require.NoError(t, err, output)

		// Check for correct tag
		expectedTag := "v0.15.0\n"
		output, err = e2e.runSSHCommand("sudo bash -c 'cd /home/%s/build/mirror__github.com__defenseunicorns__zarf && git tag'", e2e.username)
		require.NoError(t, err, output)
		require.Equal(t, expectedTag, output, "Expected tag should match output")

		// Check for correct commits
		expectedCommits := "9eb207e\n7636dd0\ne02cec9"
		output, err = e2e.runSSHCommand("sudo bash -c 'cd /home/%s/build/mirror__github.com__defenseunicorns__zarf && git log -3 --oneline --pretty=format:\"%%h\"'", e2e.username)
		require.NoError(t, err, output)
		require.Equal(t, expectedCommits, output, "Expected commits should match output")

		// Check for existence of tags without specifying them, signifying that not using '@1.2.3' syntax brought over the whole repo
		expectedTag = "0.2.2"
		output, err = e2e.runSSHCommand("sudo bash -c 'cd /home/%s/build/mirror__github.com__stefanprodan__podinfo && git tag'", e2e.username)
		require.NoError(t, err, output)
		require.Contains(t, output, expectedTag, "Output should contain expected tag")
	})
}
