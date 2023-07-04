// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
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
	tmp    string
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
		bundler = &Bundler{
			cfg: cfg,
		}
	)

	tmp, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, fmt.Errorf("bundler unable to create temp directory: %w", err)
	}
	bundler.tmp = tmp

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
	_ = os.RemoveAll(b.tmp)
	_ = os.RemoveAll(config.ZarfSBOMDir)
}

// ReadBundleYaml is a wrapper around utils.ReadYaml
func (b *Bundler) ReadBundleYaml(path string, bndl *types.ZarfBundle) error {
	return utils.ReadYaml(path, bndl)
}

// ExtractPackage should extract a package from a bundle
func (b *Bundler) ExtractPackage(name string, out string) error {
	message.Infof("Extracting %s to %s", name, out)
	return nil
	// read the index.json from the bfs.SourceTarball
	// get all the layers for the package
	// extract the layers to the output directory
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
		err = remote.PullPackageMetadata(b.tmp)
		if err != nil {
			return err
		}
		defer b.ClearPaths()
		// TODO: validate signatures here
		// TODO: are we gonna re-sign the packages within a bundle?
		// not re-signing @wayne
		requestedComponents := pkg.Components
		if len(requestedComponents) > 0 {
			zarfYAML := types.ZarfPackage{}
			zarfYAMLPath := filepath.Join(b.tmp, config.ZarfYAML)
			err := utils.ReadYaml(zarfYAMLPath, &zarfYAML)
			if err != nil {
				return err
			}
			for _, component := range requestedComponents {
				// TODO: filter out components from packages before creation that do not match the architecture
				// error or just filter out?
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

// ValidateBundleSignature validates the bundle signature
func (b *Bundler) ValidateBundleSignature(base string) error {
	message.Infof("Validating bundle signature from %s/%s", base, config.ZarfYAMLSignature)
	return nil
	// err := utils.CosignVerifyBlob(bfs.tmp.ZarfBundleYaml, bfs.tmp.ZarfSig, <keypath>)
	// if err != nil {
	// 	return err
	// }
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

	// Set the arch from the package config before filtering.
	b.bundle.Build.Architecture = b.bundle.Metadata.Architecture
	if config.CLIArch != b.bundle.Metadata.Architecture {
		b.bundle.Build.Architecture = config.CLIArch
		b.bundle.Metadata.Architecture = config.CLIArch
	}

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

// IsValidTarballPath returns true if the path is a valid tarball path to a bundle tarball
func IsValidTarballPath(path string) bool {
	if utils.InvalidPath(path) || utils.IsDir(path) {
		return false
	}
	return true // TODO: insert tarball regex here
}

// adapted from p.fillActiveTemplate
func (b *Bundler) templateBundleYaml() error {
	templateMap := map[string]string{}
	setFromCLIConfig := b.cfg.CreateOpts.SetVariables
	yamlTemplates, err := utils.FindYamlTemplates(&b.bundle, "###ZARF_BNDL_TMPL_", "###")
	if err != nil {
		return err
	}

	for key := range yamlTemplates {
		_, present := setFromCLIConfig[key]
		if !present && !config.CommonOptions.Confirm {
			setVal, err := b.promptVariable(types.ZarfPackageVariable{
				Name:    key,
				Default: "",
			})

			if err == nil {
				setFromCLIConfig[key] = setVal
			} else {
				return err
			}
		} else if !present {
			return fmt.Errorf("template '%s' must be '--set' when using the '--confirm' flag", key)
		}
	}
	for key, value := range setFromCLIConfig {
		templateMap[fmt.Sprintf("###ZARF_BNDL_TMPL_%s###", key)] = value
	}

	templateMap["###ZARF_PKG_ARCH###"] = b.bundle.Metadata.Architecture

	return utils.ReloadYamlTemplate(&b.bundle, templateMap)
}

// mirrored from p.promptVariable()
func (b *Bundler) promptVariable(variable types.ZarfPackageVariable) (value string, err error) {

	if variable.Description != "" {
		message.Question(variable.Description)
	}

	prompt := &survey.Input{
		Message: fmt.Sprintf("Please provide a value for \"%s\"", variable.Name),
		Default: variable.Default,
	}

	if err = survey.AskOne(prompt, &value); err != nil {
		return "", err
	}

	return value, nil
}
