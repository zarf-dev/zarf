package packager

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/mholt/archiver/v3"
)

// ViewSBOM indicates if image SBOM information should be displayed when inspecting a package
var ViewSBOM bool

// Inspect list the contents of a package
func(p *Package) Inspect(packageName string) {
	if utils.InvalidPath(packageName) {
		message.Fatalf(nil, "The package archive %s seems to be missing or unreadable.", packageName)
	}

	// Extract the archive
	_ = archiver.Extract(packageName, config.ZarfYAML, p.tempPath.Base)

	configPath := filepath.Join(p.tempPath.Base, "zarf.yaml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		message.Fatal(err, "Unable to read the config file in the package")
	}

	// Convert []byte to string and print to screen
	text := string(content)

	utils.ColorPrintYAML(text)

	// Load the config to get the build version
	if err := config.LoadConfig(configPath, false); err != nil {
		message.Fatalf(err, "Unable to read %s", p.tempPath.Base)
	}

	message.Infof("The package was built with Zarf CLI version %s\n", config.GetBuildData().Version)

	if ViewSBOM {
		err = archiver.Extract(packageName, "sboms", p.tempPath.Base)
		if err != nil {
			message.Fatalf(err, "Unable to extract sbom information from the package.")
		}

		sbomViewFiles, _ := filepath.Glob(filepath.Join(p.tempPath.Sboms, "sbom-viewer-*"))
		if len(sbomViewFiles) > 1 {
			link := sbomViewFiles[0]
			msg := fmt.Sprintf("This package has %d images with software bill-of-materials (SBOM) included. You can view them now in the zarf-sbom folder in this directory or to go directly to one, open this in your browser: %s\n\n", len(sbomViewFiles), link)
			message.Note(msg)

			// Use survey.Input to hang until user input
			var value string
			prompt := &survey.Input{
				Message: "Hit the 'enter' key when you are done viewing the SBOM files",
				Default: "",
			}
			_ = survey.AskOne(prompt, &value)
		} else {
			message.Note("There were no images with software bill-of-materials (SBOM) included.")
		}
	}
}
