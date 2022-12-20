package test

import (
	"fmt"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/stretchr/testify/require"
)

func TestBigBangPackageCreation(t *testing.T) {
	t.Log("E2E: BigBang Package Creation")

	e2e.setup(t)
	defer e2e.teardown(t)

	bbPackage := types.ZarfPackage{}
	utils.ReadYaml("packages/big-bang-core/zarf.yaml", &bbPackage)

	var (
		createPath = "packages/big-bang-core/"
		deployPath = fmt.Sprintf("build/zarf-package-big-bang-core-demo-%s-%s.tar.zst", e2e.arch, bbPackage.Metadata.Version)
	)

	stdOut, stdErr, err := e2e.execZarfCommand("init", "--components=git-server", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	e2e.cleanFiles(deployPath)

	// Create the bigbang package
	stdOut, stdErr, err = e2e.execZarfCommand("package", "create", createPath, "--confirm", "--skip-sbom", "--output-directory", "build/")
	require.NoError(t, err, stdOut, stdErr)

	// Attempt to deploy the bigbang package
	stdOut, stdErr, err = e2e.execZarfCommand("package", "deploy", deployPath, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.execZarfCommand("package", "remove", "big-bang-core-demo", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = e2e.execZarfCommand("package", "remove", "init", "--components=git-server", "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	e2e.cleanFiles(deployPath)
}
