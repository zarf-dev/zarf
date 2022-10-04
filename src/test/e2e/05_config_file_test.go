package test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/stretchr/testify/require"
)

func TestConfigFile(t *testing.T) {
	t.Log("E2E: Config file")
	e2e.setup(t)
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

	e2e.cleanFiles(path, config)
}

func configFileTests(t *testing.T, dir, path string) {
	_, stdErr, err := e2e.execZarfCommand("package", "create", dir, "--confirm")
	require.NoError(t, err)
	require.Contains(t, string(stdErr), "this is a zebra and they have stripes")
	require.Contains(t, string(stdErr), "this is a leopard and they have spots")

	_, stdErr, err = e2e.execZarfCommand("package", "deploy", path, "--confirm")
	require.NoError(t, err)
	require.Contains(t, string(stdErr), "📦 LION COMPONENT")
	require.NotContains(t, string(stdErr), "📦 LEAPORD COMPONENT")
	require.NotContains(t, string(stdErr), "📦 ZEBRA COMPONENT")
}
