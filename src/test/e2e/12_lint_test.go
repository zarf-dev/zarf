// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/config/lang"
)

// TODO (@AustinAbro321) - remove this test in favor of unit testing lint.Validate
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
		osSetErr := os.Setenv("ZARF_CONFIG", configPath)
		require.NoError(t, osSetErr, "Unable to set ZARF_CONFIG")
		stdout, stderr, err := e2e.Zarf(t, "dev", "lint", testPackagePath, "-f", "good-flavor")
		osUnsetErr := os.Unsetenv("ZARF_CONFIG")
		require.NoError(t, osUnsetErr, "Unable to cleanup ZARF_CONFIG")
		require.Error(t, err, "Require an exit code since there was warnings / errors")
		multiSpaceRegex := regexp.MustCompile(`\s{2,}|\n`)
		strippedStdOut := multiSpaceRegex.ReplaceAllString(stdout, " ")

		key := "WHATEVER_IMAGE"
		require.Contains(t, strippedStdOut, lang.UnsetVarLintWarning)
		require.Contains(t, strippedStdOut, fmt.Sprintf(lang.PkgValidateTemplateDeprecation, key, key, key))
		require.Contains(t, strippedStdOut, ".components.[2].repos.[0] | Unpinned repository")
		require.Contains(t, strippedStdOut, ".metadata | Additional property description1 is not allowed")
		require.Contains(t, strippedStdOut, ".components.[0].import | Additional property not-path is not allowed")
		// Testing the import / compose on lint is working
		require.Contains(t, strippedStdOut, ".components.[1].images.[0] | Image not pinned with digest - registry.com:9001/whatever/image:latest")
		// Testing import / compose + variables are working
		require.Contains(t, strippedStdOut, ".components.[2].images.[3] | Image not pinned with digest - busybox:latest")
		// Testing OCI imports get linted
		require.Contains(t, strippedStdOut, ".components.[0].images.[0] | Image not pinned with digest - ghcr.io/zarf-dev/doom-game:0.0.1")

		// Check flavors
		require.NotContains(t, stdout, "image-in-bad-flavor-component:unpinned")
		require.Contains(t, stdout, "image-in-good-flavor-component:unpinned")

		// Check reported filepaths
		require.Contains(t, stderr, "linting package name=dos-games path=oci://ghcr.io/zarf-dev/packages/dos-games:1.1.0")
		require.Contains(t, stderr, fmt.Sprintf("linting package name=lint path=%s", testPackagePath))
	})
}
