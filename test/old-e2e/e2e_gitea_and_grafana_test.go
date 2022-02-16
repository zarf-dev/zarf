package test

import (
	"fmt"
	"testing"
	"time"

	teststructure "github.com/gruntwork-io/terratest/modules/test-structure"
	"github.com/stretchr/testify/require"
)

func TestGiteaAndGrafana(t *testing.T) {
	e2e := NewE2ETest(t)

	// At the end of the test, run `terraform destroy` to clean up any resources that were created
	defer teststructure.RunTestStage(e2e.testing, "TEARDOWN", e2e.teardown)

	// Upload the Zarf artifacts
	teststructure.RunTestStage(e2e.testing, "UPLOAD", func() {
		e2e.syncFileToRemoteServer("../../build/zarf", fmt.Sprintf("/home/%s/build/zarf", e2e.username), "0700")
		e2e.syncFileToRemoteServer("../../build/zarf-init.tar.zst", fmt.Sprintf("/home/%s/build/zarf-init.tar.zst", e2e.username), "0600")
	})

	teststructure.RunTestStage(e2e.testing, "TEST", func() {
		// run `zarf init`
		output, err := e2e.runSSHCommand("sudo bash -c 'cd /home/%s/build && ./zarf init --confirm --components k3s,logging,gitops-service'", e2e.username)
		require.NoError(e2e.testing, err, output)

		// Establish the port-forward into the gitea service; give the service a few seconds to come up since this is not a command we can retry
		time.Sleep(15 * time.Second)
		_, _ = e2e.runSSHCommand("sudo bash -c '(/home/%s/build/zarf connect git &> /dev/nul &)'", e2e.username)

		// Make sure Gitea comes up cleanly
		output, err = e2e.runSSHCommand(`bash -c '[[ $(curl -sfSL -o /dev/null -w '%%{http_code}' 'http://127.0.0.1:45003/explore/repos') == 200 ]]'`)
		require.NoError(e2e.testing, err, output)

		// Establish the port-forward into the logging service
		_, _ = e2e.runSSHCommand("sudo bash -c '(/home/%s/build/zarf connect logging &> /dev/nul &)'", e2e.username)

		// 	// Make sure Grafana comes up cleanly
		output, err = e2e.runSSHCommand(`bash -c '[[ $(curl -sfSL -o /dev/null -w '%%{http_code}' 'http://127.0.0.1:45002/monitor/login') == 200 ]]'`)
		require.NoError(e2e.testing, err, output)
	})

}
