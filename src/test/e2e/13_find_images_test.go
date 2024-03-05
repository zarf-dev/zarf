package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindImages(t *testing.T) {
	t.Log("E2E: Find Images")

	t.Run("zarf test find images success", func(t *testing.T) {
		t.Log("E2E: Test Find Images")

		testPackagePath := filepath.Join("examples", "dos-games")
		expectedOutput, err := os.ReadFile("src/test/packages/13-find-images/dos-games-find-images-expected.txt")
		require.NoError(t, err)

		stdout, _, err := e2e.Zarf("dev", "find-images", testPackagePath)
		require.NoError(t, err)
		require.Contains(t, stdout, string(expectedOutput))
	})

	t.Run("zarf test find images --why  w/ helm chart success", func(t *testing.T) {
		t.Log("E2E: Test Find Images against a helm chart with why flag")

		testPackagePath := filepath.Join("examples", "wordpress")
		expectedOutput, err := os.ReadFile("src/test/packages/13-find-images/helm-charts-find-images-why-expected.txt")
		require.NoError(t, err)

		stdout, _, err := e2e.Zarf("dev", "find-images", testPackagePath, "--why", "docker.io/bitnami/apache-exporter:0.13.3-debian-11-r2")
		require.NoError(t, err)
		require.Contains(t, stdout, string(expectedOutput))
	})

	t.Run("zarf test find images --why w/  manifests success", func(t *testing.T) {
		t.Log("E2E: Test Find Images against manifests with why flag")

		testPackagePath := filepath.Join("examples", "manifests")
		expectedOutput, err := os.ReadFile("src/test/packages/13-find-images/manifests-find-images-why-expected.txt")
		require.NoError(t, err)

		stdout, _, err := e2e.Zarf("dev", "find-images", testPackagePath, "--why", "httpd:alpine3.18")
		require.NoError(t, err)
		require.Contains(t, stdout, string(expectedOutput))
	})

}
