package test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataInjection(t *testing.T) {
	defer e2e.cleanupAfterTest(t)

	// run `zarf init`
	output, err := e2e.execZarfCommand("init", "--confirm", "-l=trace")
	require.NoError(t, err, output)

	path := fmt.Sprintf("../../build/zarf-package-data-injection-demo-%s.tar", e2e.arch)

	// Deploy the data injection example
	output, err = e2e.execZarfCommand("package", "deploy", path, "--confirm", "-l=trace")
	require.NoError(t, err, output)

	// Test to confirm the root file was placed
	// TODO: This retry is disgusting, but race condition...
	var execStdOut string
	attempt := 0
	for attempt < 10 && execStdOut == "" {
		execStdOut, _, err = e2e.execCommandInPod("data-injection", "demo", []string{"ls", "/test"})
		attempt++
		time.Sleep(2 * time.Second)
	}
	assert.NoError(t, err)
	assert.Contains(t, execStdOut, "subdirectory-test")

	// Test to confirm the subdirectory file was placed
	execStdOut = ""
	attempt = 0
	for attempt < 10 && execStdOut == "" {
		execStdOut, _, err = e2e.execCommandInPod("data-injection", "demo", []string{"ls", "/test/subdirectory-test"})
		attempt++
		time.Sleep(2 * time.Second)
	}
	assert.NoError(t, err)
	assert.Contains(t, execStdOut, "this-is-an-example-file.txt")
}
