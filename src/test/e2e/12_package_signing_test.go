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

	// Exposes only the cosign flags whose underlying flow is wired through.
	// Keyless, hardware-key, TSA, and cert-based flags are hidden until later stages
	// wire them.
	t.Run("visible cosign flags on verify", func(t *testing.T) {
		stdOut, stdErr, err := e2e.Zarf(t, "package", "verify", "--help")
		require.NoError(t, err, stdOut, stdErr)
		for _, flag := range []string{
			"--trusted-root",
			"--insecure-ignore-tlog",
			"--rekor-url",
		} {
			require.Contains(t, stdOut, flag, "expected %q in `package verify --help`", flag)
		}
		// Air-gap-safe default must remain enabled.
		require.Regexp(t, `--insecure-ignore-tlog[^\n]*\(default true\)`, stdOut)
	})

	// Hidden-flag assertions match flag-entry lines specifically (leading indent +
	// flag name) so substrings inside other flags' description text don't false-match.
	t.Run("hidden verify flags", func(t *testing.T) {
		stdOut, stdErr, err := e2e.Zarf(t, "package", "verify", "--help")
		require.NoError(t, err, stdOut, stdErr)
		for _, hidden := range []string{
			"bundle", "signature", "rfc3161-timestamp", "new-bundle-format",
			"insecure-ignore-sct", "max-workers",
			"certificate-identity", "certificate-oidc-issuer",
			"certificate-github-workflow-trigger", "certificate-github-workflow-sha",
			"certificate-github-workflow-name", "certificate-github-workflow-repository",
			"certificate-github-workflow-ref",
			"certificate", "certificate-chain", "ca-roots", "ca-intermediates",
			"sk", "slot",
			"timestamp-certificate-chain", "use-signed-timestamps",
			"sct", "private-infrastructure", "experimental-oci11",
		} {
			require.NotRegexp(t, `(?m)^\s+--`+hidden+`( |$)`, stdOut,
				"expected --%s hidden from `package verify --help`", hidden)
		}
	})

	t.Run("hidden sign flags", func(t *testing.T) {
		stdOut, stdErr, err := e2e.Zarf(t, "package", "sign", "--help")
		require.NoError(t, err, stdOut, stdErr)
		for _, hidden := range []string{
			"bundle", "output-signature", "output-certificate", "issue-certificate",
			"new-bundle-format",
			"rekor-url", "signing-algorithm",
			"signing-config", "use-signing-config", "trusted-root",
			"fulcio-url", "identity-token", "oidc-issuer", "oidc-client-id",
			"fulcio-auth-flow", "insecure-skip-verify",
			"sk", "slot",
			"timestamp-client-cacert", "timestamp-client-cert", "timestamp-client-key",
			"timestamp-server-name", "timestamp-server-url",
			"certificate", "certificate-chain",
		} {
			require.NotRegexp(t, `(?m)^\s+--`+hidden+`( |$)`, stdOut,
				"expected --%s hidden from `package sign --help`", hidden)
		}
	})
}
