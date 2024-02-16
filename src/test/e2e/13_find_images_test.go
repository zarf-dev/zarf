package test

import (
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestFindImages(t *testing.T) {
	t.Log("E2E: Find Images")

	t.Run("zarf test find images success", func(t *testing.T) {
		t.Log("E2E: Test Find Images")

		testPackagePath := filepath.Join("examples", "dos-games")
		expectedOutput := []byte{}
		f, err := os.Open("src/test/packages/13-find-images/dos-games-find-images-expected.txt")
		defer f.Close()

		_, err = f.Read(expectedOutput)
		require.NoError(t, err, "Expect no error here while reading expectedOutput of the expected output file")

		stdout, _, err := e2e.Zarf("dev", "find-images", testPackagePath)
		require.NoError(t, err, "Expect no error here")
		require.Contains(t, stdout, string(expectedOutput))
	})

	t.Run("zarf test find images --why  w/ helm chart success", func(t *testing.T) {
		t.Log("E2E: Test Find Images against a helm chart with why flag")

		testPackagePath := filepath.Join("examples", "helm-charts")
		expectedOutput := []byte{}
		f, err := os.Open("src/test/packages/13-find-images/helm-charts-find-images-why-expected.txt")
		defer f.Close()

		_, err = f.Read(expectedOutput)
		require.NoError(t, err, "Expect no error here while reading expectedOutput of the expected output file")

		stdout, _, err := e2e.Zarf("dev", "find-images", testPackagePath, "--why", "curlimages/curl:7.69.0")
		require.NoError(t, err, "Expect no error here")
		require.Contains(t, stdout, string(expectedOutput))
	})

	t.Run("zarf test find images --why w/  manifests success", func(t *testing.T) {
		t.Log("E2E: Test Find Images against a helm chart with why flag")

		testPackagePath := filepath.Join("examples", "manifests")
		expectedOutput := []byte{}
		f, err := os.Open("src/test/packages/13-find-images/manifests-find-images-why-expected.txt")
		defer f.Close()

		_, err = f.Read(expectedOutput)
		require.NoError(t, err, "Expect no error here while reading expectedOutput of the expected output file")

		stdout, _, err := e2e.Zarf("dev", "find-images", testPackagePath, "--why", "httpd:alpine3.18")
		require.NoError(t, err, "Expect no error here")
		require.Contains(t, stdout, string(expectedOutput))
	})

}
