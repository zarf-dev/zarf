package packager

import (
	"fmt"
	"io/ioutil"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/mholt/archiver/v3"
	"github.com/sirupsen/logrus"
)

// Inspect list the contents of a package
func Inspect(packageName string) {
	tempPath := createPaths()

	if utils.InvalidPath(packageName) {
		logrus.WithField("archive", packageName).Fatal("The package archive seems to be missing or unreadable.")
	}

	// Extract the archive
	_ = archiver.Extract(packageName, "zarf.yaml", tempPath.base)

	content, err := ioutil.ReadFile(tempPath.base + "/zarf.yaml")
	if err != nil {
		logrus.Fatal(err)
	}

	// Convert []byte to string and print to screen
	text := string(content)

	utils.ColorPrintYAML(text)

	// Load the config to get the build version
	if err := config.LoadConfig(tempPath.base + "/zarf.yaml"); err != nil {
		logrus.Fatalf("Unable to read the zarf.yaml file from %s", tempPath.base)
	}

	fmt.Printf("The package was built with Zarf CLI version %s\n", config.GetBuildData().Version)
	cleanup(tempPath)

}
