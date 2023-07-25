// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/oci"
	"github.com/defenseunicorns/zarf/src/pkg/packager"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
)

// Bundler handles bundler operations
type Bundler struct {
	// cfg is the Bundler's configuration options
	cfg *types.BundlerConfig
	// bundle is the bundle's metadata read into memory
	bundle types.ZarfBundle
	// tmp is the temporary directory used by the Bundler cleaned up with ClearPaths()
	tmp string
}

// New creates a new Bundler
func New(cfg *types.BundlerConfig) (*Bundler, error) {
	message.Debugf("bundler.New(%s)", message.JSONValue(cfg))

	if cfg == nil {
		return nil, errors.New("bundler.New() called with nil config")
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

// ValidateBundle validates the bundle
func (b *Bundler) ValidateBundle() error {
	if b.bundle.Metadata.Architecture == "" {
		// ValidateBundle was erroneously called before CalculateBuildInfo
		if err := b.CalculateBuildInfo(); err != nil {
			return err
		}
		if b.bundle.Metadata.Architecture == "" {
			return errors.New("unable to determine architecture")
		}
	}
	if b.bundle.Metadata.Version == "" {
		return fmt.Errorf("%s is missing required field: metadata.version", BundleYAML)
	}
	if b.bundle.Metadata.Name == "" {
		return fmt.Errorf("%s is missing required field: metadata.name", BundleYAML)
	}
	if len(b.bundle.Packages) == 0 {
		return fmt.Errorf("%s is missing required list: packages", BundleYAML)
	}
	// validate access to packages as well as components referenced in the package
	for idx, pkg := range b.bundle.Packages {
		url := fmt.Sprintf("%s:%s-%s", pkg.Repository, pkg.Ref, b.bundle.Metadata.Architecture)

		if strings.Contains(pkg.Ref, "@sha256:") {
			url = fmt.Sprintf("%s:%s", pkg.Repository, pkg.Ref)
		}

		remote, err := oci.NewOrasRemote(url)
		if err != nil {
			return err
		}

		manifestDesc, err := remote.ResolveRoot()
		if err != nil {
			return err
		}

		// mutate the ref to <tag>-<arch>@sha256:<digest> so we can reference it later
		if err := remote.Repo().Reference.ValidateReferenceAsDigest(); err != nil {
			b.bundle.Packages[idx].Ref = pkg.Ref + "-" + b.bundle.Metadata.Architecture + "@sha256:" + manifestDesc.Digest.Encoded()
		}

		message.Debug("Validating package:", message.JSONValue(pkg))

		tmp, err := utils.MakeTempDir("")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)

		if pkg.Repository == "" {
			return fmt.Errorf("%s .packages[%d] is missing required field: repository", BundleYAML, idx)
		}
		if pkg.Ref == "" {
			return fmt.Errorf("%s .packages[%s] is missing required field: ref", BundleYAML, pkg.Repository)
		}

		if _, err := remote.PullPackageMetadata(tmp); err != nil {
			return err
		}

		publicKeyPath := filepath.Join(tmp, "public-key.txt")
		if err := utils.WriteFile(publicKeyPath, []byte(pkg.PublicKey)); err != nil {
			return err
		}

		if err := packager.ValidatePackageSignature(tmp, publicKeyPath); err != nil {
			return err
		}
		if len(pkg.OptionalComponents) > 0 {
			// make sure if a wildcard is given, it is the first and only element
			for idx, component := range pkg.OptionalComponents {
				if (component == "*" && idx != 0) || (component == "*" && len(pkg.OptionalComponents) > 1) {
					return fmt.Errorf("%s .packages[%s].optional-components[%d] wildcard '*' must be first and only item", BundleYAML, pkg.Repository, idx)
				}
			}
			zarfYAML := types.ZarfPackage{}
			zarfYAMLPath := filepath.Join(tmp, config.ZarfYAML)
			err := utils.ReadYaml(zarfYAMLPath, &zarfYAML)
			if err != nil {
				return err
			}
			if pkg.OptionalComponents[0] == "*" {
				// a wildcard has been given, so all optional components will be included
				for _, c := range zarfYAML.Components {
					// TODO: do we even need an arch check here? doesnt zarf only include components for the current arch during publish?
					if c.Only.Cluster.Architecture == "" || c.Only.Cluster.Architecture == b.bundle.Metadata.Architecture {
						pkg.OptionalComponents = append(pkg.OptionalComponents, c.Name)
					}
				}
				// mutate the package to include all optional components
				b.bundle.Packages[idx].OptionalComponents = pkg.OptionalComponents
				continue
			}
			// expand partial wildcards
			// TODO: move this to a `expandWildcards` helper so packager can use it too
			for idx, component := range pkg.OptionalComponents {
				// TODO: move this to helpers
				wildCardToRegexp := func(pattern string) string {
					components := strings.Split(pattern, "*")
					if len(components) == 1 {
						// if len is 1, there are no *'s, return exact match pattern
						return "^" + pattern + "$"
					}
					var result strings.Builder
					for i, literal := range components {

						// Replace * with .*
						if i > 0 {
							result.WriteString(".*")
						}

						// Quote any regular expression meta characters in the
						// literal text.
						result.WriteString(regexp.QuoteMeta(literal))
					}
					return "^" + result.String() + "$"
				}

				// if the component is a partial wildcard, expand it
				if strings.Contains(component, "*") {
					// expand the wildcard
					components := helpers.Filter(zarfYAML.Components, func(c types.ZarfComponent) bool {
						return regexp.MustCompile(wildCardToRegexp(component)).MatchString(c.Name)
					})
					// add the expanded components to the optional components list
					for _, c := range components {
						if c.Only.Cluster.Architecture == "" || c.Only.Cluster.Architecture == b.bundle.Metadata.Architecture {
							pkg.OptionalComponents = append(pkg.OptionalComponents[:idx], append([]string{c.Name}, pkg.OptionalComponents[idx:]...)...)
						}
					}
					b.bundle.Packages[idx].OptionalComponents = pkg.OptionalComponents
				}
			}
			// validate the optional components exist in the package and support the bundle's target architecture
			for _, component := range pkg.OptionalComponents {
				c := helpers.Find(zarfYAML.Components, func(c types.ZarfComponent) bool {
					return c.Name == component
				})
				// make sure the component exists
				if c.Name == "" {
					return fmt.Errorf("%s .packages[%s].components[%s] does not exist in upstream: %s", BundleYAML, pkg.Repository, component, url)
				}
				// make sure the component supports the bundle's target architecture
				if c.Only.Cluster.Architecture != "" && c.Only.Cluster.Architecture != b.bundle.Metadata.Architecture {
					return fmt.Errorf("%s .packages[%s].components[%s] does not support architecture: %s", BundleYAML, pkg.Repository, component, b.bundle.Metadata.Architecture)
				}
			}
			sort.Strings(pkg.OptionalComponents)
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

	// --architecture flag > metadata.arch > build.arch / runtime.GOARCH (default)
	b.bundle.Build.Architecture = config.GetArch(b.bundle.Metadata.Architecture, b.bundle.Build.Architecture)
	b.bundle.Metadata.Architecture = b.bundle.Build.Architecture

	b.bundle.Build.Timestamp = now.Format(time.RFC1123Z)

	b.bundle.Build.Version = config.CLIVersion

	return nil
}

// ValidateBundleSignature validates the bundle signature
func ValidateBundleSignature(bundleYAMLPath, signaturePath, publicKey string) error {
	if utils.InvalidPath(bundleYAMLPath) {
		return fmt.Errorf("path for %s at %s does not exist", BundleYAML, bundleYAMLPath)
	}
	// The package is not signed, but a public key was provided
	if utils.InvalidPath(signaturePath) && !utils.InvalidPath(publicKey) {
		return fmt.Errorf("package is not signed, but a public key was provided")
	}
	// The package is signed, but no public key was provided
	if !utils.InvalidPath(signaturePath) && utils.InvalidPath(publicKey) {
		return fmt.Errorf("package is signed, but no public key was provided")
	}

	// The package is signed, and a public key was provided
	return utils.CosignVerifyBlob(bundleYAMLPath, signaturePath, publicKey)
}

// MergeVariables merges the variables from the config file and the CLI
//
// TODO: move this to helpers.MergeAndTransformMap
func MergeVariables(left map[string]string, right map[string]string) map[string]string {
	// Ensure uppercase keys from viper and CLI --set
	leftUpper := helpers.TransformMapKeys(left, strings.ToUpper)
	rightUpper := helpers.TransformMapKeys(right, strings.ToUpper)

	// Merge the viper config file variables and provided CLI flag variables (CLI takes precedence))
	return helpers.MergeMap(leftUpper, rightUpper)
}

// IsValidTarballPath returns true if the path is a valid tarball path to a bundle tarball
func IsValidTarballPath(path string) bool {
	if utils.InvalidPath(path) || utils.IsDir(path) {
		return false
	}
	name := filepath.Base(path)
	if name == "" {
		return false
	}
	if !strings.HasPrefix(name, BundlePrefix) {
		return false
	}
	re := regexp.MustCompile(`^zarf-bundle-.*-.*.tar(.zst)?$`) // TODO: change this during the port
	return re.MatchString(name)
}
