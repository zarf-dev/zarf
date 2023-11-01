package test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLint(t *testing.T) {
	t.Log("E2E: Lint")

	t.Run("zarf test lint fail", func(t *testing.T) {
		t.Log("E2E: Test lint on schema")

		path := filepath.Join("src", "test", "packages", "09-lint", "invalid_yaml")
		_, _, err := e2e.Zarf("prepare", "lint", path)
		require.Error(t, err, "Expect error here because the yaml file is not following schema")
		//Require contains to make sure the messaging is roughly how we want
	})

	t.Run("zarf test lint success", func(t *testing.T) {
		t.Log("E2E: Test lint on schema")

		path := filepath.Join("src", "test", "packages", "09-lint", "valid_yaml")
		_, _, err := e2e.Zarf("prepare", "lint", path)
		require.NoError(t, err, "Expect no error here because the yaml file is following schema")
	})
}
