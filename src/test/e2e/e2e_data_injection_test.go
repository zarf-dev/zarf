package test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataInjection(t *testing.T) {
	t.Log("E2E: Testing data injection")
	e2e.setup(t)
	defer e2e.teardown(t)

	path := fmt.Sprintf("build/zarf-package-data-injection-demo-%s.tar", e2e.arch)

	// Deploy the data injection example
	stdOut, stdErr, err := e2e.execZarfCommand("package", "deploy", path, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Get the data injection pod
	namespace := "demo"
	pods, err := e2e.getPodsFromNamespace(namespace)
	require.NoError(t, err)
	require.Equal(t, len(pods.Items), 1)
	pod := pods.Items[0]
	podname := pod.Name

	// Test to confirm the root file was placed
	// NOTE: We need to loop this because the k8s api isn't able to ls the files right away for some reason??
	var execStdOut string
	attempt := 0
	for attempt < 10 && execStdOut == "" {
		execStdOut, _, err = e2e.execCommandInPod(podname, namespace, []string{"ls", "/test"})
		attempt++
		time.Sleep(2 * time.Second)
	}
	assert.NoError(t, err)
	assert.Contains(t, execStdOut, "this-is-an-example-file.txt")

	// Test to confirm the subdirectory file was placed
	// NOTE: This data gets injected after pod comes up as 'healthy' so we need to retry the check until it is able to populate
	execStdOut = ""
	attempt = 0
	for attempt < 10 && execStdOut == "" {
		execStdOut, _, err = e2e.execCommandInPod(podname, namespace, []string{"ls", "/test/subdirectory-test"})
		attempt++
		time.Sleep(2 * time.Second)
	}
	assert.NoError(t, err)
	assert.Contains(t, execStdOut, "this-is-an-example-file.txt")
}
