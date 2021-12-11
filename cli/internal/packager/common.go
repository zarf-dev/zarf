package packager

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
	"github.com/sirupsen/logrus"
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

// HandleIfURL If provided package is a URL download it to a temp directory
func HandleIfURL(packagePath string, shasum string, insecureDeploy bool) string {
	// Check if the user gave us a remote package
	providedURL, err := url.Parse(packagePath)
	if err != nil || providedURL.Scheme == "" || providedURL.Host == "" {
		logrus.WithField("archive", packagePath).Debug("The package provided is not a remote package.")
		return packagePath
	}

	if !insecureDeploy && shasum == "" {
		logrus.Fatal("When deploying a remote package you must provide either a `--shasum` or the `--insecure` flag. Neither were provided.")
	}

	// Check the extension on the package is what we expect
	if !isValidFileExtension(providedURL.Path) {
		logrus.Fatalf("Only %s file extensions are permitted.\n", config.GetValidPackageExtensions)
	}

	// Download the package
	resp, err := http.Get(packagePath)
	if err != nil {
		logrus.Fatal("Unable to download the package: ", err)
	}
	defer resp.Body.Close()

	// Write the package to a local file
	tempPath := createPaths()
	localPackagePath := tempPath.base + providedURL.Path
	logrus.Debug("Creating local package with the path: ", localPackagePath)
	packageFile, _ := os.Create(localPackagePath)
	_, err = io.Copy(packageFile, resp.Body)
	if err != nil {
		logrus.Debug(err)
		logrus.Fatal("Unable to copy the contents of the provided URL into a local file.")
	}

	// Check the shasum if necessary
	if !insecureDeploy {
		hasher := sha256.New()
		_, err = io.Copy(hasher, packageFile)
		if err != nil {
			logrus.Debug(err)
			logrus.Fatal("Unable to calculate the sha256 of the provided remote package.")
		}

		value := hex.EncodeToString(hasher.Sum(nil))
		if value != shasum {
			_ = os.Remove(localPackagePath)
			logrus.Fatalf("Provided shasum (%s) of the package did not match what was downloaded (%s)\n", shasum, value)
		}
	}

	return localPackagePath
}

func isValidFileExtension(filename string) bool {
	for _, extension := range config.GetValidPackageExtensions() {
		if strings.HasSuffix(filename, extension) {
			logrus.WithField("packagePath", filename).Warn("Package extension is valid.")
			return true
		}
	}

	return false
}

func loopScriptUntilSuccess(script string, retry bool) {
	logContext := logrus.WithField("script", script)
	logContext.Info("Waiting for script to complete successfully")

	var output string
	var err error

	// Try to patch the zarf binary path in case the name isn't exactly "./zarf"
	binaryPath, err := os.Executable()
	if err != nil {
		logContext.Debug(err)
		logContext.Warn("Unable to determine the current zarf binary path")
	} else {
		script = strings.ReplaceAll(script, "./zarf ", binaryPath+" ")
		// Update since we may have a new parsed script
		logContext = logrus.WithField("script", script)
	}

	// 2 minutes per script (60 * 2 second waits)
	tries := 60
	for {
		tries--
		// If there are no more tries left, drop a warning and continue
		if tries < 1 {
			logContext.Warn("Script failed or timed out")
			logContext.Print(output)
			break
		}
		scriptEnvVars := []string{
			"ZARF_TARGET_ENDPOINT=" + config.GetTargetEndpoint(),
		}
		// Try to silently run the script
		output, err = utils.ExecCommand(false, scriptEnvVars, "sh", "-c", script)
		if err != nil {
			logrus.Debug(err)
			if retry {
				// if retry is enabled, on error wait 2 seconds and try again
				time.Sleep(time.Second * 2)
			} else {
				// No retry, abort
				tries = 0
			}
			continue
		} else {
			// Script successful, output results and continue
			if output != "" {
				logContext.Print(output)
			}
			break
		}
	}
}
