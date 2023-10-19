package test

import (
	"fmt"
	"testing"
)

func TestLint(t *testing.T) {
	t.Log("E2E: Lint")

	t.Run("zarf test lint", func(t *testing.T) {
		t.Log("E2E: Test lint")

		//zarfYaml := filepath.Join("src", "test", "packages", "09-lint")
		path := "src/test/packages/09-lint"
		stdOut, stdErr, err := e2e.Zarf("prepare", "lint", path)
		fmt.Println("printing stdout", stdOut)
		fmt.Println(stdErr)
		fmt.Println(err)
	})
}
