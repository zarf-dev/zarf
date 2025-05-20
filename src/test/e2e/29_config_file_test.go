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

	layout2 "github.com/zarf-dev/zarf/src/internal/packager2/layout"
)

func TestConfigFileCreate(t *testing.T) {
	tmpDir := t.TempDir()
	dir := "examples/config-file"

	t.Setenv("ZARF_CONFIG", filepath.Join(dir, "zarf-config.toml"))

	_, _, err := e2e.Zarf(t, "package", "create", dir, "--confirm", "-o", tmpDir)
	require.NoError(t, err)

	tarPath := filepath.Join(tmpDir, fmt.Sprintf("zarf-package-config-file-%s.tar.zst", e2e.Arch))
	pkgLayout, err := layout2.LoadFromTar(context.Background(), tarPath, layout2.PackageLayoutOptions{})
	require.NoError(t, err)
	require.Equal(t, "This is a zebra and they have stripes", pkgLayout.Pkg.Components[1].Description)
	require.Equal(t, "This is a leopard and they have spots", pkgLayout.Pkg.Components[2].Description)
	_, err = pkgLayout.GetSBOM(t.TempDir())
	require.NoError(t, err)

	_, _, err = e2e.Zarf(t, "package", "deploy", tarPath, "--confirm")
	require.NoError(t, err)

	// Verify the configmap was properly templated
	kubectlOut, _, err := e2e.Kubectl(t, "-n", "zarf", "get", "configmap", "simple-configmap", "-o", "jsonpath={.data.templateme\\.properties}")
	require.NoError(t, err)
	require.Contains(t, string(kubectlOut), "scorpion=iridescent")
	require.Contains(t, string(kubectlOut), "camel_spider=matte")

	// verify the multiline dummy private key was properly templated
	tlsKey := `-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDDvKUzWiZucm6/
8D2Nx4KVe8t6uHtARpw112f4yGv7xKcOJkbxLbVtor8pj/HS5tRSZq2ziIQl9y98
8TVAOBezgzPPMDxOqDeyHl5gAtqzpK/eSPmueZIhR88BH2+SMYqa5kxmjn752Rf0
jVeCrVdQ5MD9rqA00oQi/zO+gQQoz6QSuiEQ2pSKYB3gv9oIoJorIU1n4qLYAezn
TvFwjmKWPPhRdyslpcAi1rVO+mVX3Y2DKU/CfpWNFVVT+H788Srn4yP6iWUymfQU
vHOXII1erMnES2H9BDffumrRf3m3IpgueQ3vPhB8ftjFZozURj2t/WSeaKsyQSoZ
Wr99DWxpAgMBAAECggEAAW8ARsACSAzOgtlfmgo8Cpw9gUiYnn/l5P8O4+OT5uQp
1RCytFGBYqwuej9zpffK1k+qNgZp8V0+G8wod6/xfH8Zggr4ZhsVTVirmEhtEaPD
Jf2i1oRNbbD48yknyApU2Y2WQaoJhArzAfeHDI34db83KqR8x+ZC0X7NAjgvr5zS
b0OfY2tht4oxEWh2m67FzlFgF+cWyszRYyfvHfOFBqLesuCnSfMoOzmbT3SlnxHo
6GSa1e/kCJVzFJNb74BZTIH0w6Ar/a0QG829VXivqj8lRENU/1xUI2JhNz4RdH7F
6MeiwQbq4pWjHfh4djuzQFIwOgCnSNRnNuNywOVuAQKBgQDjleEI1XFQawXmHtHu
6GMhbgptRoSUyutDDdo2MHGvDbxDOIsczIBjxCuYAM47nmGMuWbDJUN+2VQAX32J
WZagRxWikxnEqv3B7No7tLSQ42rRo/tDBrZPCCuS9u/ZJM4o7MCa/VzTtbicGOCh
bTIoTeEtT2piIdkrjHFGGlYOLQKBgQDcLNFHrSJCkHfCoz75+zytfYan+2dIxuV/
MlnrT8XHt33cst4ZwoIQbsE6mv7J4CJqOgUYDvoJpioLV3InUACDxXd+bVY7RwxP
j25pXzYL++RctVO3IEOCmFkwlq0fNFdrOn8Y/cnRTwd2e60n08rCKgJS8KhEAaO0
QvVmAHw4rQKBgQDL7hCAnunzuoLFqpZI8tlpKjaTpp3EynO3WSFQb2ZfCvrIbVFS
U/kz7KN3iDlEeO5GcBeiA7EQaGN6FhbiTXHIWwoK7K8paGMMM1V2LL2kGvQruDm8
3LXd6Z9KCJXxSKanS0ZnW2KjnnE3Bp+6ZqOMNATzWfckydnUyPrza0PzXQKBgEYS
1YCUb8Tzqcn+nrp85XDp9INeFh8pfj0fT1L/DpljouEs5Fcaer60ITd/wPuLJCje
0mQ30AhmJBd7+07bvW4y2LcaIUm4cQiZQ7CxpsfloWaIJ16vHA1iY3B9ZBf8Vp4/
/dd8XlEJb/ybnB6C35MwP5EaGtOaGfnzHZsbKG35AoGAWm9tpqhuldQ3MCvoAr5Q
b42JLSKqwpvVjQDiFZPI/0wZTo3WkWm9Rd7CAACheb8S70K1r/JIzsmIcnj0v4xs
sfd+R35UE+m8MExbDP4lKFParmvi2/UZfb3VFNMmMPTV6AEIBl6N4PmhHMZOsIRs
H4RxbE+FpmsMAUCpdrzvFkc=
-----END PRIVATE KEY-----`
	kubectlOut, _, err = e2e.Kubectl(t, "-n", "zarf", "get", "configmap", "simple-configmap", "-o", "jsonpath={.data.tls-key}")
	require.NoError(t, err)
	require.Equal(t, tlsKey, kubectlOut)
}

func TestConfigFileDefault(t *testing.T) {
	globalFlags := []string{
		"architecture: 509a38f0",
		"log_level: 6a845a41",
		"Disable log file creation (default true)",
		"Disable fancy UI progress bars, spinners, logos, etc (default true)",
		"zarf_cache: 978499a5",
		"Force the connections over HTTP instead of HTTPS. This flag should only be used if you have a specific reason and accept the reduced security posture.",
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
		"Between [30000-32767] (default 186282)",
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
