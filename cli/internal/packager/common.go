package packager

import (
	"io/ioutil"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/log"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
)

type componentPaths struct {
	base      string
	files     string
	charts    string
	images    string
	repos     string
	manifests string
}
type tempPaths struct {
	base           string
	dataInjections string
	components     string
}

func createPaths() tempPaths {
	basePath := utils.MakeTempDir()
	return tempPaths{
		base:           basePath,
		dataInjections: basePath + "/data",
		components:     basePath + "/components",
	}
}

func createComponentPaths(basePath string, component config.ZarfComponent) componentPaths {
	basePath = basePath + "/" + component.Name
	_ = utils.CreateDirectory(basePath, 0700)
	return componentPaths{
		base:      basePath,
		files:     basePath + "/files",
		charts:    basePath + "/charts",
		images:    basePath + "/images-component-" + component.Name + ".tar",
		repos:     basePath + "/repos",
		manifests: basePath + "/manifests",
	}
}

func cleanup(tempPath tempPaths) {
	log.Logger.Info("Cleaning up temp files")
	_ = os.RemoveAll(tempPath.base)
}

func confirmAction(configPath string, confirm bool, message string) bool {
	content, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Logger.Fatal(err)
	}

	// Convert []byte to string and print to screen
	text := string(content)

	utils.ColorPrintYAML(text)

	// Display prompt if not auto-confirmed
	if confirm {
		log.Logger.Info(message + " Zarf package confirmed")
	} else {
		prompt := &survey.Confirm{
			Message: message + " this Zarf package?",
		}
		_ = survey.AskOne(prompt, &confirm)
	}

	return confirm
}
