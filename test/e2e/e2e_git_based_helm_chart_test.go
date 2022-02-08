package test

import (
	"fmt"
	"testing"

	teststructure "github.com/gruntwork-io/terratest/modules/test-structure"
	"github.com/stretchr/testify/require"
)

func TestGitBasedHelmChart(t *testing.T) {
	e2e := NewE2ETest(t)

	// At the end of the test, run `terraform destroy` to clean up any resources that were created
	defer teststructure.RunTestStage(e2e.testing, "TEARDOWN", e2e.teardown)

	// Upload the Zarf artifacts
	teststructure.RunTestStage(e2e.testing, "UPLOAD", func() {

		e2e.syncFileToRemoteServer("../../build/zarf", fmt.Sprintf("/home/%s/build/zarf", e2e.username), "0700")
		e2e.syncFileToRemoteServer("../../build/zarf-init.tar.zst", fmt.Sprintf("/home/%s/build/zarf-init.tar.zst", e2e.username), "0600")
		e2e.syncFileToRemoteServer("../../build/zarf-package-big-bang-single-package-demo.tar.zst", fmt.Sprintf("/home/%s/build/zarf-package-big-bang-single-package-demo.tar.zst", e2e.username), "0600")
	})

	teststructure.RunTestStage(e2e.testing, "TEST", func() {
		// run `zarf init`
		output, err := e2e.runSSHCommand("sudo bash -c 'cd /home/%s/build && ./zarf init --confirm --components k3s'", e2e.username)
		require.NoError(e2e.testing, err, output)

		// Deploy the single-big-bang-package example
		output, err = e2e.runSSHCommand("sudo bash -c 'cd /home/%s/build && ./zarf package deploy zarf-package-big-bang-single-package-demo.tar.zst --confirm'", e2e.username)
		require.NoError(e2e.testing, err, output)

		// Wait until the deployment is ready
		output, err = e2e.runSSHCommand(`timeout 300 sudo bash -c 'while [ "$(/usr/local/bin/kubectl get pods -n twistlock -l app=twistlock-console --field-selector=status.phase=Running -o json | jq -r '"'"'.items | length'"'"')" -lt "1" ]; do sleep 1; done' || false`)
		require.NoError(e2e.testing, err, output)
	})

}
