// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/config/lang"
)

func TestLint(t *testing.T) {
	t.Log("E2E: Lint")

	t.Run("zarf test lint success", func(t *testing.T) {
		t.Log("E2E: Test lint on schema success")

		// This runs lint on the zarf.yaml in the base directory of the repo
		_, _, err := e2e.Zarf(t, "dev", "lint")
		require.NoError(t, err, "Expect no error here because the yaml file is following schema")
	})

	t.Run("zarf test lint fail", func(t *testing.T) {
		t.Log("E2E: Test lint on schema fail")

		testPackagePath := filepath.Join("src", "test", "packages", "12-lint")
		configPath := filepath.Join(testPackagePath, "zarf-config.toml")
		os.Setenv("ZARF_CONFIG", configPath)
		_, stderr, err := e2e.Zarf(t, "dev", "lint", testPackagePath, "-f", "good-flavor")
		os.Unsetenv("ZARF_CONFIG")
		require.Error(t, err, "Require an exit code since there was warnings / errors")
		strippedStderr := e2e.StripMessageFormatting(stderr)

		key := "WHATEVER_IMAGE"
		require.Contains(t, strippedStderr, lang.UnsetVarLintWarning)
		require.Contains(t, strippedStderr, fmt.Sprintf(lang.PkgValidateTemplateDeprecation, key, key, key))
		require.Contains(t, strippedStderr, ".components.[2].repos.[0] | Unpinned repository")
		require.Contains(t, strippedStderr, ".metadata | Additional property description1 is not allowed")
		require.Contains(t, strippedStderr, ".components.[0].import | Additional property not-path is not allowed")
		// Testing the import / compose on lint is working
		require.Contains(t, strippedStderr, ".components.[1].images.[0] | Image not pinned with digest - registry.com:9001/whatever/image:latest")
		// Testing import / compose + variables are working
		require.Contains(t, strippedStderr, ".components.[2].images.[3] | Image not pinned with digest - busybox:latest")
		// Testing OCI imports get linted
		require.Contains(t, strippedStderr, ".components.[0].images.[0] | Image not pinned with digest - defenseunicorns/zarf-game:0.0.1")

		// Check flavors
		require.NotContains(t, strippedStderr, "image-in-bad-flavor-component:unpinned")
		require.Contains(t, strippedStderr, "image-in-good-flavor-component:unpinned")

		// Check reported filepaths
		require.Contains(t, strippedStderr, "Linting package \"dos-games\" at oci://🦄/dos-games:1.0.0")
		require.Contains(t, strippedStderr, fmt.Sprintf("Linting package \"lint\" at %s", testPackagePath))
	})
}
