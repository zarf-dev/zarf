package test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestE2eMultipleFilesInKubeconfig tests that `zarf init` works even if the KUBECONFIG env var contains multiple files
// in it separated by a colon, which is a syntax supported by `kubectl`
func TestE2eMultipleFilesInKubeconfig(t *testing.T) {
	defer e2e.cleanupAfterTest(t)

	originalKubeconfig := os.Getenv("KUBECONFIG")
	defer func(key, value string) {
		err := os.Setenv(key, value)
		require.NoErrorf(t, err, "Unable to set KUBECONFIG env var back to original value")
	}("KUBECONFIG", originalKubeconfig)
	err := os.Setenv("KUBECONFIG", fmt.Sprintf("%s:/foo/bar.yaml", originalKubeconfig))

	//run `zarf init`
	output, err := e2e.execZarfCommand("init", "--confirm")
	require.NoError(t, err, output)
}
