// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying zarf packages
package packager

import (
	"fmt"
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

	// Set the arch
	pkgConfig.arch = config.GetArch(cfg.Pkg.Metadata.Architecture, cfg.Pkg.Build.Architecture)

	return pkgConfig, nil
}

/*
NewOrDie creates a new package instance with the provided config or throws a fatal error.

Note: This function creates a tmp directory that should be cleaned up with p.ClearTempPaths().
*/
func NewOrDie(config *types.PackagerConfig) *Packager {
	message.Debug("packager.NewOrDie()")

	if pkgConfig, err := New(config); err != nil {
		message.Fatal(err, "Unable to create the package")
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
	message.Debugf("packager.GetPackageName(%s)", message.JSONValue(p))

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

func (p *Packager) ClearTempPaths() {
	os.RemoveAll(p.tmp.Base)
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
		SeedImage:    filepath.Join(basePath, "seed-image.tar"),
		Images:       filepath.Join(basePath, "images.tar"),
		Components:   filepath.Join(basePath, "components"),
		Sboms:        filepath.Join(basePath, "sboms"),
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
