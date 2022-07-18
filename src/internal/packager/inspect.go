package packager

import (
	"io/ioutil"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
	"github.com/mholt/archiver/v3"
)

// Inspect list the contents of a package
func Inspect(packageName string) {
	tempPath := createPaths()
	defer tempPath.clean()

	if utils.InvalidPath(packageName) {
		message.Fatalf(nil, "The package archive %s seems to be missing or unreadable.", packageName)
	}

	// Extract the archive
	_ = archiver.Extract(packageName, config.ZarfYAML, tempPath.base)

	configPath := filepath.Join(tempPath.base, "zarf.yaml")
	content, err := ioutil.ReadFile(configPath)
	if err != nil {
		message.Fatal(err, "Unable to read the config file in the package")
	}

	// Convert []byte to string and print to screen
	text := string(content)

	utils.ColorPrintYAML(text)

	// Load the config to get the build version
	if err := config.LoadConfig(configPath, false); err != nil {
		message.Fatalf(err, "Unable to read %s", tempPath.base)
	}

	// Setup the variables in the active config's template
	if err := config.SetActiveVariables(configPath, false); err != nil {
		message.Fatalf(err, "Unable to set variables in template: %s", err.Error())
	}

	message.Infof("The package was built with Zarf CLI version %s\n", config.GetBuildData().Version)
}
