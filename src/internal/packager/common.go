package packager

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/types"
	"github.com/pterm/pterm"
	"gopkg.in/yaml.v2"

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
	base         string
	injectBinary string
	seedImage    string
	images       string
	components   string
	sboms        string
	zarfYaml     string
}

func createPaths() tempPaths {
	basePath, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		message.Fatalf(err, "Unable to create tmpdir:  %s", config.CommonOptions.TempDirectory)
	}
	return tempPaths{
		base: basePath,

		injectBinary: filepath.Join(basePath, "zarf-injector"),
		seedImage:    filepath.Join(basePath, "seed-image.tar"),
		images:       filepath.Join(basePath, "images.tar"),
		components:   filepath.Join(basePath, "components"),
		sboms:        filepath.Join(basePath, "sboms"),
		zarfYaml:     filepath.Join(basePath, "zarf.yaml"),
	}
}

func (t tempPaths) clean() {
	message.Debug("Cleaning up temp files")
	_ = os.RemoveAll(t.base)
	_ = os.RemoveAll("zarf-sbom")
}

func createComponentPaths(basePath string, component types.ZarfComponent) componentPaths {
	basePath = filepath.Join(basePath, component.Name)
	_ = utils.CreateDirectory(basePath, 0700)
	return componentPaths{
		base:           basePath,
		files:          filepath.Join(basePath, "files"),
		charts:         filepath.Join(basePath, "charts"),
		repos:          filepath.Join(basePath, "repos"),
		manifests:      filepath.Join(basePath, "manifests"),
		dataInjections: filepath.Join(basePath, "data"),
		values:         filepath.Join(basePath, "values"),
	}
}

func confirmAction(userMessage string, sbomViewFiles []string) bool {
	active := config.GetActiveConfig()

	content, err := yaml.Marshal(active)
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

// HandleIfURL If provided package is a URL download it to a temp directory
func HandleIfURL(packagePath string, shasum string, insecureDeploy bool) (string, func()) {
	// Check if the user gave us a remote package
	providedURL, err := url.Parse(packagePath)
	if err != nil || providedURL.Scheme == "" || providedURL.Host == "" {
		return packagePath, func() {}
	}

	// Handle case where deploying remote package validated via sget
	if strings.HasPrefix(packagePath, "sget://") {
		return handleSgetPackage(packagePath)
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

func handleSgetPackage(sgetPackagePath string) (string, func()) {
	// Write the package to a local file in a temp path
	tempPath := createPaths()

	// Create the local file for the package
	localPackagePath := filepath.Join(tempPath.base, "remote.tar.zst")
	destinationFile, err := os.Create(localPackagePath)
	if err != nil {
		message.Fatal(err, "Unable to create the destination file")
	}
	defer destinationFile.Close()

	// If this is a DefenseUnicorns package, use an internal sget public key
	if strings.HasPrefix(sgetPackagePath, "sget://defenseunicorns") {
		os.Setenv("DU_SGET_KEY", config.SGetPublicKey)
		config.DeployOptions.SGetKeyPath = "env://DU_SGET_KEY"
	}

	// Remove the 'sget://' header for the actual sget call
	sgetPackagePath = strings.TrimPrefix(sgetPackagePath, "sget://")

	// Sget the package
	err = utils.Sget(sgetPackagePath, config.DeployOptions.SGetKeyPath, destinationFile, context.TODO())
	if err != nil {
		message.Fatal(err, "Unable to get the remote package via sget")
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

func handlePartialPkg(source string) (destination string, err error) {
	message.Debugf("packager.handlePartialPkg(%s)", source)

	// Replace part 000 with *
	pattern := strings.Replace(source, ".part000", ".part*", 1)
	fileList, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("unable to find partial package files: %s", err)
	}

	// Create the new package
	destination = strings.Replace(source, ".part000", "", 1)
	pkgFile, err := os.Create(destination)
	if err != nil {
		return "", fmt.Errorf("unable to create new package file: %s", err)
	}
	defer pkgFile.Close()

	var pgkData types.ZarfPartialPackageData

	// Loop through the partial packages and append them to the new package
	for idx, file := range fileList {
		// The first file contains metadata about the package
		if idx == 0 {
			var bytes []byte

			if bytes, err = os.ReadFile(file); err != nil {
				return destination, fmt.Errorf("unable to read file %s: %w", file, err)
			}

			if err := json.Unmarshal(bytes, &pgkData); err != nil {
				return destination, fmt.Errorf("unable to unmarshal file %s: %w", file, err)
			}

			count := len(fileList) - 1
			if count != pgkData.Count {
				return destination, fmt.Errorf("package is missing parts, expected %d, found %d", pgkData.Count, count)
			}

			continue
		}

		// Open the file
		f, err := os.Open(file)
		if err != nil {
			return destination, fmt.Errorf("unable to open file %s: %w", file, err)
		}
		defer f.Close()

		// Add the file contents to the package
		if _, err = io.Copy(pkgFile, f); err != nil {
			return destination, fmt.Errorf("unable to copy file %s: %w", file, err)
		}
	}

	var shasum string
	if shasum, err = utils.GetSha256Sum(destination); err != nil {
		return destination, fmt.Errorf("unable to get sha256sum of package: %w", err)
	}

	if shasum != pgkData.Sha256Sum {
		return destination, fmt.Errorf("package sha256sum does not match, expected %s, found %s", pgkData.Sha256Sum, shasum)
	}

	// Remove the partial packages to reduce disk space before extracting
	for _, file := range fileList {
		_ = os.Remove(file)
	}

	return destination, nil
}
