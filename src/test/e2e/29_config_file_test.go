// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
)

func TestConfigFileCreate(t *testing.T) {
	tmpDir := t.TempDir()
	dir := "examples/config-file"

	t.Setenv("ZARF_CONFIG", filepath.Join("src", "test", "zarf-config-config-file.toml"))

	_, _, err := e2e.Zarf(t, "package", "create", dir, "--confirm", "-o", tmpDir)
	require.NoError(t, err)

	tarPath := filepath.Join(tmpDir, fmt.Sprintf("zarf-package-config-file-%s.tar.zst", e2e.Arch))
	pkgLayout, err := layout.LoadFromTar(context.Background(), tarPath, layout.PackageLayoutOptions{})
	require.NoError(t, err)
	require.Equal(t, "This is a zebra and they have stripes", pkgLayout.AsV1alpha1().Components[1].Description)
	require.Equal(t, "This is a leopard and they have spots", pkgLayout.AsV1alpha1().Components[2].Description)

	_, _, err = e2e.Zarf(t, "package", "deploy", tarPath, "--confirm")
	require.NoError(t, err)

	// Verify the configmap was properly templated
	kubectlOut, _, err := e2e.Kubectl(t, "-n", "zarf", "get", "configmap", "simple-configmap", "-o", "jsonpath={.data.templateme\\.properties}")
	require.NoError(t, err)
	require.Contains(t, string(kubectlOut), "scorpion=iridescent")
	require.Contains(t, string(kubectlOut), "camel_spider=matte")

	// verify the multiline dummy CA was properly templated
	tlsCA := `-----BEGIN CERTIFICATE-----
MIIDWjCCAkKgAwIBAgIRAKX5F/lce63a8HD+hATcOw0wDQYJKoZIhvcNAQELBQAw
NzEXMBUGA1UEChMOWmFyZiBDb21tdW5pdHkxHDAaBgNVBAMTE2NhLnByaXZhdGUu
emFyZi5kZXYwHhcNMjYwNzMxMTYzNjEzWhcNMjcwODEwMTYzNjEzWjA3MRcwFQYD
VQQKEw5aYXJmIENvbW11bml0eTEcMBoGA1UEAxMTY2EucHJpdmF0ZS56YXJmLmRl
djCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAKqY0okwGGk755NU//Dx
KRBDrN2bjGJYYyeKQORunCwTduY8ROD6wvE1Cfh+tReQdvvVRsmvgQqJxBHOFy5V
ZEhAmJePtkMvoUA0ZxIe4Hnws5M67WQjsYWSKK+1hzCp/37Y/oVxO0rzn4IFgr0z
cpf2r4zRKgzDFMUOqCvCvDy7loLs9+N/gG8+6PwZk8sozBbFc8HrvmzVrsaJRMf5
HsSomEuMoxZ21G+EqjJmJqoRKR21xjaWALonhECk2q4pbHwiA/uTeyBT/ZD+/i4L
BTP8FCflKBR3/stFRqNbTFeHofN0JR1TPr/RmY3IAc2ybaN3sX7Z/tHmLoj4c+KO
2nkCAwEAAaNhMF8wDgYDVR0PAQH/BAQDAgKEMB0GA1UdJQQWMBQGCCsGAQUFBwMB
BggrBgEFBQcDAjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBSAuqjNxPbA+rUJ
2BSzq+LbuItG7DANBgkqhkiG9w0BAQsFAAOCAQEASrN3E/0t1AP8EeDZElxAetmw
nvo8f4GiewmP780XPIj3OEeAns9TAXvj1MF2l4+v+6DOAhk3HUuHiP6x8zCkdewO
kgf+dDneMrUl2ZIdjNT75zHptwuxxY4E2SBHJ7MDsUE2UeG7vgbi75NFW4mBBh76
VzT/FvsqpINJBRV/AKBTJMxxsrsOQ3t6GdppryL1VLlBeRFBPfev4IJF7/TSexfX
y9/Vzl6ZUm8vmzkpG80Jog9Fqg7LC68imuISqTzsHQ0I/UKQsymvlAsw7kLbxjWf
5EYDmpvaUP8EpGWx6VMxs9LajYwFqmt7+ZE2FWxWxwepL03WC+Lv8R7OWGM4Zg==
-----END CERTIFICATE-----`
	kubectlOut, _, err = e2e.Kubectl(t, "-n", "zarf", "get", "configmap", "simple-configmap", "-o", "jsonpath={.data['tls-ca']}")
	require.NoError(t, err)
	require.Equal(t, tlsCA, kubectlOut)
}

func TestConfigFileDefault(t *testing.T) {
	globalFlags := []string{
		"architecture: 509a38f0",
		"log_level: 6a845a41",
		"zarf_cache: 978499a5",
		"Allow OCI registry connections over HTTP instead of HTTPS. This flag should only be used if you have a specific reason and accept the reduced security posture.",
		"Skip checking server's certificate for validity. This flag should only be used if you have a specific reason and accept the reduced security posture.",
		"tmp_dir: c457359e",
	}

	initFlags := []string{
		"components: 359049b9",
		"storage_class: 9cae917f",
		"git.pull_password: 8522ccca",
		"git.pull_username: 36646dbe",
		"git.push_password: ba00d92d",
		"git.push_username: eb76dca8",
		"git.url: 7c63c1b9",
		"(default 186282)",
		"registry.pull_password: b8152e38",
		"registry.pull_username: d0961a97",
		"registry.push_password: 8f58ca41",
		"registry.push_username: 7aab3f6f",
		"registry.secret: 881ae9dd",
		"registry.url: c0ac2e47",
	}

	packageCreateFlags := []string{
		"create.output: 52d061d5",
		"Skip generating SBOM for this package (default true)",
		"[thing1=1a2b3c4d]",
		"Specify the maximum size of the package in megabytes, packages larger than this will be split into multiple parts to be loaded onto smaller media (i.e. DVDs). Use 0 to disable splitting. (default 42)",
	}

	packageDeployFlags := []string{
		"deploy.components: 8d6fde37",
		"deploy.shasum: 7606fe19",
		"[thing2=2b3c4d5e]",
	}

	// Test remaining default initializers
	t.Setenv("ZARF_CONFIG", filepath.Join("src", "test", "zarf-config-test.toml"))

	// Test global flags
	stdOut, _, err := e2e.Zarf(t, "--help")
	require.NoError(t, err)
	for _, test := range globalFlags {
		require.Contains(t, stdOut, test)
	}

	// Test init flags
	stdOut, _, err = e2e.Zarf(t, "init", "--help")
	require.NoError(t, err)
	for _, test := range initFlags {
		require.Contains(t, stdOut, test)
	}

	// Test package create flags
	stdOut, _, err = e2e.Zarf(t, "package", "create", "--help")
	require.NoError(t, err)
	for _, test := range packageCreateFlags {
		require.Contains(t, stdOut, test)
	}

	// Test package deploy flags
	stdOut, _, err = e2e.Zarf(t, "package", "deploy", "--help")
	require.NoError(t, err)
	for _, test := range packageDeployFlags {
		require.Contains(t, stdOut, test)
	}
}
