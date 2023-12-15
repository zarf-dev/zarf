package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrivateHelm(t *testing.T) {
	t.Log("E2E: Private Helm")

	t.Run("zarf test helm success", func(t *testing.T) {
		t.Log("E2E: Private helm success")

		packagePath := filepath.Join("src", "test", "packages", "13-private-helm")
		cwd, _ := os.Getwd()
		repoPath := filepath.Join(cwd, packagePath, "repositories.yaml")

		os.Setenv("HELM_REPOSITORY_CONFIG", repoPath)
		// TODO this doesn't work, do we care about giving this option through CLI
		// _, _, err := e2e.Zarf("prepare", "find-images", packagePath, "--repository-config", repoPath)
		_, _, err := e2e.Zarf("prepare", "find-images", packagePath)
		require.NoError(t, err, "don't require an error because we want this to be a success")
	})
}
