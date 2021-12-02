package packager

import (
	"io/ioutil"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/sirupsen/logrus"
)

type componentPaths struct {
	base            string
	files           string
	charts          string
	values          string
	imagesAppliance string
	imagesGitops    string
	repos           string
	manifests       string
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
		base:            basePath,
		files:           basePath + "/files",
		charts:          basePath + "/charts",
		imagesAppliance: basePath + "/images-component-appliance-" + component.Name + ".tar",
		imagesGitops:    basePath + "/images-component-gitops-" + component.Name + ".tar",
		repos:           basePath + "/repos",
		manifests:       basePath + "/manifests",
		values:          basePath + "/values",
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
		logrus.Info(message + " Zarf package confirmed")
	} else {
		prompt := &survey.Confirm{
			Message: message + " this Zarf package?",
		}
		_ = survey.AskOne(prompt, &confirm)
	}

	return confirm
}

func getValidComponents(allComponents []config.ZarfComponent, requestedComponentNames []string) []config.ZarfComponent {
	var validComponentsList []config.ZarfComponent
	confirmedComponents := make([]bool, len(requestedComponentNames))
	for _, component := range allComponents {
		confirmComponent := component.Required

		// If the component is not required check if the user wants it deployed
		if !confirmComponent {
			// Check if this is one of the components that has been requested
			if len(requestedComponentNames) > 0 {
				for index, requestedComponent := range requestedComponentNames {
					if strings.ToLower(requestedComponent) == component.Name {
						confirmComponent = true
						confirmedComponents[index] = true
					}
				}
			} else {
				// Since no requested components were provided, prompt the user
				prompt := &survey.Confirm{
					Message: "Deploy the " + component.Name + " component?",
					Default: component.Default,
					Help:    component.Description,
				}
				_ = survey.AskOne(prompt, &confirmComponent)
			}
		}

		if confirmComponent {
			validComponentsList = append(validComponentsList, component)
		}
	}

	// Verify that we were able to successfully identify all of the requested components
	var nonMatchedComponents []string
	for requestedComponentIndex, componentMatched := range confirmedComponents {
		if !componentMatched {
			nonMatchedComponents = append(nonMatchedComponents, requestedComponentNames[requestedComponentIndex])
		}
	}

	if len(nonMatchedComponents) > 0 {
		logrus.Fatalf("Unable to find these components to deploy: %v.", nonMatchedComponents)
	}

	return validComponentsList
}
