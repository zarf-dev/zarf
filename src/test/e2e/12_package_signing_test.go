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
		require.Contains(t, stdErr, "no verification material was provided")
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
		require.Contains(t, stdErr, "a key was provided but the package is not signed")
	})

	// Confirms cosign-aligned flag surface is bound on verify. Round-trip keyless verify
	// against real Sigstore fixtures lands in Stage 3 alongside trusted-root embedding.
	t.Run("Cosign-aligned verify flags are accepted", func(t *testing.T) {
		stdOut, stdErr, err := e2e.Zarf(t, "package", "verify", "--help")
		require.NoError(t, err, stdOut, stdErr)
		for _, flag := range []string{
			"--certificate-identity",
			"--certificate-oidc-issuer",
			"--certificate-identity-regexp",
			"--certificate-oidc-issuer-regexp",
			"--certificate-github-workflow-trigger",
			"--trusted-root",
			"--insecure-ignore-tlog",
			"--insecure-ignore-sct",
			"--rekor-url",
		} {
			require.Contains(t, stdOut, flag, "expected %q in `package verify --help`", flag)
		}
		// airgap-safe defaults must remain enabled.
		require.Contains(t, stdOut, "--insecure-ignore-tlog                            ignore transparency log verification, to be used when an artifact signature has not been uploaded to the transparency log. Artifacts cannot be publicly verified when not included in a log (default true)")
		require.Contains(t, stdOut, "--insecure-ignore-sct                             when set, verification will not check that a certificate contains an embedded SCT, a proof of inclusion in a certificate transparency log (default true)")
	})

	t.Run("Cosign-aligned sign flags are accepted", func(t *testing.T) {
		stdOut, stdErr, err := e2e.Zarf(t, "package", "sign", "--help")
		require.NoError(t, err, stdOut, stdErr)
		for _, flag := range []string{
			"--fulcio-url",
			"--rekor-url",
			"--oidc-issuer",
			"--identity-token",
			"--certificate",
			"--certificate-chain",
			"--sk",
			"--slot",
			"--keyless",
			"--signing-config",
			"--use-signing-config",
			"--trusted-root",
		} {
			require.Contains(t, stdOut, flag, "expected %q in `package sign --help`", flag)
		}
	})

	t.Run("Hidden flags do not appear in help", func(t *testing.T) {
		stdOut, stdErr, err := e2e.Zarf(t, "package", "verify", "--help")
		require.NoError(t, err, stdOut, stdErr)
		for _, hidden := range []string{"--bundle", "--signature", "--rfc3161-timestamp"} {
			require.NotContains(t, stdOut, hidden+" string", "expected %q to be hidden in `package verify --help`", hidden)
		}

		stdOut, stdErr, err = e2e.Zarf(t, "package", "sign", "--help")
		require.NoError(t, err, stdOut, stdErr)
		for _, hidden := range []string{"--bundle ", "--output-signature", "--output-certificate", "--issue-certificate"} {
			require.NotContains(t, stdOut, hidden, "expected %q to be hidden in `package sign --help`", hidden)
		}
	})

	t.Run("--keyless lifts the --signing-key requirement", func(t *testing.T) {
		// Without --signing-key and without --keyless: should error with the guard message.
		_, stdErr, err := e2e.Zarf(t, "package", "sign", "nonexistent.tar.zst")
		require.Error(t, err)
		require.Contains(t, stdErr, "--signing-key is required")

		// With --keyless: guard is lifted. The command will fail later for unrelated
		// reasons (no package, no OIDC), but not on the --signing-key check.
		_, stdErr, err = e2e.Zarf(t, "package", "sign", "nonexistent.tar.zst", "--keyless")
		require.Error(t, err)
		require.NotContains(t, stdErr, "--signing-key is required")
	})
}
