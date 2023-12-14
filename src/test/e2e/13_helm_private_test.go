package test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrivateHelm(t *testing.T) {
	t.Log("E2E: Private Helm")

	t.Run("zarf test helm success", func(t *testing.T) {
		t.Log("E2E: Test lint on schema fail")

		path := filepath.Join("src", "test", "packages", "13-private-helm")
		_, _, err := e2e.Zarf("prepare", "find-images", path)
		require.NoError(t, err, "don't require an error because we want this to be a success")
	})
}
