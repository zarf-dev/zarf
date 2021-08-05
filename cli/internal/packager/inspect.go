package packager

import (
	"io/ioutil"

	"github.com/mholt/archiver/v3"
	"github.com/sirupsen/logrus"
	"repo1.dso.mil/platform-one/big-bang/apps/product-tools/zarf/cli/internal/utils"
)

// Inspect list the contents of a package
func Inspect(packageName string) {
	tempPath := createPaths()

	if utils.InvalidPath(packageName) {
		logrus.WithField("archive", packageName).Fatal("The package archive seems to be missing or unreadable.")
	}

	// Extract the archive
	_ = archiver.Extract(packageName, "config.yaml", tempPath.base)

	content, err := ioutil.ReadFile(tempPath.base + "/config.yaml")
	if err != nil {
		logrus.Fatal(err)
	}

	// Convert []byte to string and print to screen
	text := string(content)

	utils.ColorPrintYAML(text)

	cleanup(tempPath)
}
