package test

import (
	"path/filepath"
	"testing"
)

func TestLint(t *testing.T) {
	t.Log("E2E: Lint")

	t.Run("zarf test lint", func(t *testing.T) {
		t.Log("E2E: Test lint")

		zarfYaml := filepath.Join("src", "test", "packages", "09-lint")
		e2e.Zarf("prepare", "lint", zarfYaml)
	})
}
