// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying zarf packages
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

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

type Packager struct {
	cfg     *types.PackagerConfig
	cluster *cluster.Cluster
	tmp     types.TempPaths
	arch    string
}

// New creates a new package instance with the provided config.
func New(cfg *types.PackagerConfig) (*Packager, error) {
	message.Debugf("packager.New(%s)", message.JsonValue(cfg))

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

	// Set the arch
	pkgConfig.arch = config.GetArch(cfg.Pkg.Metadata.Architecture, cfg.Pkg.Build.Architecture)

	// Track if this is an init package
	// TODO: We need to read the yaml of the package before 'cfg.Pkg.Kind' will be populated for us to read
	pkgConfig.cfg.IsInitConfig = strings.ToLower(cfg.Pkg.Kind) == "zarfinitconfig"

	return pkgConfig, nil
}

// NewOrDie creates a new package instance with the provided config or throws a fatal error.
func NewOrDie(config *types.PackagerConfig) *Packager {
	message.Debug("packager.NewOrDie()")

	if pkgConfig, err := New(config); err != nil {
		message.Fatal(err, "Unable to create package the package")
		return nil
	} else {
		return pkgConfig
	}
}

// GetInitPackageName returns the formatted name of the init package
func GetInitPackageName(arch string) string {
	message.Debug("packager.GetInitPackageName()")
	if arch == "" {
		arch = config.GetArch()
	}
	return fmt.Sprintf("zarf-init-%s-%s.tar.zst", arch, config.CLIVersion)
}

// GetPackageName returns the formatted name of the package
func (p *Packager) GetPackageName() string {
	message.Debugf("packager.GetPackageName(%s)", message.JsonValue(p))

	if p.cfg.IsInitConfig {
		return GetInitPackageName(p.arch)
	}

	packageName := p.cfg.Pkg.Metadata.Name
	prefix := "zarf-package"
	suffix := "tar.zst"
	if p.cfg.Pkg.Metadata.Uncompressed {
		suffix = "tar"
	}

	if p.cfg.Pkg.Metadata.Version == "" {
		return fmt.Sprintf("%s-%s-%s.%s", prefix, packageName, p.arch, suffix)
	}

	return fmt.Sprintf("%s-%s-%s-%s.%s", prefix, packageName, p.arch, p.cfg.Pkg.Metadata.Version, suffix)
}

// HandleIfURL If provided package is a URL download it to a temp directory
func (p *Packager) HandleIfURL(packagePath, shasum string, insecureDeploy bool) string {
	message.Debugf("packager.HandleIfURL(%s, %s, %t)", packagePath, shasum, insecureDeploy)

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

	localPackagePath := p.tmp.Base + providedURL.Path
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

func (p *Packager) handleSgetPackage(sgetPackagePath string) string {
	message.Debugf("packager.handleSgetPackage(%s)", sgetPackagePath)

	// Create the local file for the package
	localPackagePath := filepath.Join(p.tmp.Base, "remote.tar.zst")
	destinationFile, err := os.Create(localPackagePath)
	if err != nil {
		message.Fatal(err, "Unable to create the destination file")
	}
	defer destinationFile.Close()

	// If this is a DefenseUnicorns package, use an internal sget public key
	if strings.HasPrefix(sgetPackagePath, "sget://defenseunicorns") {
		os.Setenv("DU_SGET_KEY", config.SGetPublicKey)
		p.cfg.DeployOpts.SGetKeyPath = "env://DU_SGET_KEY"
	}

	// Remove the 'sget://' header for the actual sget call
	sgetPackagePath = strings.TrimPrefix(sgetPackagePath, "sget://")

	// Sget the package
	err = utils.Sget(sgetPackagePath, p.cfg.DeployOpts.SGetKeyPath, destinationFile, context.TODO())
	if err != nil {
		message.Fatal(err, "Unable to get the remote package via sget")
	}

	return localPackagePath
}

func (p *Packager) createComponentPaths(component types.ZarfComponent) (paths types.ComponentPaths, err error) {
	message.Debugf("packager.createComponentPaths(%s)", message.JsonValue(component))

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
		SeedImage:    filepath.Join(basePath, "seed-image.tar"),
		Images:       filepath.Join(basePath, "images.tar"),
		Components:   filepath.Join(basePath, "components"),
		Sboms:        filepath.Join(basePath, "sboms"),
		ZarfYaml:     filepath.Join(basePath, "zarf.yaml"),
	}

	return paths, err
}

func getRequestedComponentList(requestedComponents string) []string {
	if requestedComponents != "" {
		return strings.Split(requestedComponents, ",")
	}

	return []string{}
}
