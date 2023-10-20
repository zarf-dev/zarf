package test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLint(t *testing.T) {
	t.Log("E2E: Lint")

	t.Run("zarf test lint", func(t *testing.T) {
		t.Log("E2E: Test lint on schema")

		path := filepath.Join("src", "test", "packages", "09-lint")
		_, _, err := e2e.Zarf("lint", path)
		require.Error(t, err, "We should get an error here because the yaml file is bad")
	})
}
