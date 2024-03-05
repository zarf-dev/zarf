// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"slices"

	"github.com/Masterminds/semver/v3"
	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/internal/packager/sbom"
	"github.com/defenseunicorns/zarf/src/internal/packager/template"
	"github.com/defenseunicorns/zarf/src/pkg/cluster"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/mholt/archiver/v3"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/interactive"
	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/packager/deprecated"
	"github.com/defenseunicorns/zarf/src/pkg/packager/sources"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

// Packager is the main struct for managing packages.
type Packager struct {
	cfg            *types.PackagerConfig
	cluster        *cluster.Cluster
	layout         *layout.PackagePaths
	arch           string
	warnings       []string
	valueTemplate  *template.Values
	hpaModified    bool
	connectStrings types.ConnectStrings
	sbomViewFiles  []string
	source         sources.PackageSource
	generation     int
}

// Zarf Packager Variables.
var (
	// Find zarf-packages on the local system (https://regex101.com/r/TUUftK/1)
	ZarfPackagePattern = regexp.MustCompile(`zarf-package[^\s\\\/]*\.tar(\.zst)?$`)

	// Find zarf-init packages on the local system
	ZarfInitPattern = regexp.MustCompile(GetInitPackageName("") + "$")
)

// Modifier is a function that modifies the packager.
type Modifier func(*Packager)

// WithSource sets the source for the packager.
func WithSource(source sources.PackageSource) Modifier {
	return func(p *Packager) {
		p.source = source
	}
}

// WithCluster sets the cluster client for the packager.
func WithCluster(cluster *cluster.Cluster) Modifier {
	return func(p *Packager) {
		p.cluster = cluster
	}
}

// WithTemp sets the temp directory for the packager.
//
// This temp directory is used as the destination where p.source loads the package.
func WithTemp(base string) Modifier {
	return func(p *Packager) {
		p.layout = layout.New(base)
	}
}

/*
New creates a new package instance with the provided config.

Note: This function creates a tmp directory that should be cleaned up with p.ClearTempPaths().
*/
func New(cfg *types.PackagerConfig, mods ...Modifier) (*Packager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("no config provided")
	}

	if cfg.SetVariableMap == nil {
		cfg.SetVariableMap = make(map[string]*types.ZarfSetVariable)
	}

	var (
		err  error
		pkgr = &Packager{
			cfg: cfg,
		}
	)

	if config.CommonOptions.TempDirectory != "" {
		// If the cache directory is within the temp directory, warn the user
		if strings.HasPrefix(config.CommonOptions.CachePath, config.CommonOptions.TempDirectory) {
			message.Warnf("The cache directory (%q) is within the temp directory (%q) and will be removed when the temp directory is cleaned up", config.CommonOptions.CachePath, config.CommonOptions.TempDirectory)
		}
	}

	for _, mod := range mods {
		mod(pkgr)
	}

	// Fill the source if it wasn't provided - note source can be nil if the package is being created
	if pkgr.source == nil && pkgr.cfg.CreateOpts.BaseDir == "" {
		pkgr.source, err = sources.New(&pkgr.cfg.PkgOpts)
		if err != nil {
			return nil, err
		}
	}

	// If the temp directory is not set, set it to the default
	if pkgr.layout == nil {
		if err = pkgr.setTempDirectory(config.CommonOptions.TempDirectory); err != nil {
			return nil, fmt.Errorf("unable to create package temp paths: %w", err)
		}
	}

	return pkgr, nil
}

/*
NewOrDie creates a new package instance with the provided config or throws a fatal error.

Note: This function creates a tmp directory that should be cleaned up with p.ClearTempPaths().
*/
func NewOrDie(config *types.PackagerConfig, mods ...Modifier) *Packager {
	var (
		err  error
		pkgr *Packager
	)

	if pkgr, err = New(config, mods...); err != nil {
		message.Fatalf(err, "Unable to setup the package config: %s", err.Error())
	}

	return pkgr
}

// setTempDirectory sets the temp directory for the packager.
func (p *Packager) setTempDirectory(path string) error {
	dir, err := utils.MakeTempDir(path)
	if err != nil {
		return fmt.Errorf("unable to create package temp paths: %w", err)
	}

	p.layout = layout.New(dir)
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
	if p.isInitConfig() {
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

// ClearTempPaths removes the temp directory and any files within it.
func (p *Packager) ClearTempPaths() {
	// Remove the temp directory, but don't throw an error if it fails
	_ = os.RemoveAll(p.layout.Base)
	_ = os.RemoveAll(layout.SBOMDir)
}

// connectToCluster attempts to connect to a cluster if a connection is not already established
func (p *Packager) connectToCluster(timeout time.Duration) (err error) {
	if p.isConnectedToCluster() {
		return nil
	}

	p.cluster, err = cluster.NewClusterWithWait(timeout)
	if err != nil {
		return err
	}

	return p.attemptClusterChecks()
}

// isConnectedToCluster returns whether the current packager instance is connected to a cluster
func (p *Packager) isConnectedToCluster() bool {
	return p.cluster != nil
}

// isInitConfig returns whether the current packager instance is deploying an init config
func (p *Packager) isInitConfig() bool {
	return p.cfg.Pkg.Kind == types.ZarfInitConfig
}

// hasImages returns whether the current package contains images
func (p *Packager) hasImages() bool {
	for _, component := range p.cfg.Pkg.Components {
		if len(component.Images) > 0 {
			return true
		}
	}
	return false
}

// attemptClusterChecks attempts to connect to the cluster and check for useful metadata and config mismatches.
// NOTE: attemptClusterChecks should only return an error if there is a problem significant enough to halt a deployment, otherwise it should return nil and print a warning message.
func (p *Packager) attemptClusterChecks() (err error) {

	spinner := message.NewProgressSpinner("Gathering additional cluster information (if available)")
	defer spinner.Stop()

	// Check if the package has already been deployed and get its generation
	if existingDeployedPackage, _ := p.cluster.GetDeployedPackage(p.cfg.Pkg.Metadata.Name); existingDeployedPackage != nil {
		// If this package has been deployed before, increment the package generation within the secret
		p.generation = existingDeployedPackage.Generation + 1
	}

	// Check the clusters architecture matches the package spec
	if err := p.validatePackageArchitecture(); err != nil {
		if errors.Is(err, lang.ErrUnableToCheckArch) {
			message.Warnf("Unable to validate package architecture: %s", err.Error())
		} else {
			return err
		}
	}

	// Check for any breaking changes between the initialized Zarf version and this CLI
	if existingInitPackage, _ := p.cluster.GetDeployedPackage("init"); existingInitPackage != nil {
		// Use the build version instead of the metadata since this will support older Zarf versions
		deprecated.PrintBreakingChanges(existingInitPackage.Data.Build.Version)
	}

	spinner.Success()

	return nil
}

// validatePackageArchitecture validates that the package architecture matches the target cluster architecture.
func (p *Packager) validatePackageArchitecture() error {
	// Ignore this check if the architecture is explicitly "multi", we don't have a cluster connection, or the package contains no images
	if p.arch == "multi" || !p.isConnectedToCluster() || !p.hasImages() {
		return nil
	}

	clusterArchitectures, err := p.cluster.GetArchitectures()
	if err != nil {
		return lang.ErrUnableToCheckArch
	}

	// Check if the package architecture and the cluster architecture are the same.
	if !slices.Contains(clusterArchitectures, p.arch) {
		return fmt.Errorf(lang.CmdPackageDeployValidateArchitectureErr, p.arch, strings.Join(clusterArchitectures, ", "))
	}

	return nil
}

// validateLastNonBreakingVersion validates the Zarf CLI version against a package's LastNonBreakingVersion.
func (p *Packager) validateLastNonBreakingVersion() (err error) {
	cliVersion := config.CLIVersion
	lastNonBreakingVersion := p.cfg.Pkg.Build.LastNonBreakingVersion

	if lastNonBreakingVersion == "" {
		return nil
	}

	lastNonBreakingSemVer, err := semver.NewVersion(lastNonBreakingVersion)
	if err != nil {
		return fmt.Errorf("unable to parse lastNonBreakingVersion '%s' from Zarf package build data : %w", lastNonBreakingVersion, err)
	}

	cliSemVer, err := semver.NewVersion(cliVersion)
	if err != nil {
		warning := fmt.Sprintf(lang.CmdPackageDeployInvalidCLIVersionWarn, config.CLIVersion)
		p.warnings = append(p.warnings, warning)
		return nil
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

func (p *Packager) archivePackage(destinationTarball string) error {
	spinner := message.NewProgressSpinner("Writing %s to %s", p.layout.Base, destinationTarball)
	defer spinner.Stop()

	// Make the archive
	archiveSrc := []string{p.layout.Base + string(os.PathSeparator)}
	if err := archiver.Archive(archiveSrc, destinationTarball); err != nil {
		return fmt.Errorf("unable to create package: %w", err)
	}
	spinner.Updatef("Wrote %s to %s", p.layout.Base, destinationTarball)

	fi, err := os.Stat(destinationTarball)
	if err != nil {
		return fmt.Errorf("unable to read the package archive: %w", err)
	}
	spinner.Successf("Package saved to %q", destinationTarball)

	// Convert Megabytes to bytes.
	chunkSize := p.cfg.CreateOpts.MaxPackageSizeMB * 1000 * 1000

	// If a chunk size was specified and the package is larger than the chunk size, split it into chunks.
	if p.cfg.CreateOpts.MaxPackageSizeMB > 0 && fi.Size() > int64(chunkSize) {
		if fi.Size()/int64(chunkSize) > 999 {
			return fmt.Errorf("unable to split the package archive into multiple files: must be less than 1,000 files")
		}
		message.Notef("Package is larger than %dMB, splitting into multiple files", p.cfg.CreateOpts.MaxPackageSizeMB)
		err := utils.SplitFile(destinationTarball, chunkSize)
		if err != nil {
			return fmt.Errorf("unable to split the package archive into multiple files: %w", err)
		}
	}
	return nil
}

func (p *Packager) signPackage(signingKeyPath, signingKeyPassword string) error {
	p.layout = p.layout.AddSignature(signingKeyPath)
	passwordFunc := func(_ bool) ([]byte, error) {
		if signingKeyPassword != "" {
			return []byte(signingKeyPassword), nil
		}
		if !config.CommonOptions.Confirm {
			return interactive.PromptSigPassword()
		}
		return nil, nil
	}
	_, err := utils.CosignSignBlob(p.layout.ZarfYAML, p.layout.Signature, signingKeyPath, passwordFunc)
	if err != nil {
		return fmt.Errorf("unable to sign the package: %w", err)
	}
	return nil
}

func (p *Packager) stageSBOMViewFiles() error {
	if p.layout.SBOMs.IsTarball() {
		return fmt.Errorf("unable to process the SBOM files for this package: %s is a tarball", p.layout.SBOMs.Path)
	}
	// If SBOMs were loaded, temporarily place them in the deploy directory
	sbomDir := p.layout.SBOMs.Path
	if !utils.InvalidPath(sbomDir) {
		p.sbomViewFiles, _ = filepath.Glob(filepath.Join(sbomDir, "sbom-viewer-*"))
		_, err := sbom.OutputSBOMFiles(sbomDir, layout.SBOMDir, "")
		if err != nil {
			// Don't stop the deployment, let the user decide if they want to continue the deployment
			warning := fmt.Sprintf("Unable to process the SBOM files for this package: %s", err.Error())
			p.warnings = append(p.warnings, warning)
		}
	}
	return nil
}
