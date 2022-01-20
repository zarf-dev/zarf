package packager

import (
	"io/ioutil"

	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/mholt/archiver/v3"
)

// Inspect list the contents of a package
func Inspect(packageName string) {
	tempPath := createPaths()

	if utils.InvalidPath(packageName) {
		message.Fatalf(nil, "The package archive %s seems to be missing or unreadable.", packageName)
	}

	// Extract the archive
	_ = archiver.Extract(packageName, "zarf.yaml", tempPath.base)

	content, err := ioutil.ReadFile(tempPath.base + "/zarf.yaml")
	if err != nil {
		message.Fatal(err, "Unable to read the config file in the package")
	}

	// Convert []byte to string and print to screen
	text := string(content)

	utils.ColorPrintYAML(text)

	// Load the config to get the build version
	if err := config.LoadConfig(tempPath.base + "/zarf.yaml"); err != nil {
		message.Fatalf(err, "Unable to read %s", tempPath.base)
	}

	message.Infof("The package was built with Zarf CLI version %s\n", config.GetBuildData().Version)
	cleanup(tempPath)

}
