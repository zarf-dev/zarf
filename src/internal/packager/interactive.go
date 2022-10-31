package packager

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/pterm/pterm"
	"gopkg.in/yaml.v2"
)

func (p *Packager) confirmAction(userMessage string, sbomViewFiles []string) bool {

	content, err := yaml.Marshal(p.cfg)
	if err != nil {
		message.Fatal(err, "Unable to open the package config file")
	}

	// Convert []byte to string and print to screen
	text := string(content)

	pterm.Println()
	utils.ColorPrintYAML(text)

	if len(sbomViewFiles) > 0 {
		cwd, _ := os.Getwd()
		link := filepath.Join(cwd, "zarf-sbom", filepath.Base(sbomViewFiles[0]))
		msg := fmt.Sprintf("This package has %d images with software bill-of-materials (SBOM) included. You can view them now in the zarf-sbom folder in this directory or to go directly to one, open this in your browser: %s\n * This directory will be removed after package deployment.", len(sbomViewFiles), link)
		message.Note(msg)
	}

	pterm.Println()

	// Display prompt if not auto-confirmed
	var confirmFlag bool
	if config.CommonOptions.Confirm {
		message.SuccessF("%s Zarf package confirmed", userMessage)

		return config.CommonOptions.Confirm
	} else {
		prompt := &survey.Confirm{
			Message: userMessage + " this Zarf package?",
		}
		if err := survey.AskOne(prompt, &confirmFlag); err != nil {
			message.Fatalf(nil, "Confirm selection canceled: %s", err.Error())
		}
	}

	return confirmFlag
}
