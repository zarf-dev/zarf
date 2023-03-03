// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"crypto"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
		cfg.SetVariableMap = make(map[string]string)
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
		message.Fatal(err, "Unable to create the package")
	}

	return pkgConfig
}

// GetInitPackageName returns the formatted name of the init package.
func GetInitPackageName(arch string) string {
	message.Debug("packager.GetInitPackageName()")
	if arch == "" {
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

func (p *Packager) createComponentPaths(component types.ZarfComponent) (paths types.ComponentPaths, err error) {
	message.Debugf("packager.createComponentPaths(%s)", message.JSONValue(component))

	basePath := filepath.Join(p.tmp.Components, component.Name)
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
		SbomTar:      filepath.Join(basePath, "sboms.tar.zst"),
		ZarfYaml:     filepath.Join(basePath, config.ZarfYAML),
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
	spinner := message.NewProgressSpinner("Loading Zarf Package %s", p.cfg.DeployOpts.PackagePath)
	defer spinner.Stop()

	if err := p.handlePackagePath(); err != nil {
		return fmt.Errorf("unable to handle the provided package path: %w", err)
	}

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

	// If SBOM files exist, temporarily place them in the deploy directory
	if _, err := os.Stat(filepath.Join(p.tmp.Base, "sboms.tar.zst")); err == nil {
		_ = archiver.Unarchive(filepath.Join(p.tmp.Base, "sboms.tar.zst"), p.tmp.Base)

		p.cfg.SBOMViewFiles, _ = filepath.Glob(filepath.Join(p.tmp.Base, "sbom-viewer-*"))
		if err := sbom.OutputSBOMFiles(p.tmp, config.ZarfSBOMDir, ""); err != nil {
			// Don't stop the deployment, let the user decide if they want to continue the deployment
			spinner.Errorf(err, "Unable to process the SBOM files for this package")
		}
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
