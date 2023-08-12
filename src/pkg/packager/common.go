// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"crypto"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/internal/packager/sbom"
	"github.com/defenseunicorns/zarf/src/internal/packager/template"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/mholt/archiver/v3"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/interactive"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/packager/deprecated"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

// Packager is the main struct for managing packages.
type Packager struct {
	cfg            *types.PackagerConfig
	cluster        *cluster.Cluster
	remote         *oci.OrasRemote
	tmp            types.TempPaths
	arch           string
	warnings       []string
	valueTemplate  *template.Values
	hpaModified    bool
	connectStrings types.ConnectStrings
}

// Zarf Packager Variables.
var (
	// Find zarf-packages on the local system (https://regex101.com/r/TUUftK/1)
	ZarfPackagePattern = regexp.MustCompile(`zarf-package[^\s\\\/]*\.tar(\.zst)?$`)

	// Find zarf-init packages on the local system
	ZarfInitPattern = regexp.MustCompile(GetInitPackageName("") + "$")
)

/*
New creates a new package instance with the provided config.

Note: This function creates a tmp directory that should be cleaned up with p.ClearTempPaths().
*/
func New(cfg *types.PackagerConfig) (*Packager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("no config provided")
	}

	if cfg.SetVariableMap == nil {
		cfg.SetVariableMap = make(map[string]*types.ZarfSetVariable)
	}

	if cfg.Pkg.Build.OCIImportedComponents == nil {
		cfg.Pkg.Build.OCIImportedComponents = make(map[string]string)
	}

	var (
		err       error
		pkgConfig = &Packager{
			cfg: cfg,
		}
	)

	if config.CommonOptions.TempDirectory != "" {
		// If the cache directory is within the temp directory, warn the user
		if strings.HasPrefix(config.CommonOptions.CachePath, config.CommonOptions.TempDirectory) {
			message.Warnf("The cache directory (%q) is within the temp directory (%q) and will be removed when the temp directory is cleaned up", config.CommonOptions.CachePath, config.CommonOptions.TempDirectory)
		}
	}

	// Create a temp directory for the package
	if pkgConfig.tmp, err = createPaths(config.CommonOptions.TempDirectory); err != nil {
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

// SetTempDirectory sets the temp directory for the packager.
func (p *Packager) SetTempDirectory(path string) error {
	p.ClearTempPaths()
	tmp, err := createPaths(path)
	if err != nil {
		return fmt.Errorf("unable to create package temp paths: %w", err)
	}
	p.tmp = tmp
	return nil
}

// GetInitPackageName returns the formatted name of the init package.
func GetInitPackageName(arch string) string {
	if arch == "" {
		// No package has been loaded yet so lookup GetArch() with no package info
		arch = config.GetArch()
	}
	return fmt.Sprintf("zarf-init-%s-%s.tar.zst", arch, config.CLIVersion)
}

// GetPackageName returns the formatted name of the package.
func (p *Packager) GetPackageName() string {
	if p.cfg.IsInitConfig {
		return GetInitPackageName(p.arch)
	}

	packageName := p.cfg.Pkg.Metadata.Name
	suffix := "tar.zst"
	if p.cfg.Pkg.Metadata.Uncompressed {
		suffix = "tar"
	}

	packageFileName := fmt.Sprintf("%s%s-%s", config.ZarfPackagePrefix, packageName, p.arch)
	if p.cfg.Pkg.Build.Differential {
		packageFileName = fmt.Sprintf("%s-%s-differential-%s", packageFileName, p.cfg.CreateOpts.DifferentialData.DifferentialPackageVersion, p.cfg.Pkg.Metadata.Version)
	} else if p.cfg.Pkg.Metadata.Version != "" {
		packageFileName = fmt.Sprintf("%s-%s", packageFileName, p.cfg.Pkg.Metadata.Version)
	}

	return fmt.Sprintf("%s.%s", packageFileName, suffix)
}

// GetInitPackageRemote returns the URL for a remote init package for the given architecture
func GetInitPackageRemote(arch string) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", config.GithubProject, config.CLIVersion, GetInitPackageName(arch))
}

// ClearTempPaths removes the temp directory and any files within it.
func (p *Packager) ClearTempPaths() {
	// Remove the temp directory, but don't throw an error if it fails
	_ = os.RemoveAll(p.tmp.Base)
	_ = os.RemoveAll(config.ZarfSBOMDir)
}

func (p *Packager) createOrGetComponentPaths(component types.ZarfComponent) (paths types.ComponentPaths, err error) {
	base := filepath.Join(p.tmp.Components, component.Name)

	err = utils.CreateDirectory(base, 0700)
	if err != nil {
		return paths, err
	}

	paths = types.ComponentPaths{
		Base:           base,
		Temp:           filepath.Join(base, types.TempFolder),
		Files:          filepath.Join(base, types.FilesFolder),
		Charts:         filepath.Join(base, types.ChartsFolder),
		Repos:          filepath.Join(base, types.ReposFolder),
		Manifests:      filepath.Join(base, types.ManifestsFolder),
		DataInjections: filepath.Join(base, types.DataInjectionsFolder),
		Values:         filepath.Join(base, types.ValuesFolder),
	}

	if len(component.Files) > 0 {
		err = utils.CreateDirectory(paths.Files, 0700)
		if err != nil {
			return paths, err
		}
	}

	if len(component.Charts) > 0 {
		err = utils.CreateDirectory(paths.Charts, 0700)
		if err != nil {
			return paths, err
		}
		for _, chart := range component.Charts {
			if len(chart.ValuesFiles) > 0 {
				err = utils.CreateDirectory(paths.Values, 0700)
				if err != nil {
					return paths, err
				}
				break
			}
		}
	}

	if len(component.Repos) > 0 {
		err = utils.CreateDirectory(paths.Repos, 0700)
		if err != nil {
			return paths, err
		}
	}

	if len(component.Manifests) > 0 {
		err = utils.CreateDirectory(paths.Manifests, 0700)
		if err != nil {
			return paths, err
		}
	}

	if len(component.DataInjections) > 0 {
		err = utils.CreateDirectory(paths.DataInjections, 0700)
		if err != nil {
			return paths, err
		}
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

func createPaths(basePath string) (paths types.TempPaths, err error) {
	if basePath == "" {
		basePath, err = utils.MakeTempDir()
		if err != nil {
			return paths, err
		}
	} else {
		if err := utils.CreateDirectory(basePath, 0700); err != nil {
			return paths, fmt.Errorf("unable to create temp directory: %w", err)
		}
	}
	message.Debug("Using temp", basePath)
	paths = types.TempPaths{
		Base: basePath,

		InjectBinary: filepath.Join(basePath, "zarf-injector"),
		SeedImages:   filepath.Join(basePath, "seed-images"),
		Images:       filepath.Join(basePath, "images"),
		Components:   filepath.Join(basePath, config.ZarfComponentsDir),
		SbomTar:      filepath.Join(basePath, config.ZarfSBOMTar),
		Sboms:        filepath.Join(basePath, "sboms"),
		Checksums:    filepath.Join(basePath, config.ZarfChecksumsTxt),
		ZarfYaml:     filepath.Join(basePath, config.ZarfYAML),
		ZarfSig:      filepath.Join(basePath, config.ZarfYAMLSignature),
	}

	return paths, err
}

func getRequestedComponentList(requestedComponents string) []string {
	if requestedComponents != "" {
		split := strings.Split(requestedComponents, ",")
		for idx, component := range split {
			split[idx] = strings.ToLower(strings.TrimSpace(component))
		}
		return split
	}

	return []string{}
}

func (p *Packager) loadZarfPkg() error {
	pathsToCheck, err := p.handlePackagePath()
	if err != nil {
		return fmt.Errorf("unable to handle the provided package path: %w", err)
	}

	alreadyExtracted := p.cfg.PkgOpts.PackagePath == p.tmp.Base

	if alreadyExtracted && pathsToCheck == nil {
		paths, err := utils.RecursiveFileList(p.tmp.Base, nil, false)
		if err != nil {
			return fmt.Errorf("unable to get a list of files in the package: %w", err)
		}
		for _, path := range paths {
			pathsToCheck = append(pathsToCheck, strings.TrimPrefix(path, p.tmp.Base+string(os.PathSeparator)))
		}
	}

	spinner := message.NewProgressSpinner("Loading Zarf Package %s", p.cfg.PkgOpts.PackagePath)
	defer spinner.Stop()

	// Make sure the user gave us a package we can work with
	if utils.InvalidPath(p.cfg.PkgOpts.PackagePath) {
		return fmt.Errorf("unable to find the package at %s", p.cfg.PkgOpts.PackagePath)
	}

	// If packagePath has partial in the name, we need to combine the partials into a single package
	if err := p.handleIfPartialPkg(); err != nil {
		return fmt.Errorf("unable to process partial package: %w", err)
	}

	// If the package was pulled from OCI, there is no need to extract it since it is unpacked already
	if !alreadyExtracted {
		// Extract the archive
		spinner.Updatef("Extracting the package, this may take a few moments")
		if err := archiver.Unarchive(p.cfg.PkgOpts.PackagePath, p.tmp.Base); err != nil {
			return fmt.Errorf("unable to extract the package: %w", err)
		}
	}

	// Load the config from the extracted archive zarf.yaml
	spinner.Updatef("Loading the Zarf package config")
	configPath := p.tmp.ZarfYaml
	if err := p.readYaml(configPath); err != nil {
		return fmt.Errorf("unable to read the zarf.yaml in %s: %w", p.tmp.Base, err)
	}

	// Validate the checksums of all the things!!!
	if err := p.validatePackageChecksums(p.tmp.Base, p.cfg.Pkg.Metadata.AggregateChecksum, pathsToCheck); err != nil {
		return fmt.Errorf("unable to validate the package checksums: %w", err)
	}

	// Get a list of paths for the components of the package
	components, err := os.ReadDir(p.tmp.Components)
	if err != nil {
		return fmt.Errorf("unable to get a list of components... %w", err)
	}
	for _, path := range components {
		// If the components are tarballs, extract them!
		componentPath := filepath.Join(p.tmp.Components, path.Name())
		if !path.IsDir() && strings.HasSuffix(path.Name(), ".tar") {
			if err := archiver.Unarchive(componentPath, p.tmp.Components); err != nil {
				return fmt.Errorf("unable to extract the component: %w", err)
			}

			// After extracting the component, remove the compressed tarball to release disk space
			_ = os.Remove(filepath.Join(p.tmp.Components, path.Name()))
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
		var warnings []string
		p.cfg.Pkg.Components[idx], warnings = deprecated.MigrateComponent(p.cfg.Pkg.Build, component)
		p.warnings = append(p.warnings, warnings...)
	}

	spinner.Success()
	return nil
}

func (p *Packager) handleIfPartialPkg() error {
	message.Debugf("Checking for partial package: %s", p.cfg.PkgOpts.PackagePath)

	// If packagePath has partial in the name, we need to combine the partials into a single package
	if !strings.Contains(p.cfg.PkgOpts.PackagePath, ".part000") {
		message.Debug("No partial package detected")
		return nil
	}

	message.Debug("Partial package detected")

	// Replace part 000 with *
	pattern := strings.Replace(p.cfg.PkgOpts.PackagePath, ".part000", ".part*", 1)
	fileList, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("unable to find partial package files: %s", err)
	}

	// Ensure the files are in order so they are appended in the correct order
	sort.Strings(fileList)

	// Create the new package
	destination := strings.Replace(p.cfg.PkgOpts.PackagePath, ".part000", "", 1)
	pkgFile, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("unable to create new package file: %s", err)
	}
	defer pkgFile.Close()

	// Remove the new package if there is an error
	defer func() {
		// If there is an error, remove the new package
		if p.cfg.PkgOpts.PackagePath != destination {
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
	if shasum, err = utils.GetCryptoHashFromFile(destination, crypto.SHA256); err != nil {
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
	p.cfg.PkgOpts.PackagePath = destination
	return nil
}

// validatePackageArchitecture validates that the package architecture matches the target cluster architecture.
func (p *Packager) validatePackageArchitecture() (err error) {
	// Ignore this check if the package architecture is explicitly "multi"
	if p.arch == "multi" {
		return nil
	}

	// Fetch cluster architecture only if we're already connected to a cluster.
	if p.cluster != nil {
		clusterArch, err := p.cluster.GetArchitecture()
		if err != nil {
			return lang.ErrUnableToCheckArch
		}

		// Check if the package architecture and the cluster architecture are the same.
		if p.arch != clusterArch {
			return fmt.Errorf(lang.CmdPackageDeployValidateArchitectureErr, p.arch, clusterArch)
		}
	}

	return nil
}

// validateLastNonBreakingVersion compares the Zarf CLI version against a package's LastNonBreakingVersion.
// It will return an error if there is an error parsing either of the two versions,
// and will throw a warning if the CLI version is less than the LastNonBreakingVersion.
func (p *Packager) validateLastNonBreakingVersion() (err error) {
	cliVersion := config.CLIVersion
	lastNonBreakingVersion := p.cfg.Pkg.Build.LastNonBreakingVersion

	if lastNonBreakingVersion == "" || cliVersion == "UnknownVersion" {
		return nil
	}

	lastNonBreakingSemVer, err := semver.NewVersion(lastNonBreakingVersion)
	if err != nil {
		return fmt.Errorf("unable to parse lastNonBreakingVersion '%s' from Zarf package build data : %w", lastNonBreakingVersion, err)
	}

	cliSemVer, err := semver.NewVersion(cliVersion)
	if err != nil {
		return fmt.Errorf("unable to parse Zarf CLI version '%s' : %w", cliVersion, err)
	}

	if cliSemVer.LessThan(lastNonBreakingSemVer) {
		warning := fmt.Sprintf(
			lang.CmdPackageDeployValidateLastNonBreakingVersionWarn,
			cliVersion,
			lastNonBreakingVersion,
			lastNonBreakingVersion,
		)
		p.warnings = append(p.warnings, warning)
	}

	return nil
}

var (
	// ErrPkgKeyButNoSig is returned when a key was provided but the package is not signed
	ErrPkgKeyButNoSig = errors.New("a key was provided but the package is not signed - remove the --key flag and run the command again")
	// ErrPkgSigButNoKey is returned when a package is signed but no key was provided
	ErrPkgSigButNoKey = errors.New("package is signed but no key was provided - add a key with the --key flag or use the --insecure flag and run the command again")
)

// ValidatePackageSignature validates the signature of a package
func ValidatePackageSignature(directory string, publicKeyPath string) error {
	var pkg types.ZarfPackage
	if err := utils.ReadYaml(filepath.Join(directory, config.ZarfYAML), &pkg); err != nil {
		return fmt.Errorf("unable to read the package config: %w", err)
	}
	// If the insecure flag was provided, or there is no aggregate checksum, ignore the signature validation
	if config.CommonOptions.Insecure || pkg.Metadata.AggregateChecksum == "" {
		return nil
	}

	// Handle situations where there is no signature within the package
	sigExist := !utils.InvalidPath(filepath.Join(directory, config.ZarfYAMLSignature))
	if !sigExist && publicKeyPath == "" {
		// Nobody was expecting a signature, so we can just return
		return nil
	} else if sigExist && publicKeyPath == "" {
		// The package is signed but no key was provided
		return ErrPkgSigButNoKey
	} else if !sigExist && publicKeyPath != "" {
		// A key was provided but there is no signature
		return ErrPkgKeyButNoSig
	}

	// Validate the signature with the key we were provided
	if err := utils.CosignVerifyBlob(filepath.Join(directory, config.ZarfYAML), filepath.Join(directory, config.ZarfYAMLSignature), publicKeyPath); err != nil {
		return fmt.Errorf("package signature did not match the provided key: %w", err)
	}

	return nil
}

func (p *Packager) getSigCreatePassword(_ bool) ([]byte, error) {
	// CLI flags take priority (also loads from viper configs)
	if p.cfg.CreateOpts.SigningKeyPassword != "" {
		return []byte(p.cfg.CreateOpts.SigningKeyPassword), nil
	}

	return interactive.PromptSigPassword()
}

func (p *Packager) getSigPublishPassword(_ bool) ([]byte, error) {
	// CLI flags take priority (also loads from viper configs)
	if p.cfg.CreateOpts.SigningKeyPassword != "" {
		return []byte(p.cfg.CreateOpts.SigningKeyPassword), nil
	}

	return interactive.PromptSigPassword()
}

func (p *Packager) archiveComponent(component types.ZarfComponent) error {
	componentPath := filepath.Join(p.tmp.Components, component.Name)
	size, err := utils.GetDirSize(componentPath)
	if err != nil {
		return err
	}
	if size > 0 {
		tar := fmt.Sprintf("%s.tar", componentPath)
		message.Debugf("Archiving %s to '%s'", component.Name, tar)
		err := archiver.Archive([]string{componentPath}, tar)
		if err != nil {
			return err
		}
	} else {
		message.Debugf("Component %s is empty, skipping archiving", component.Name)
	}
	return os.RemoveAll(componentPath)
}

func (p *Packager) archivePackage(sourceDir string, destinationTarball string) error {
	spinner := message.NewProgressSpinner("Writing %s to %s", sourceDir, destinationTarball)
	defer spinner.Stop()
	// Make the archive
	archiveSrc := []string{sourceDir + string(os.PathSeparator)}
	if err := archiver.Archive(archiveSrc, destinationTarball); err != nil {
		return fmt.Errorf("unable to create package: %w", err)
	}
	spinner.Updatef("Wrote %s to %s", sourceDir, destinationTarball)

	f, err := os.Stat(destinationTarball)
	if err != nil {
		return fmt.Errorf("unable to read the package archive: %w", err)
	}

	// Convert Megabytes to bytes.
	chunkSize := p.cfg.CreateOpts.MaxPackageSizeMB * 1000 * 1000

	// If a chunk size was specified and the package is larger than the chunk size, split it into chunks.
	if p.cfg.CreateOpts.MaxPackageSizeMB > 0 && f.Size() > int64(chunkSize) {
		spinner.Updatef("Package is larger than %dMB, splitting into multiple files", p.cfg.CreateOpts.MaxPackageSizeMB)
		chunks, sha256sum, err := utils.SplitFile(destinationTarball, chunkSize)
		if err != nil {
			return fmt.Errorf("unable to split the package archive into multiple files: %w", err)
		}
		if len(chunks) > 999 {
			return fmt.Errorf("unable to split the package archive into multiple files: must be less than 1,000 files")
		}

		status := fmt.Sprintf("Package split into %d files, original sha256sum is %s", len(chunks)+1, sha256sum)
		spinner.Updatef(status)
		message.Debug(status)
		_ = os.RemoveAll(destinationTarball)

		// Marshal the data into a json file.
		jsonData, err := json.Marshal(types.ZarfPartialPackageData{
			Count:     len(chunks),
			Bytes:     f.Size(),
			Sha256Sum: sha256sum,
		})
		if err != nil {
			return fmt.Errorf("unable to marshal the partial package data: %w", err)
		}

		// Prepend the json data to the first chunk.
		chunks = append([][]byte{jsonData}, chunks...)

		for idx, chunk := range chunks {
			path := fmt.Sprintf("%s.part%03d", destinationTarball, idx)
			status := fmt.Sprintf("Writing %s", path)
			spinner.Updatef(status)
			message.Debug(status)
			if err := os.WriteFile(path, chunk, 0644); err != nil {
				return fmt.Errorf("unable to write the file %s: %w", path, err)
			}
		}
	}
	spinner.Successf("Package tarball successfully written")
	return nil
}

// SetOCIRemote sets the remote OCI client for the package.
func (p *Packager) SetOCIRemote(url string) error {
	remote, err := oci.NewOrasRemote(url)
	if err != nil {
		return err
	}
	p.remote = remote
	return nil
}
