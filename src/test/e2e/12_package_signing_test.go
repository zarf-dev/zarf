// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPackageSigning(t *testing.T) {
	t.Log("E2E: Package Signing")

	t.Run("Signing a basic package", func(t *testing.T) {
		// set tmpdir and path to package
		tmpdir := t.TempDir()
		testCreate := filepath.Join("src", "test", "packages", "12-package-signing")
		testPath := filepath.Join(tmpdir, fmt.Sprintf("zarf-package-basic-signing-%s.tar.zst", e2e.Arch))

		// create package without signing
		stdOut, stdErr, err := e2e.Zarf(t, "package", "create", testCreate, "-o", tmpdir)
		require.NoError(t, err, stdOut, stdErr)

		// sign the package
		stdOut, stdErr, err = e2e.Zarf(t, "package", "sign", testPath, "--signing-key", filepath.Join("src", "test", "packages", "zarf-test.prv-key"))
		require.NoError(t, err, stdOut, stdErr)

		// placeholder - inspect the package to ensure verification was successful - replace this with a future verify command
		stdOut, stdErr, err = e2e.Zarf(t, "package", "inspect", "definition", testPath, "--key", filepath.Join("src", "test", "packages", "zarf-test.pub"))
		require.NoError(t, err, stdOut, stdErr)
		require.Contains(t, stdErr, "Verified OK")
		require.Contains(t, stdOut, "signed: true")
	})
}
