package test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/stretchr/testify/require"
)

func TestLint(t *testing.T) {
	t.Log("E2E: Lint")

	t.Run("zarf test lint success", func(t *testing.T) {
		t.Log("E2E: Test lint on schema success")

		// This runs lint on the zarf.yaml in the base directory of the repo
		_, _, err := e2e.Zarf("prepare", "lint")
		require.NoError(t, err, "Expect no error here because the yaml file is following schema")
	})

	t.Run("zarf test lint fail", func(t *testing.T) {
		t.Log("E2E: Test lint on schema fail")

		path := filepath.Join("src", "test", "packages", "12-lint")
		configPath := filepath.Join(path, "zarf-config.toml")
		os.Setenv("ZARF_CONFIG", configPath)
		// In this case I'm guessing we should also remove color from the table?
		_, stderr, err := e2e.Zarf("prepare", "lint", path)
		require.Error(t, err, "Require an exit code since there was warnings / errors")

		// It's a bit weird to have a period here and not in the other warnings
		key := "WHATEVER_IMAGE"
		require.Contains(t, stderr, "There are variables that are unset and won't be evaluated during lint")
		require.Contains(t, stderr, fmt.Sprintf(lang.PkgValidateTemplateDeprecation, key, key, key))
		require.Contains(t, stderr, ".components.[2].repos.[0]: Unpinned repository")
		require.Contains(t, stderr, ".metadata: Additional property description1 is not allowed")
		require.Contains(t, stderr, ".components.[0].import: Additional property not-path is not allowed")
		require.Contains(t, stderr, ".components.[2].images.[4]: Unpinned image")
		require.Contains(t, stderr, ".components.[1].images.[0] linted-import: Unpinned image")
		require.Contains(t, stderr, ".components.[1].images.[2] linted-import: Unpinned image")
	})

}
