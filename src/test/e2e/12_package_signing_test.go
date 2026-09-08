// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestPackageSigning(t *testing.T) {
	t.Parallel()
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

		// verify the signed package
		stdOut, stdErr, err = e2e.Zarf(t, "package", "verify", testPath, "--key", filepath.Join("src", "test", "packages", "zarf-test.pub"))
		require.NoError(t, err, stdOut, stdErr)
		require.Contains(t, stdErr, "verification complete")
		require.Contains(t, stdErr, "SUCCESS")
		require.Contains(t, stdErr, "checksum verification")
		require.Contains(t, stdErr, "signature verification")
		require.Contains(t, stdErr, "PASSED")
	})

	t.Run("Verify unsigned package", func(t *testing.T) {
		tmpdir := t.TempDir()
		testCreate := filepath.Join("src", "test", "packages", "12-package-signing")
		testPath := filepath.Join(tmpdir, fmt.Sprintf("zarf-package-basic-signing-%s.tar.zst", e2e.Arch))

		// create package without signing
		stdOut, stdErr, err := e2e.Zarf(t, "package", "create", testCreate, "-o", tmpdir)
		require.NoError(t, err, stdOut, stdErr)

		// verify unsigned package (should fail)
		stdOut, stdErr, err = e2e.Zarf(t, "package", "verify", testPath)
		require.Error(t, err, stdOut, stdErr)
		require.Contains(t, stdErr, "package is not signed - verification cannot be performed")
	})

	t.Run("Verify signed package without key fails", func(t *testing.T) {
		tmpdir := t.TempDir()
		testCreate := filepath.Join("src", "test", "packages", "12-package-signing")
		testPath := filepath.Join(tmpdir, fmt.Sprintf("zarf-package-basic-signing-%s.tar.zst", e2e.Arch))

		// create and sign package
		stdOut, stdErr, err := e2e.Zarf(t, "package", "create", testCreate, "-o", tmpdir)
		require.NoError(t, err, stdOut, stdErr)

		stdOut, stdErr, err = e2e.Zarf(t, "package", "sign", testPath, "--signing-key", filepath.Join("src", "test", "packages", "zarf-test.prv-key"))
		require.NoError(t, err, stdOut, stdErr)

		// try to verify without key (should fail)
		_, stdErr, err = e2e.Zarf(t, "package", "verify", testPath)
		require.Error(t, err)
		require.Contains(t, stdErr, "package was signed with a key; provide --key to verify")
	})

	t.Run("Verify with key but unsigned package fails", func(t *testing.T) {
		tmpdir := t.TempDir()
		testCreate := filepath.Join("src", "test", "packages", "12-package-signing")
		testPath := filepath.Join(tmpdir, fmt.Sprintf("zarf-package-basic-signing-%s.tar.zst", e2e.Arch))

		// create package without signing
		stdOut, stdErr, err := e2e.Zarf(t, "package", "create", testCreate, "-o", tmpdir)
		require.NoError(t, err, stdOut, stdErr)

		// try to verify with key but package is not signed (should fail)
		_, stdErr, err = e2e.Zarf(t, "package", "verify", testPath, "--key", filepath.Join("src", "test", "packages", "zarf-test.pub"))
		require.Error(t, err)
		require.Contains(t, stdErr, "verification material was provided but the package is not signed")
	})

	t.Run("Signing an OCI package without its source prefix", func(t *testing.T) {
		registryURL := testutil.SetupInMemoryRegistryDynamic(testutil.TestContext(t), t)
		source := registryURL + "/implicit-sign-source/simple-package:0.0.1"

		// Create directly in the registry, then sign without the optional oci:// source prefix.
		stdOut, stdErr, err := e2e.Zarf(t, "package", "create", filepath.Join("src", "test", "packages", "11-simple-package"), "-o", "oci://"+registryURL+"/implicit-sign-source", "--plain-http")
		require.NoError(t, err, stdOut, stdErr)

		stdOut, stdErr, err = e2e.Zarf(t, "package", "sign", source, "--signing-key", filepath.Join("src", "test", "packages", "zarf-test.prv-key"), "--plain-http")
		require.NoError(t, err, stdOut, stdErr)

		stdOut, stdErr, err = e2e.Zarf(t, "package", "verify", source, "--key", filepath.Join("src", "test", "packages", "zarf-test.pub"), "--plain-http")
		require.NoError(t, err, stdOut, stdErr)
	})
}
