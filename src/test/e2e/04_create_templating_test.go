package test

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/stretchr/testify/require"
)

func TestCreateTemplating(t *testing.T) {
	t.Log("E2E: Temporary directory deploy")

	e2e.setup(t)
	defer e2e.teardown(t)

	// run `zarf package create` with a specified image cache location
	imageCachePath := "/tmp/.image_cache-location"
	decompressPath := "/tmp/.package-decompressed"

	e2e.cleanFiles(imageCachePath, decompressPath)

	// Temporary chdir until #511 is merged
	// TODO: remove this once #511 is merged
	_ = os.Chdir("examples/package-variables")
	tmpBin := fmt.Sprintf("../../%s", e2e.zarfBinPath)
	pkgName := fmt.Sprintf("zarf-package-package-variables-%s.tar.zst", e2e.arch)

	stdOut, stdErr, err := utils.ExecCommandWithContext(context.TODO(), true, tmpBin, "package", "create", "examples/package-variables", "--confirm", "--zarf-cache", imageCachePath)
	require.NoError(t, err, stdOut, stdErr)

	stdOut, stdErr, err = utils.ExecCommandWithContext(context.TODO(), true, tmpBin, "t", "archiver", "decompress", pkgName, decompressPath)
	require.NoError(t, err, stdOut, stdErr)

	file, err := ioutil.ReadFile(decompressPath + "/zarf.yaml")
	require.NoError(t, err)
	// TODO: Test for other permutations of default/not default
	require.Contains(t, string(file), "woof")

	// Reset temp chdir
	_ = os.Chdir("../..")

	e2e.cleanFiles(imageCachePath)
}
