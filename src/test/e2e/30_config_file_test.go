// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for zarf
package test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/stretchr/testify/require"
)

func TestConfigFile(t *testing.T) {
	t.Log("E2E: Config file")
	e2e.setupWithCluster(t)
	defer e2e.teardown(t)

	var (
		path   = fmt.Sprintf("zarf-package-config-file-%s.tar.zst", e2e.arch)
		dir    = "examples/config-file"
		config = "zarf-config.toml"
	)

	e2e.cleanFiles(path, config)

	// Test the config file environment variable
	os.Setenv("ZARF_CONFIG", filepath.Join(dir, config))
	configFileTests(t, dir, path)
	os.Unsetenv("ZARF_CONFIG")

	// Test the config file auto-discovery
	utils.CreatePathAndCopy(filepath.Join(dir, config), config)
	configFileTests(t, dir, path)

	configFileDefaultTests(t)

	e2e.cleanFiles(path, config)
}

func configFileTests(t *testing.T, dir, path string) {
	stdOut, _, err := e2e.execZarfCommand("package", "create", dir, "--confirm")
	require.NoError(t, err)
	require.Contains(t, string(stdOut), "This is a zebra and they have stripes")
	require.Contains(t, string(stdOut), "This is a leopard and they have spots")

	_, stdErr, err := e2e.execZarfCommand("package", "deploy", path, "--confirm")
	require.NoError(t, err)
	require.Contains(t, string(stdErr), "📦 LION COMPONENT")
	require.NotContains(t, string(stdErr), "📦 LEAPORD COMPONENT")
	require.NotContains(t, string(stdErr), "📦 ZEBRA COMPONENT")

	// Verify the configmap was properly templated
	kubectlOut, _ := exec.Command("kubectl", "-n", "zarf", "get", "configmap", "simple-configmap", "-o", "jsonpath='{.data.templateme\\.properties}' ").Output()
	require.Contains(t, string(kubectlOut), "scorpion=iridescent")
	require.Contains(t, string(kubectlOut), "camel_spider=matte")
}

func configFileDefaultTests(t *testing.T) {

	globalFlags := []string{
		"architecture: 509a38f0",
		"log_level: 6a845a41",
		"Disable log file creation (default true)",
		"Disable fancy UI progress bars, spinners, logos, etc (default true)",
		"zarf_cache: 978499a5",
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
		"regisry.pull_password: b8152e38",
		"registry.pull_username: d0961a97",
		"registry.push_password: 8f58ca41",
		"registry.push_username: 7aab3f6f",
		"registry.secret: 881ae9dd",
		"registry.url: c0ac2e47",
	}

	packageCreateFlags := []string{
		"Allow insecure registry connections when pulling OCI images (default true)",
		"create.output_directory: 52d061d5",
		"Skip generating SBOM for this package (default true)",
		"[thing1=1a2b3c4d]",
	}

	packageDeployFlags := []string{
		"deploy.components: 8d6fde37",
		"Required if deploying a remote package and --shasum is not provided (default true)",
		"deploy.sget: ee7905de",
		"deploy.shasum: 7606fe19",
		"[thing2=2b3c4d5e]",
	}

	// Test remaining default initializers
	os.Setenv("ZARF_CONFIG", filepath.Join("src", "test", "zarf-config-test.toml"))

	// Test global flags
	stdOut, _, _ := e2e.execZarfCommand("--help")
	for _, test := range globalFlags {
		require.Contains(t, string(stdOut), test)
	}

	// Test init flags
	stdOut, _, _ = e2e.execZarfCommand("init", "--help")
	for _, test := range initFlags {
		require.Contains(t, string(stdOut), test)
	}

	// Test package create flags
	stdOut, _, _ = e2e.execZarfCommand("package", "create", "--help")
	for _, test := range packageCreateFlags {
		require.Contains(t, string(stdOut), test)
	}

	// Test package deploy flags
	stdOut, _, _ = e2e.execZarfCommand("package", "deploy", "--help")
	for _, test := range packageDeployFlags {
		require.Contains(t, string(stdOut), test)
	}

	os.Unsetenv("ZARF_CONFIG")
}
