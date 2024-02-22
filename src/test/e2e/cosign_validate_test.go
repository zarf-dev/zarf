package test

import (
	"github.com/stretchr/testify/require"
	"path/filepath"
	"testing"
)

func TestCosignImageValidation(t *testing.T) {
	t.Log("E2E: Validate Cosign Image Signature if available")

	e2e.SetupWithCluster(t)

	createPath := filepath.Join("examples", "argocd")

	// Destroy the cluster to test Zarf cleaning up after itselz[f
	stdOut, stdErr, err := e2e.Zarf("package", "create", createPath, "--confirm", "--log-level=debug")
	require.NoError(t, err, stdOut, stdErr)

}
