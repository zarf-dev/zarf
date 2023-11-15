package test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLint(t *testing.T) {
	t.Log("E2E: Lint")

	t.Run("zarf test lint fail", func(t *testing.T) {
		t.Log("E2E: Test lint on schema fail")

		path := filepath.Join("src", "test", "packages", "12-lint")
		_, stderr, err := e2e.Zarf("prepare", "lint", path)
		require.Error(t, err, "Expect error here because the yaml file is not following schema")
		require.Contains(t, stderr, "- components.0.import: Additional property not-path is not allowed")
		require.Contains(t, stderr, "component.2.import.path will not resolve ZARF_PKG_TMPL_* variables")
	})

	t.Run("zarf test lint success", func(t *testing.T) {
		t.Log("E2E: Test lint on schema success")

		// This runs lint on the zarf.yaml in the base directory of the repo
		_, _, err := e2e.Zarf("prepare", "lint")
		require.NoError(t, err, "Expect no error here because the yaml file is following schema")
	})
}
