// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

// Bundler handles bundler operations
type Bundler struct {
	// pkgr   packager.Packager
	cfg    *types.BundlerConfig
	bundle types.ZarfBundle
	remote *oci.OrasRemote
	FS     BFS
	// copier oci.Copier
}

// New creates a new Bundler
func New(cfg *types.BundlerConfig) (*Bundler, error) {
	message.Debugf("bundler.New(%s)", message.JSONValue(cfg))

	if cfg == nil {
		return nil, errors.New("bundler.New() called with nil config")
	}

	if cfg.SetVariableMap == nil {
		cfg.SetVariableMap = make(map[string]*types.ZarfSetVariable)
	}

	var (
		err     error
		bundler = &Bundler{
			cfg: cfg,
		}
	)

	if err = bundler.FS.MakeTemp(config.CommonOptions.TempDirectory); err != nil {
		return nil, fmt.Errorf("bundler unable to create temp directory: %w", err)
	}

	return bundler, nil
}

// NewOrDie creates a new Bundler or dies
func NewOrDie(cfg *types.BundlerConfig) *Bundler {
	var (
		err     error
		bundler *Bundler
	)
	if bundler, err = New(cfg); err != nil {
		message.Fatalf(err, "bundler unable to setup, bad config: %s", err.Error())
	}
	return bundler
}

// ClearPaths clears out the paths used by Bundler
func (b *Bundler) ClearPaths() {
	b.FS.ClearPaths()
}

// ValidateBundle validates the bundle
func (b *Bundler) ValidateBundle() error {
	if b.bundle.Metadata.Version == "" {
		return fmt.Errorf("zarf-bundle.yaml is missing required field: metadata.version")
	}
	if b.bundle.Metadata.Name == "" {
		return fmt.Errorf("zarf-bundle.yaml is missing required field: metadata.name")
	}
	if len(b.bundle.Packages) == 0 {
		return fmt.Errorf("zarf-bundle.yaml is missing required list: packages")
	}
	for idx, pkg := range b.bundle.Packages {
		if pkg.Repository == "" {
			return fmt.Errorf("zarf-bundle.yaml .packages[%d] is missing required field: repository", idx)
		}
		if pkg.Ref == "" {
			return fmt.Errorf("zarf-bundle.yaml .packages[%s] is missing required field: ref", pkg.Repository)
		}
		url := fmt.Sprintf("%s:%s", pkg.Repository, pkg.Ref)
		// validate access to packages as well as components referenced in the package
		remote, err := oci.NewOrasRemote(url)
		if err != nil {
			// remote performs access verification upon instantiation
			return err
		}
		err = remote.PullPackageMetadata(b.FS.tmp.Base)
		if err != nil {
			return err
		}
		defer b.FS.ClearPaths()
		// TODO: validate signatures here
		// TODO: are we gonna re-sign the packages within a bundle?
		requestedComponents := pkg.Components
		if len(requestedComponents) > 0 {
			zarfYAML := types.ZarfPackage{}
			err := utils.ReadYaml(b.FS.tmp.ZarfYaml, &zarfYAML)
			if err != nil {
				return err
			}
			for _, component := range requestedComponents {
				c := utils.Find(zarfYAML.Components, func(c types.ZarfComponent) bool {
					return c.Name == component
				})
				if c.Name == "" {
					return fmt.Errorf("zarf-bundle.yaml .packages[%s].components[%s] does not exist in upstream: %s", pkg.Repository, component, url)
				}
			}
		}
	}
	return nil
}

// CalculateBuildInfo calculates the build info for the bundle
//
// this is mainly mirrored from packager.writeYaml()
func (b *Bundler) CalculateBuildInfo() error {
	now := time.Now()

	// Just use $USER env variable to avoid CGO issue.
	// https://groups.google.com/g/golang-dev/c/ZFDDX3ZiJ84.
	// Record the name of the user creating the package.
	if runtime.GOOS == "windows" {
		b.bundle.Build.User = os.Getenv("USERNAME")
	} else {
		b.bundle.Build.User = os.Getenv("USER")
	}

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	b.bundle.Build.Terminal = hostname

	// TODO: investigate the best way forward for determining arch
	b.bundle.Metadata.Architecture = runtime.GOARCH
	b.bundle.Build.Architecture = runtime.GOARCH

	b.bundle.Build.Timestamp = now.Format(time.RFC1123Z)

	b.bundle.Build.Version = config.CLIVersion

	return nil
}

// SetOCIRemote sets the remote for the Bundler
func (b *Bundler) SetOCIRemote(url string) error {
	remote, err := oci.NewOrasRemote(url)
	if err != nil {
		return err
	}
	b.remote = remote
	return nil
}

// MergeVariables merges the variables from the config file and the CLI
func MergeVariables(left map[string]string, right map[string]string) map[string]string {
	// Ensure uppercase keys from viper and CLI --set
	leftUpper := utils.TransformMapKeys(left, strings.ToUpper)
	rightUpper := utils.TransformMapKeys(right, strings.ToUpper)

	// Merge the viper config file variables and provided CLI flag variables (CLI takes precedence))
	return utils.MergeMap(leftUpper, rightUpper)
}
