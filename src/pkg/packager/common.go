// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying zarf packages
package packager

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/types"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
)

// Packager defines a PackageConfig and other information for building and deploying a Zarf Package.
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

	return fmt.Sprintf("%s-%s-%s-%s.%s", prefix, packageName, p.arch, p.cfg.Pkg.Metadata.Version, suffix)
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
