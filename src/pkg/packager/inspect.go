package packager

import (
	"fmt"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/mholt/archiver/v3"
)

// Inspect list the contents of a package
func (p *Packager) Inspect(packageName string, includeSBOM bool) {
	if utils.InvalidPath(packageName) {
		message.Fatalf(nil, "The package archive %s seems to be missing or unreadable.", packageName)
	}

	// Extract the archive
	_ = archiver.Extract(packageName, config.ZarfYAML, p.tmp.Base)

	configPath := filepath.Join(p.tmp.Base, "zarf.yaml")

	// Load the config to get the build version
	if err := p.readYaml(configPath, false); err != nil {
		message.Fatalf(err, "Unable to read %s", p.tmp.Base)
	}

	message.Infof("The package was built with Zarf CLI version %s\n", p.cfg.Pkg.Build.Version)
	utils.ColorPrintYAML(p.cfg.Pkg)

	if includeSBOM {
		err := archiver.Extract(packageName, "sboms", p.tmp.Base)
		if err != nil {
			message.Fatalf(err, "Unable to extract sbom information from the package.")
		}

		sbomViewFiles, _ := filepath.Glob(filepath.Join(p.tmp.Sboms, "sbom-viewer-*"))
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
