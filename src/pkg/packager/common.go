// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"bufio"
	"crypto"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/internal/packager/sbom"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/mholt/archiver/v3"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager/deprecated"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

// Packager is the main struct for managing packages.
type Packager struct {
	cfg     *types.PackagerConfig
	cluster *cluster.Cluster
	tmp     types.TempPaths
	arch    string
}

/*
New creates a new package instance with the provided config.

Note: This function creates a tmp directory that should be cleaned up with p.ClearTempPaths().
*/
func New(cfg *types.PackagerConfig) (*Packager, error) {
	message.Debugf("packager.New(%s)", message.JSONValue(cfg))

	if cfg == nil {
		return nil, fmt.Errorf("no config provided")
	}

	if cfg.SetVariableMap == nil {
		cfg.SetVariableMap = make(map[string]*types.ZarfSetVariable)
	}

	var (
		err       error
		pkgConfig = &Packager{
			cfg: cfg,
		}
	)

	// Create a temp directory for the package
	if pkgConfig.tmp, err = createPaths(); err != nil {
		return nil, fmt.Errorf("unable to create package temp paths: %w", err)
	}

	return pkgConfig, nil
}

/*
NewOrDie creates a new package instance with the provided config or throws a fatal error.

Note: This function creates a tmp directory that should be cleaned up with p.ClearTempPaths().
*/
func NewOrDie(config *types.PackagerConfig) *Packager {
	var (
		err       error
		pkgConfig *Packager
	)

	if pkgConfig, err = New(config); err != nil {
		message.Fatalf(err, "Unable to setup the package config: %s", err.Error())
	}

	return pkgConfig
}

// GetInitPackageName returns the formatted name of the init package.
func GetInitPackageName(arch string) string {
	message.Debug("packager.GetInitPackageName()")
	if arch == "" {
		// No package has been loaded yet so lookup GetArch() with no package info
		arch = config.GetArch()
	}
	return fmt.Sprintf("zarf-init-%s-%s.tar.zst", arch, config.CLIVersion)
}

// GetPackageName returns the formatted name of the package.
func (p *Packager) GetPackageName() string {
	message.Debugf("packager.GetPackageName(%s)", message.JSONValue(p))

	if p.cfg.IsInitConfig {
		return GetInitPackageName(p.arch)
	}

	packageName := p.cfg.Pkg.Metadata.Name
	suffix := "tar.zst"
	if p.cfg.Pkg.Metadata.Uncompressed {
		suffix = "tar"
	}

	if p.cfg.Pkg.Metadata.Version == "" {
		return fmt.Sprintf("%s%s-%s.%s", config.ZarfPackagePrefix, packageName, p.arch, suffix)
	}

	return fmt.Sprintf("%s%s-%s-%s.%s", config.ZarfPackagePrefix, packageName, p.arch, p.cfg.Pkg.Metadata.Version, suffix)
}

// ClearTempPaths removes the temp directory and any files within it.
func (p *Packager) ClearTempPaths() {
	// Remove the temp directory, but don't throw an error if it fails
	_ = os.RemoveAll(p.tmp.Base)
	_ = os.RemoveAll(config.ZarfSBOMDir)
}

func (p *Packager) getComponentBasePath(component types.ZarfComponent) string {
	return filepath.Join(p.tmp.Components, component.Name)
}

func (p *Packager) createOrGetComponentPaths(component types.ZarfComponent) (paths types.ComponentPaths, err error) {
	message.Debugf("packager.createComponentPaths(%s)", message.JSONValue(component))

	basePath := p.getComponentBasePath(component)

	if _, err = os.Stat(basePath); os.IsNotExist(err) {
		// basePath does not exist
		err = utils.CreateDirectory(basePath, 0700)
	}

	paths = types.ComponentPaths{
		Base:           basePath,
		Temp:           filepath.Join(basePath, "temp"),
		Files:          filepath.Join(basePath, "files"),
		Charts:         filepath.Join(basePath, "charts"),
		Repos:          filepath.Join(basePath, "repos"),
		Manifests:      filepath.Join(basePath, "manifests"),
		DataInjections: filepath.Join(basePath, "data"),
		Values:         filepath.Join(basePath, "values"),
	}

	return paths, err
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
	message.Debug("packager.createPaths()")

	basePath, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	paths = types.TempPaths{
		Base: basePath,

		InjectBinary: filepath.Join(basePath, "zarf-injector"),
		SeedImage:    filepath.Join(basePath, "seed-image"),
		Images:       filepath.Join(basePath, "images"),
		Components:   filepath.Join(basePath, "components"),
		SbomTar:      filepath.Join(basePath, "sboms.tar"),
		Sboms:        filepath.Join(basePath, "sboms"),
		Checksums:    filepath.Join(basePath, "checksums.txt"),
		ZarfYaml:     filepath.Join(basePath, config.ZarfYAML),
		ZarfSig:      filepath.Join(basePath, "zarf.yaml.sig"),
	}

	return paths, err
}

func getRequestedComponentList(requestedComponents string) []string {
	if requestedComponents != "" {
		return strings.Split(requestedComponents, ",")
	}

	return []string{}
}

func (p *Packager) loadZarfPkg() error {

	if err := p.handlePackagePath(); err != nil {
		return fmt.Errorf("unable to handle the provided package path: %w", err)
	}

	spinner := message.NewProgressSpinner("Loading Zarf Package %s", p.cfg.DeployOpts.PackagePath)
	defer spinner.Stop()

	// Make sure the user gave us a package we can work with
	if utils.InvalidPath(p.cfg.DeployOpts.PackagePath) {
		return fmt.Errorf("unable to find the package at %s", p.cfg.DeployOpts.PackagePath)
	}

	// If packagePath has partial in the name, we need to combine the partials into a single package
	if err := p.handleIfPartialPkg(); err != nil {
		return fmt.Errorf("unable to process partial package: %w", err)
	}

	// If the package was pulled from OCI, there is no need to extract it since it is unpacked already
	if p.cfg.DeployOpts.PackagePath != p.tmp.Base {
		// Extract the archive
		spinner.Updatef("Extracting the package, this may take a few moments")
		if err := archiver.Unarchive(p.cfg.DeployOpts.PackagePath, p.tmp.Base); err != nil {
			return fmt.Errorf("unable to extract the package: %w", err)
		}
	}

	// Load the config from the extracted archive zarf.yaml
	spinner.Updatef("Loading the zarf package config")
	configPath := p.tmp.ZarfYaml
	if err := p.readYaml(configPath, true); err != nil {
		return fmt.Errorf("unable to read the zarf.yaml in %s: %w", p.tmp.Base, err)
	}

	// Validate the checksums of all the things!!!
	if p.cfg.Pkg.Metadata.AggregateChecksum != "" {
		if err := p.validatePackageChecksums(); err != nil {
			return fmt.Errorf("unable to validate the package checksums: %w", err)
		}
	}

	// Get a list of paths for the components of the package
	components, err := os.ReadDir(p.tmp.Components)
	if err != nil {
		return fmt.Errorf("unable to get a list of components... %w", err)
	}
	for _, component := range components {
		// If the components are tarballs, extract them!
		componentPath := filepath.Join(p.tmp.Components, component.Name())
		if !component.IsDir() && strings.HasSuffix(component.Name(), ".tar") {
			if err := archiver.Unarchive(componentPath, p.tmp.Components); err != nil {
				return fmt.Errorf("unable to extract the component: %w", err)
			}

			// After extracting the component, remove the compressed tarball to release disk space
			_ = os.Remove(filepath.Join(p.tmp.Components, component.Name()))
		}
	}

	// If a SBOM tar file exist, temporarily place them in the deploy directory
	_, tarErr := os.Stat(p.tmp.SbomTar)
	if tarErr == nil {
		if err = archiver.Unarchive(p.tmp.SbomTar, p.tmp.Sboms); err != nil {
			return fmt.Errorf("unable to extract the sbom data from the component: %w", err)
		}
	}

	p.cfg.SBOMViewFiles, _ = filepath.Glob(filepath.Join(p.tmp.Sboms, "sbom-viewer-*"))
	if err := sbom.OutputSBOMFiles(p.tmp, config.ZarfSBOMDir, ""); err != nil {
		// Don't stop the deployment, let the user decide if they want to continue the deployment
		spinner.Errorf(err, "Unable to process the SBOM files for this package")
	}

	// Handle component configuration deprecations
	for idx, component := range p.cfg.Pkg.Components {
		p.cfg.Pkg.Components[idx] = deprecated.MigrateComponent(p.cfg.Pkg.Build, component)
	}

	spinner.Success()
	return nil
}

func (p *Packager) handleIfPartialPkg() error {
	message.Debugf("Checking for partial package: %s", p.cfg.DeployOpts.PackagePath)

	// If packagePath has partial in the name, we need to combine the partials into a single package
	if !strings.Contains(p.cfg.DeployOpts.PackagePath, ".part000") {
		message.Debug("No partial package detected")
		return nil
	}

	message.Debug("Partial package detected")

	// Replace part 000 with *
	pattern := strings.Replace(p.cfg.DeployOpts.PackagePath, ".part000", ".part*", 1)
	fileList, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("unable to find partial package files: %s", err)
	}

	// Ensure the files are in order so they are appended in the correct order
	sort.Strings(fileList)

	// Create the new package
	destination := strings.Replace(p.cfg.DeployOpts.PackagePath, ".part000", "", 1)
	pkgFile, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("unable to create new package file: %s", err)
	}
	defer pkgFile.Close()

	// Remove the new package if there is an error
	defer func() {
		// If there is an error, remove the new package
		if p.cfg.DeployOpts.PackagePath != destination {
			os.Remove(destination)
		}
	}()

	var pgkData types.ZarfPartialPackageData

	// Loop through the partial packages and append them to the new package
	for idx, file := range fileList {
		// The first file contains metadata about the package
		if idx == 0 {
			var bytes []byte

			if bytes, err = os.ReadFile(file); err != nil {
				return fmt.Errorf("unable to read file %s: %w", file, err)
			}

			if err := json.Unmarshal(bytes, &pgkData); err != nil {
				return fmt.Errorf("unable to unmarshal file %s: %w", file, err)
			}

			count := len(fileList) - 1
			if count != pgkData.Count {
				return fmt.Errorf("package is missing parts, expected %d, found %d", pgkData.Count, count)
			}

			continue
		}

		// Open the file
		f, err := os.Open(file)
		if err != nil {
			return fmt.Errorf("unable to open file %s: %w", file, err)
		}
		defer f.Close()

		// Add the file contents to the package
		if _, err = io.Copy(pkgFile, f); err != nil {
			return fmt.Errorf("unable to copy file %s: %w", file, err)
		}
	}

	var shasum string
	if shasum, err = utils.GetCryptoHash(destination, crypto.SHA256); err != nil {
		return fmt.Errorf("unable to get sha256sum of package: %w", err)
	}

	if shasum != pgkData.Sha256Sum {
		return fmt.Errorf("package sha256sum does not match, expected %s, found %s", pgkData.Sha256Sum, shasum)
	}

	// Remove the partial packages to reduce disk space before extracting
	for _, file := range fileList {
		_ = os.Remove(file)
	}

	// Success, update the package path
	p.cfg.DeployOpts.PackagePath = destination
	return nil
}

func (p *Packager) validatePackageChecksums() error {

	// Run pre-checks to make sure we have what we need to validate the checksums
	_, err := os.Stat(p.tmp.Checksums)
	if err != nil {
		return fmt.Errorf("unable to validate checksums as we are unable to find checksums.txt file within the package")
	}
	if p.cfg.Pkg.Metadata.AggregateChecksum == "" {
		return fmt.Errorf("unable to validate checksums because of missing metadata checksum signature")
	}

	// Create a map of all the files in the package so we can track which files we have processed
	filepathMap := make(map[string]bool)
	err = filepath.Walk(p.tmp.Base, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			filepathMap[path] = false
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Verify the that checksums.txt file matches the aggregated checksum provided
	actualAggregateChecksum, err := utils.GetSHA256OfFile(p.tmp.Checksums)
	if err != nil {
		return fmt.Errorf("unable to get the checksum of the checksums.txt file: %w", err)
	}
	if actualAggregateChecksum != p.cfg.Pkg.Metadata.AggregateChecksum {
		return fmt.Errorf("mismatch on the checksum of the checksums.txt file, the checksums.txt file might have been tampered with")
	}

	// Check off all the files that we can trust given the checksum and signing checks
	filepathMap[p.tmp.Checksums] = true
	filepathMap[p.tmp.ZarfYaml] = true
	filepathMap[p.tmp.ZarfSig] = true

	// Load the contents of the checksums file
	checksumsFile, err := os.Open(filepath.Join(p.tmp.Base, "checksums.txt"))
	if err != nil {
		return err
	}
	defer checksumsFile.Close()

	/* Process every line in the checksums file */
	scanner := bufio.NewScanner(checksumsFile)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		// Separate the checksum from the file path
		strs := strings.Split(scanner.Text(), " ")
		itemPath := strs[1]
		expectedShasum := strs[0]

		actualShasum, err := utils.GetSHA256OfFile(filepath.Join(p.tmp.Base, itemPath))
		if err != nil {
			return err
		}

		if expectedShasum != actualShasum {
			return fmt.Errorf("mismatch on the checksum of the %s file (expected: %s, actual: %s)", itemPath, expectedShasum, actualShasum)
		}

		filepathMap[filepath.Join(p.tmp.Base, itemPath)] = true
	}

	for path, processed := range filepathMap {
		if !processed {
			return fmt.Errorf("the file %s was present in the Zarf package but not specified in the checksums.txt, the package might have been tampered with", path)
		}
	}

	message.Successf("All of the checksums matched!")
	return nil
}

func (p *Packager) validatePackageSignature(publicKeyPath string) error {

	// If the insecure flag was provided, ignore the signature validation
	if config.CommonOptions.Insecure {
		return nil
	}

	// Handle situations where there is no signature within the package
	_, sigCheckErr := os.Stat(p.tmp.ZarfSig)
	if sigCheckErr != nil {
		// Nobody was expecting a signature, so we can just return
		if publicKeyPath == "" {
			return nil
		}

		// We were expecting a signature, but there wasn't one..
		return fmt.Errorf("package is not signed but a key was provided")
	}

	// Validate the signature of the package
	if publicKeyPath == "" {
		return fmt.Errorf("package is signed but no key was provided, using signed packages requires a --key or --insecure flag to continue")
	}

	// Validate the signature with the key we were provided
	if err := utils.CosignVerifyBlob(p.tmp.ZarfYaml, p.tmp.ZarfSig, publicKeyPath); err != nil {
		return fmt.Errorf("package signature did not match the provided key: %w", err)
	}

	return nil
}

func (p *Packager) getSigCreatePassword(_ bool) ([]byte, error) {
	// CLI flags take priority (also loads from viper configs)
	if p.cfg.CreateOpts.SigningKeyPassword != "" {
		return []byte(p.cfg.CreateOpts.SigningKeyPassword), nil
	}

	return promptForSigPassword()
}

func (p *Packager) getSigPublishPassword(_ bool) ([]byte, error) {
	// CLI flags take priority (also loads from viper configs)
	if p.cfg.CreateOpts.SigningKeyPassword != "" {
		return []byte(p.cfg.CreateOpts.SigningKeyPassword), nil
	}

	return promptForSigPassword()
}

func promptForSigPassword() ([]byte, error) {
	var password string

	// If we're in interactive mode, prompt the user for the password to their private key
	if !config.CommonOptions.Confirm {
		prompt := &survey.Password{
			Message: "Private key password (empty for no password): ",
		}
		if err := survey.AskOne(prompt, &password); err != nil {
			return nil, fmt.Errorf("unable to get password for private key: %w", err)
		}
		return []byte(password), nil
	}

	// We are returning a nil error here because purposefully avoiding a password input is a valid use condition
	return nil, nil
}
