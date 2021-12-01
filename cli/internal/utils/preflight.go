package utils

import (
	"os"
	"regexp"
	"runtime"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/sirupsen/logrus"
)

func CheckHostName(hostname string) bool {
	expression := regexp.MustCompile(`^[a-zA-Z0-9\-.]+$`)
	return expression.MatchString(hostname)
}

func IsValidHostName() bool {
	logrus.Info("Preflight check: validating hostname")
	// Quick & dirty character validation instead of a complete RFC validation since the OS is already allowing it
	hostname, err := os.Hostname()

	if err != nil {
		return false
	}

	return CheckHostName(hostname)
}

func IsUserRoot() bool {
	logrus.Info("Preflight check: validating user is root")
	return os.Getuid() == 0
}

func IsAMD64() bool {
	logrus.Info("Preflight check: validating AMD64 arch")
	return runtime.GOARCH == "amd64"
}

func IsLinux() bool {
	logrus.Info("Preflight check: validating os type")
	return runtime.GOOS == "linux"
}

func IsRHEL() bool {
	return !InvalidPath("/etc/redhat-release")
}

func GetValidComponents(allComponents []config.ZarfComponent, requestedComponentNames []string) []config.ZarfComponent {
	validComponentsList := []config.ZarfComponent{}
	confirmedCompoonents := make([]bool, len(requestedComponentNames))
	for _, component := range allComponents {
		confirmComponent := component.Required

		// If the component is not required check if the user wants it deployed
		if !confirmComponent {
			// Check if this is one of the components that has been requested
			if len(requestedComponentNames) > 0 {
				for index, requestedComponent := range requestedComponentNames {
					if strings.ToLower(requestedComponent) == component.Name {
						confirmComponent = true
						confirmedCompoonents[index] = true
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
	nonMatchedComponents := []string{}
	for requestedComponentIndex, componentMatched := range confirmedCompoonents {
		if !componentMatched {
			nonMatchedComponents = append(nonMatchedComponents, requestedComponentNames[requestedComponentIndex])
		}
	}

	if len(nonMatchedComponents) > 0 {
		logrus.Fatalf("Unable to find these components to deploy: %v.", nonMatchedComponents)
	}

	return validComponentsList
}

func RunPreflightChecks() {
	if !IsLinux() {
		logrus.Fatal("This program requires a Linux OS")
	}

	if !IsAMD64() {
		logrus.Fatal("This program currently only runs on AMD64 architectures")
	}

	if !IsUserRoot() {
		logrus.Fatal("You must run this program as root.")
	}

	if !IsValidHostName() {
		logrus.Fatal("Please ensure this hostname is valid according to https://www.ietf.org/rfc/rfc1123.txt.")
	}
}
