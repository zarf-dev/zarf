package test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLint(t *testing.T) {
	t.Log("E2E: Lint")

	t.Run("zarf test lint", func(t *testing.T) {
		t.Log("E2E: Test lint")

		path := filepath.Join("src", "test", "packages", "09-lint")
		stdOut, stdErr, err := e2e.Zarf("prepare", "lint", path)
		require.NoError(t, err, "We don't get an error")
		fmt.Println("printing stdout", stdOut)
		fmt.Println(stdErr)
		fmt.Println(err)
	})
}
