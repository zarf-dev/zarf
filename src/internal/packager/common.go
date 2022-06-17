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
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/types"
	"github.com/pterm/pterm"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/defenseunicorns/zarf/src/internal/utils"
)

type componentPaths struct {
	base           string
	files          string
	charts         string
	values         string
	repos          string
	manifests      string
	dataInjections string
}
type tempPaths struct {
	base             string
	injectZarfBinary string
	injectBinary     string
	seedImage        string
	images           string
	components       string
	sboms            string
}

func createPaths() tempPaths {
	basePath, _ := utils.MakeTempDir()
	return tempPaths{
		base: basePath,

		injectZarfBinary: basePath + "/zarf-registry",
		injectBinary:     basePath + "/zarf-injector",
		seedImage:        basePath + "/seed-image.tar",
		images:           basePath + "/images.tar",
		components:       basePath + "/components",
		sboms:            basePath + "/sboms",
	}
}

func (t tempPaths) clean() {
	message.Debug("Cleaning up temp files")
	_ = os.RemoveAll(t.base)
	_ = os.RemoveAll("zarf-sbom")
}

func createComponentPaths(basePath string, component types.ZarfComponent) componentPaths {
	basePath = basePath + "/" + component.Name
	_ = utils.CreateDirectory(basePath, 0700)
	return componentPaths{
		base:           basePath,
		files:          basePath + "/files",
		charts:         basePath + "/charts",
		repos:          basePath + "/repos",
		manifests:      basePath + "/manifests",
		dataInjections: basePath + "/data",
		values:         basePath + "/values",
	}
}

func confirmAction(configPath, userMessage string, sbomViewFiles []string) bool {
	content, err := ioutil.ReadFile(configPath)
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
