package test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEphemeralContainers(t *testing.T) {
	t.Log("E2E: Ephemeral Containers mutation")

	tmpdir := t.TempDir()

	// we need to create a test package that contains the images we want to potentially use
	// this should ideally be a single pod such that naming is static
	stdOut, stdErr, err := e2e.Zarf(t, "package", "create", "src/test/packages/38-ephemeral-container", "-o", tmpdir, "--skip-sbom")
	require.NoError(t, err, stdOut, stdErr)
	packageName := fmt.Sprintf("zarf-package-basic-pod-%s-0.0.1.tar.zst", e2e.Arch)
	path := filepath.Join(tmpdir, packageName)

	// deploy the above package
	stdOut, stdErr, err = e2e.Zarf(t, "package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// using a pod the package deploys - run a kubectl debug command
	stdOut, stdErr, err = e2e.Kubectl(t, "debug", "test-pod", "-n", "test", "--image=busybox:1.36")
	require.NoError(t, err, stdOut, stdErr)

	// get the pod and inspect the image used by the ephemeral container
	// it should have been mutated
	podStdOut, _, err := e2e.Kubectl(t, "get", "pod", "test-pod", "-n", "test", "-o", "jsonpath={.status.ephemeralContainerStatuses[*].image}")
	require.NoError(t, err)
	require.Contains(t, podStdOut, "127.0.0.1:31999/library/busybox:1.36-zarf-")

	// cleanup
	stdOut, stdErr, err = e2e.Zarf(t, "package", "remove", "basic-pod", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

}
