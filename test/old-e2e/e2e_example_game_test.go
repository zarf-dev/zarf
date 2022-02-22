package test

import (
	"fmt"
	"testing"
	"time"

	teststructure "github.com/gruntwork-io/terratest/modules/test-structure"
	"github.com/stretchr/testify/require"
)

func TestE2eExampleGame(t *testing.T) {

	e2e := NewE2ETest(t)

	// At the end of the test, run `terraform destroy` to clean up any resources that were created
	defer teststructure.RunTestStage(e2e.testing, "TEARDOWN", e2e.teardown)

	// Upload the Zarf artifacts
	teststructure.RunTestStage(e2e.testing, "UPLOAD", func() {
		e2e.syncFileToRemoteServer("../../build/zarf", fmt.Sprintf("/home/%s/build/zarf", e2e.username), "0700")
		e2e.syncFileToRemoteServer("../../build/zarf-init.tar.zst", fmt.Sprintf("/home/%s/build/zarf-init.tar.zst", e2e.username), "0600")
		e2e.syncFileToRemoteServer("../../build/zarf-package-appliance-demo-multi-games.tar.zst", fmt.Sprintf("/home/%s/build/zarf-package-appliance-demo-multi-games.tar.zst", e2e.username), "0600")
	})

	teststructure.RunTestStage(e2e.testing, "TEST", func() {
		// Make sure `zarf --help` doesn't error
		output, err := e2e.runSSHCommand("sudo /home/%s/build/zarf --help", e2e.username)
		require.NoError(e2e.testing, err, output)

		// run `zarf init`
		output, err = e2e.runSSHCommand("sudo bash -c 'cd /home/%s/build && ./zarf init --confirm --components k3s'", e2e.username)
		require.NoError(e2e.testing, err, output)

		// Deploy the game
		output, err = e2e.runSSHCommand("sudo bash -c 'cd /home/%s/build && ./zarf package deploy zarf-package-appliance-demo-multi-games.tar.zst --confirm'", e2e.username)
		require.NoError(e2e.testing, err, output)

		// Establish the port-forward into the game service; give the service a few seconds to come up since this is not a command we can retry
		time.Sleep(5 * time.Second)
		output, err = e2e.runSSHCommand("sudo bash -c '(/home/%s/build/zarf connect doom --local-port 22333 &> /dev/nul &)'", e2e.username)
		require.NoError(e2e.testing, err, output)

		// Right now we're just checking that `curl` returns 0. It can be enhanced by scraping the HTML that gets returned or something.
		output, err = e2e.runSSHCommand("bash -c '[[ $(curl -sfSL --retry 15 --retry-connrefused --retry-delay 5 -o /dev/null -w \"%%{http_code}\" 'http://127.0.0.1:22333?doom') == 200 ]]'")
		require.NoError(e2e.testing, err, output)

		// Run `zarf destroy` to make sure that works correctly
		output, err = e2e.runSSHCommand("sudo bash -c 'cd /home/%s/build && ./zarf destroy --confirm'", e2e.username)
		require.NoError(e2e.testing, err, output)
	})

}
