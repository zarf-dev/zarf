package packager

import (
	"os"

	"github.com/sirupsen/logrus"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/utils"
)

type tempPaths struct {
	base           string
	localBin       string
	localCharts    string
	localImage     string
	localManifests string
	remoteImage    string
	remoteRepos    string
}

func createPaths() tempPaths {
	basePath := utils.MakeTempDir()
	return tempPaths{
		base:           basePath,
		localBin:       basePath + "/bin",
		localCharts:    basePath + "/charts",
		localImage:     basePath + "/images-local.tar",
		localManifests: basePath + "/manifests",
		remoteImage:    basePath + "/images-remote.tar",
		remoteRepos:    basePath + "/repos",
	}
}

func cleanup(tempPath tempPaths) {
	logrus.Info("Cleaning up temp files")
	_ = os.RemoveAll(tempPath.base)
}
