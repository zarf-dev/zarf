package test

import (
	"fmt"
	"testing"

	teststructure "github.com/gruntwork-io/terratest/modules/test-structure"
	"github.com/stretchr/testify/require"
)

func TestDataInjection(t *testing.T) {

	e2e := NewE2ETest(t)

	// At the end of the test, run `terraform destroy` to clean up any resources that were created
	defer teststructure.RunTestStage(e2e.testing, "TEARDOWN", e2e.teardown)

	// Upload the Zarf artifacts
	teststructure.RunTestStage(e2e.testing, "UPLOAD", func() {
		e2e.syncFileToRemoteServer("../../build/zarf", fmt.Sprintf("/home/%s/build/zarf", e2e.username), "0700")
		e2e.syncFileToRemoteServer("../../build/zarf-init.tar.zst", fmt.Sprintf("/home/%s/build/zarf-init.tar.zst", e2e.username), "0600")
		e2e.syncFileToRemoteServer("../../build/zarf-package-data-injection-demo.tar", fmt.Sprintf("/home/%s/build/zarf-package-data-injection-demo.tar", e2e.username), "0600")
	})

	teststructure.RunTestStage(e2e.testing, "TEST", func() {
		// run `zarf init`
		output, err := e2e.runSSHCommand("sudo bash -c 'cd /home/%s/build && ./zarf init --confirm --components k3s'", e2e.username)
		require.NoError(e2e.testing, err, output)

		// Deploy the data injection example
		output, err = e2e.runSSHCommand("sudo bash -c 'cd /home/%s/build && ./zarf package deploy zarf-package-data-injection-demo.tar --confirm'", e2e.username)
		require.NoError(e2e.testing, err, output)

		// Test to confirm the root file was placed
		output, err = e2e.runSSHCommand(`sudo bash -c '/usr/sbin/kubectl -n demo exec data-injection -- ls /test | grep this-is-an-example'`)
		require.NoError(e2e.testing, err, output)

		// Test to confirm the subdirectory file was placed
		output, err = e2e.runSSHCommand(`sudo bash -c '/usr/sbin/kubectl -n demo exec data-injection -- ls /test/subdirectory-test | grep this-is-an-example'`)
		require.NoError(e2e.testing, err, output)
	})

}
