package packager

import (
	"io/ioutil"

	"github.com/defenseunicorns/zarf/cli/internal/log"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/mholt/archiver/v3"
)

// Inspect list the contents of a package
func Inspect(packageName string) {
	tempPath := createPaths()

	if utils.InvalidPath(packageName) {
		log.Logger.WithField("archive", packageName).Fatal("The package archive seems to be missing or unreadable.")
	}

	// Extract the archive
	_ = archiver.Extract(packageName, "zarf.yaml", tempPath.base)

	content, err := ioutil.ReadFile(tempPath.base + "/zarf.yaml")
	if err != nil {
		log.Logger.Fatal(err)
	}

	// Convert []byte to string and print to screen
	text := string(content)

	utils.ColorPrintYAML(text)

	cleanup(tempPath)
}
