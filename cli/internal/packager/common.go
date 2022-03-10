package packager

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/defenseunicorns/zarf/cli/types"

	"github.com/goccy/go-yaml"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/cli/config"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/utils"
)

type componentPaths struct {
	base      string
	files     string
	charts    string
	values    string
	repos     string
	manifests string
}
type tempPaths struct {
	base           string
	seedImages     string
	images         string
	dataInjections string
	components     string
}

func createPaths() tempPaths {
	basePath, _ := utils.MakeTempDir()
	return tempPaths{
		base:           basePath,
		seedImages:     basePath + "/seed-images.tar",
		images:         basePath + "/images.tar",
		dataInjections: basePath + "/data",
		components:     basePath + "/components",
	}
}

func (t tempPaths) clean() {
	message.Debug("Cleaning up temp files")
	_ = os.RemoveAll(t.base)
}

func createComponentPaths(basePath string, component types.ZarfComponent) componentPaths {
	basePath = basePath + "/" + component.Name
	_ = utils.CreateDirectory(basePath, 0700)
	return componentPaths{
		base:      basePath,
		files:     basePath + "/files",
		charts:    basePath + "/charts",
		repos:     basePath + "/repos",
		manifests: basePath + "/manifests",
		values:    basePath + "/values",
	}
}

func confirmAction(configPath string, userMessage string) bool {
	content, err := ioutil.ReadFile(configPath)
	if err != nil {
		message.Fatal(err, "Unable to open the package config file")
	}

	// Convert []byte to string and print to screen
	text := string(content)

	utils.ColorPrintYAML(text)

	// Display prompt if not auto-confirmed
	var confirmFlag bool
	if config.DeployOptions.Confirm {
		message.Infof("%s Zarf package confirmed", userMessage)
		return config.DeployOptions.Confirm
	} else {
		prompt := &survey.Confirm{
			Message: userMessage + " this Zarf package?",
		}
		_ = survey.AskOne(prompt, &confirmFlag)
	}

	return confirmFlag
}

func getValidComponents(allComponents []types.ZarfComponent, requestedComponentNames []string) []types.ZarfComponent {
	var validComponentsList []types.ZarfComponent
	confirmedComponents := make([]bool, len(requestedComponentNames))
	for _, component := range allComponents {
		confirmComponent := component.Required

		// If the component is not required check if the user wants it deployed
		if !confirmComponent {
			// Check if this is one of the components that has been requested
			if len(requestedComponentNames) > 0 || config.DeployOptions.Confirm {
				for index, requestedComponent := range requestedComponentNames {
					if strings.ToLower(requestedComponent) == component.Name {
						confirmComponent = true
						confirmedComponents[index] = true
					}
				}
			} else {
				confirmComponent = ConfirmOptionalComponent(component)
			}
		}

		if confirmComponent {
			validComponentsList = append(validComponentsList, component)
			// Make it easier to know we are running k3s
			if config.IsZarfInitConfig() && component.Name == "k3s" {
				config.DeployOptions.ApplianceMode = true
			}
		}
	}

	// Verify that we were able to successfully identify all the requested components
	var nonMatchedComponents []string
	for requestedComponentIndex, componentMatched := range confirmedComponents {
		if !componentMatched {
			nonMatchedComponents = append(nonMatchedComponents, requestedComponentNames[requestedComponentIndex])
		}
	}

	if len(nonMatchedComponents) > 0 {
		message.Fatalf(nil, "Unable to find these components to deploy: %v.", nonMatchedComponents)
	}

	return validComponentsList
}

// Confirm optional component
func ConfirmOptionalComponent(component types.ZarfComponent) (confirmComponent bool) {
	displayComponent := component
	displayComponent.Description = ""
	content, _ := yaml.Marshal(displayComponent)
	utils.ColorPrintYAML(string(content))
	message.Question(fmt.Sprintf("%s: %s", component.Name, component.Description))

	// Since no requested components were provided, prompt the user
	prompt := &survey.Confirm{
		Message: "Deploy this component?",
		Default: component.Default,
	}
	_ = survey.AskOne(prompt, &confirmComponent)
	return confirmComponent
}

// HandleIfURL If provided package is a URL download it to a temp directory
func HandleIfURL(packagePath string, shasum string, insecureDeploy bool) (string, func()) {
	// Check if the user gave us a remote package
	providedURL, err := url.Parse(packagePath)
	if err != nil || providedURL.Scheme == "" || providedURL.Host == "" {
		return packagePath, func() {}
	}

	if !insecureDeploy && shasum == "" {
		message.Fatal(nil, "When deploying a remote package you must provide either a `--shasum` or the `--insecure` flag. Neither were provided.")
	}

	// Check the extension on the package is what we expect
	if !isValidFileExtension(providedURL.Path) {
		message.Fatalf(nil, "Only %s file extensions are permitted.\n", config.GetValidPackageExtensions())
	}

	// Download the package
	resp, err := http.Get(packagePath)
	if err != nil {
		message.Fatal(err, "Unable to download the package")
	}
	defer resp.Body.Close()

	// Write the package to a local file
	tempPath := createPaths()

	localPackagePath := tempPath.base + providedURL.Path
	message.Debugf("Creating local package with the path: %s", localPackagePath)
	packageFile, _ := os.Create(localPackagePath)
	_, err = io.Copy(packageFile, resp.Body)
	if err != nil {
		message.Fatal(err, "Unable to copy the contents of the provided URL into a local file.")
	}

	// Check the shasum if necessary
	if !insecureDeploy {
		hasher := sha256.New()
		_, err = io.Copy(hasher, packageFile)
		if err != nil {
			message.Fatal(err, "Unable to calculate the sha256 of the provided remote package.")
		}

		value := hex.EncodeToString(hasher.Sum(nil))
		if value != shasum {
			_ = os.Remove(localPackagePath)
			message.Fatalf(nil, "Provided shasum (%s) of the package did not match what was downloaded (%s)\n", shasum, value)
		}
	}

	return localPackagePath, tempPath.clean
}

func isValidFileExtension(filename string) bool {
	for _, extension := range config.GetValidPackageExtensions() {
		if strings.HasSuffix(filename, extension) {
			return true
		}
	}

	return false
}

func loopScriptUntilSuccess(script string, retry bool) {
	spinner := message.NewProgressSpinner("Waiting for command \"%s\"", script)
	defer spinner.Stop()

	// Try to patch the zarf binary path in case the name isn't exactly "./zarf"
	binaryPath, err := os.Executable()
	if err != nil {
		spinner.Errorf(err, "Unable to determine the current zarf binary path")
	} else {
		script = strings.ReplaceAll(script, "./zarf ", binaryPath+" ")
	}

	// 2 minutes per script (60 * 2 second waits)
	tries := 60
	for {
		scriptEnvVars := []string{
			"ZARF_REGISTRY=" + config.ZarfRegistry,
			"ZARF_SEED_REGISTRY=" + config.ZarfLocalSeedRegistry,
		}
		// Try to silently run the script
		output, err := utils.ExecCommand(false, scriptEnvVars, "sh", "-c", script)

		if err != nil {
			message.Debug(err, output)

			if retry {
				tries--

				// If there are no more tries left, we have failed
				if tries < 1 {
					spinner.Fatalf(nil, "Script timed out after 2 minutes")
				} else {
					// if retry is enabled, on error wait 2 seconds and try again
					time.Sleep(time.Second * 2)
					continue
				}
			}

			spinner.Fatalf(nil, "Script failed")
		}

		// Script successful,continue
		message.Debug(output)
		spinner.Success()
		break
	}
}

// removeDuplicates reduces a string slice to unique values only, https://www.dotnetperls.com/duplicates-go
func removeDuplicates(elements []string) []string {
	seen := map[string]bool{}

	// Create a map of all unique elements.
	for v := range elements {
		seen[elements[v]] = true
	}

	// Place all keys from the map into a slice.
	var result []string
	for key := range seen {
		result = append(result, key)
	}
	return result
}
