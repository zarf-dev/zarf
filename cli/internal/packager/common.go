package packager

import (
	"io/ioutil"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/sirupsen/logrus"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/utils"
)

type tempPaths struct {
	base           string
	localFiles     string
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
		localFiles:     basePath + "/files",
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

func confirmAction(configPath string, confirm bool, message string) bool {
	content, err := ioutil.ReadFile(configPath)
	if err != nil {
		logrus.Fatal(err)
	}

	// Convert []byte to string and print to screen
	text := string(content)

	utils.ColorPrintYAML(text)

	// Display prompt if not auto-confirmed
	if confirm {
		logrus.Info(message + "ing Zarf package")
	} else {
		prompt := &survey.Confirm{
			Message: message + " this Zarf package?",
		}
		_ = survey.AskOne(prompt, &confirm)
	}

	return confirm
}
