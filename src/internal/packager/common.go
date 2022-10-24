package packager

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/pterm/pterm"
	"gopkg.in/yaml.v2"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

type Package struct {
	config   *PackageConfig
	cluster  *cluster.Cluster
	kube     *k8s.Client
	tempPath types.TempPaths
}

type PackageConfig struct {
	// CreeateOptions tracks the user-defined options used to create the package
	CreateOptions types.ZarfCreateOptions

	// DeployOptions tracks user-defined values for the active deployment
	DeployOptions types.ZarfDeployOptions

	// InitOptions tracks user-defined values for the active Zarf initialization.
	InitOptions types.ZarfInitOptions
}

// NewPackage creates a new package instance with the provided config
func NewPackage(config *PackageConfig) (*Package, error) {
	paths, err := createPaths()
	if err != nil {
		return nil, fmt.Errorf("unable to create package temp paths: %w", err)
	}

	return &Package{config: config, tempPath: paths}, nil
}

// NewPackageOrDie creates a new package instance with the provided config or throws a fatal error
func NewPackageOrDie(config *PackageConfig) *Package {
	pkg, err := NewPackage(config)
	if err != nil {
		message.Fatal(err, "Unable to create package the package")
	}

	return pkg
}

// HandleIfURL If provided package is a URL download it to a temp directory
func (p *Package) HandleIfURL(packagePath string, shasum string, insecureDeploy bool) string {
	// Check if the user gave us a remote package
	providedURL, err := url.Parse(packagePath)
	if err != nil || providedURL.Scheme == "" || providedURL.Host == "" {
		return packagePath
	}

	// Handle case where deploying remote package validated via sget
	if strings.HasPrefix(packagePath, "sget://") {
		return p.handleSgetPackage(packagePath)
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

	localPackagePath := p.tempPath.Base + providedURL.Path
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

	return localPackagePath
}

func (p *Package) handleSgetPackage(sgetPackagePath string) string {

	// Create the local file for the package
	localPackagePath := filepath.Join(p.tempPath.Base, "remote.tar.zst")
	destinationFile, err := os.Create(localPackagePath)
	if err != nil {
		message.Fatal(err, "Unable to create the destination file")
	}
	defer destinationFile.Close()

	// If this is a DefenseUnicorns package, use an internal sget public key
	if strings.HasPrefix(sgetPackagePath, "sget://defenseunicorns") {
		os.Setenv("DU_SGET_KEY", config.SGetPublicKey)
		p.config.DeployOptions.SGetKeyPath = "env://DU_SGET_KEY"
	}

	// Remove the 'sget://' header for the actual sget call
	sgetPackagePath = strings.TrimPrefix(sgetPackagePath, "sget://")

	// Sget the package
	err = utils.Sget(sgetPackagePath, p.config.DeployOptions.SGetKeyPath, destinationFile, context.TODO())
	if err != nil {
		message.Fatal(err, "Unable to get the remote package via sget")
	}

	return localPackagePath
}

func isValidFileExtension(filename string) bool {
	for _, extension := range config.GetValidPackageExtensions() {
		if strings.HasSuffix(filename, extension) {
			return true
		}
	}

	return false
}

func createPaths() (paths types.TempPaths, err error) {
	basePath, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)

	paths = types.TempPaths{
		Base: basePath,

		InjectZarfBinary: filepath.Join(basePath, "zarf-registry"),
		InjectBinary:     filepath.Join(basePath, "zarf-injector"),
		SeedImage:        filepath.Join(basePath, "seed-image.tar"),
		Images:           filepath.Join(basePath, "images.tar"),
		Components:       filepath.Join(basePath, "components"),
		Sboms:            filepath.Join(basePath, "sboms"),
		ZarfYaml:         filepath.Join(basePath, "zarf.yaml"),
	}

	return paths, err
}

func (p *Package) createComponentPaths(component types.ZarfComponent) (paths types.ComponentPaths, err error) {
	basePath := filepath.Join(p.tempPath.Base, component.Name)
	err = utils.CreateDirectory(basePath, 0700)

	paths = types.ComponentPaths{
		Base:           basePath,
		Files:          filepath.Join(basePath, "files"),
		Charts:         filepath.Join(basePath, "charts"),
		Repos:          filepath.Join(basePath, "repos"),
		Manifests:      filepath.Join(basePath, "manifests"),
		DataInjections: filepath.Join(basePath, "data"),
		Values:         filepath.Join(basePath, "values"),
	}

	return paths, err
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
