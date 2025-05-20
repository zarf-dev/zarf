// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"context"
	"errors"
	"fmt"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"os"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/layout"
	"github.com/zarf-dev/zarf/src/pkg/message"
	"github.com/zarf-dev/zarf/src/pkg/packager/deprecated"
	"github.com/zarf-dev/zarf/src/pkg/packager/sources"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/variables"
	"github.com/zarf-dev/zarf/src/types"
)

// Packager is the main struct for managing packages.
type Packager struct {
	ctx            context.Context
	cfg            *types.PackagerConfig
	variableConfig *variables.VariableConfig
	state          *types.ZarfState
	cluster        *cluster.Cluster
	layout         *layout.PackagePaths
	hpaModified    bool
	source         sources.PackageSource
}

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

// WithContext sets the packager's context
func WithContext(ctx context.Context) Modifier {
	return func(p *Packager) {
		p.ctx = ctx
	}
}

/*
New creates a new package instance with the provided config.

Note: This function creates a tmp directory that should be cleaned up with p.ClearTempPaths().
*/
func New(cfg *types.PackagerConfig, mods ...Modifier) (*Packager, error) {
	var err error
	if cfg == nil {
		return nil, fmt.Errorf("no config provided")
	}

	pkgr := &Packager{cfg: cfg}
	for _, mod := range mods {
		mod(pkgr)
	}

	l := logger.From(pkgr.ctx)

	pkgr.variableConfig = template.GetZarfVariableConfig(pkgr.ctx)
	cacheDir := config.CommonOptions.CachePath
	tmpDir := config.CommonOptions.TempDirectory
	if config.CommonOptions.TempDirectory != "" {
		// If the cache directory is within the temp directory, warn the user
		if strings.HasPrefix(cacheDir, tmpDir) {
			// TODO(mkcp): Remove message on logger release
			message.Warnf("The cache directory (%q) is within the temp directory (%q) and will be removed when the temp directory is cleaned up", config.CommonOptions.CachePath, config.CommonOptions.TempDirectory)
			l.Warn("the cache directory is within the temp directory and will be removed when the temp directory is cleaned up", "cacheDir", cacheDir, "tmpDir", tmpDir)
		}
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
		dir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
		if err != nil {
			return nil, fmt.Errorf("unable to create package temp paths: %w", err)
		}
		// TODO(mkcp): Remove message on logger release
		message.Debug("Using temporary directory:", dir)
		l.Debug("using temporary directory", "tmpDir", dir)
		pkgr.layout = layout.New(dir)
	}

	return pkgr, nil
}

// ClearTempPaths removes the temp directory and any files within it.
func (p *Packager) ClearTempPaths() {
	// Remove the temp directory
	l := logger.From(p.ctx)
	l.Debug("clearing temp paths",
		"basePath", p.layout.Base,
		"SBOMDir", layout.SBOMDir,
	)
	err := os.RemoveAll(p.layout.Base)
	if err != nil {
		l.Error(err.Error(), "basePath", p.layout.Base)
	}
	err2 := os.RemoveAll(layout.SBOMDir)
	if err2 != nil {
		l.Error(err2.Error(), "SBOMDir", layout.SBOMDir)
	}
}

// GetVariableConfig returns the variable configuration for the packager.
func (p *Packager) GetVariableConfig() *variables.VariableConfig {
	return p.variableConfig
}

// connectToCluster attempts to connect to a cluster if a connection is not already established
func (p *Packager) connectToCluster(ctx context.Context) error {
	if p.isConnectedToCluster() {
		return nil
	}

	cluster, err := cluster.NewClusterWithWait(ctx)
	if err != nil {
		return err
	}
	p.cluster = cluster

	return p.attemptClusterChecks(ctx)
}

// isConnectedToCluster returns whether the current packager instance is connected to a cluster
func (p *Packager) isConnectedToCluster() bool {
	return p.cluster != nil
}

// attemptClusterChecks attempts to connect to the cluster and check for useful metadata and config mismatches.
// NOTE: attemptClusterChecks should only return an error if there is a problem significant enough to halt a deployment, otherwise it should return nil and print a warning message.
func (p *Packager) attemptClusterChecks(ctx context.Context) error {
	spinner := message.NewProgressSpinner("Gathering additional cluster information (if available)")
	defer spinner.Stop()

	// Check the clusters architecture matches the package spec
	if err := p.validatePackageArchitecture(ctx); err != nil {
		if errors.Is(err, lang.ErrUnableToCheckArch) {
			message.Warnf("Unable to validate package architecture: %s", err.Error())
		} else {
			return err
		}
	}

	// Check for any breaking changes between the initialized Zarf version and this CLI
	if existingInitPackage, _ := p.cluster.GetDeployedPackage(ctx, "init"); existingInitPackage != nil {
		// Use the build version instead of the metadata since this will support older Zarf versions
		err := deprecated.PrintBreakingChanges(os.Stderr, existingInitPackage.Data.Build.Version, config.CLIVersion)
		if err != nil {
			return err
		}
	}

	spinner.Success()

	return nil
}

// validatePackageArchitecture validates that the package architecture matches the target cluster architecture.
func (p *Packager) validatePackageArchitecture(ctx context.Context) error {
	// Ignore this check if we don't have a cluster connection, or the package contains no images
	if !p.isConnectedToCluster() || !p.cfg.Pkg.HasImages() {
		return nil
	}

	// Get node architectures
	nodeList, err := p.cluster.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return lang.ErrUnableToCheckArch
	}
	if len(nodeList.Items) == 0 {
		return lang.ErrUnableToCheckArch
	}
	archMap := map[string]bool{}
	for _, node := range nodeList.Items {
		archMap[node.Status.NodeInfo.Architecture] = true
	}
	architectures := []string{}
	for arch := range archMap {
		architectures = append(architectures, arch)
	}

	// Check if the package architecture and the cluster architecture are the same.
	if !slices.Contains(architectures, p.cfg.Pkg.Metadata.Architecture) {
		return fmt.Errorf(lang.CmdPackageDeployValidateArchitectureErr, p.cfg.Pkg.Metadata.Architecture, strings.Join(architectures, ", "))
	}

	return nil
}

// validateLastNonBreakingVersion validates the Zarf CLI version against a package's LastNonBreakingVersion.
func validateLastNonBreakingVersion(cliVersion, lastNonBreakingVersion string) ([]string, error) {
	if lastNonBreakingVersion == "" {
		return nil, nil
	}
	lastNonBreakingSemVer, err := semver.NewVersion(lastNonBreakingVersion)
	if err != nil {
		return nil, fmt.Errorf("unable to parse last non breaking version %s from Zarf package build data: %w", lastNonBreakingVersion, err)
	}
	cliSemVer, err := semver.NewVersion(cliVersion)
	if err != nil {
		return []string{fmt.Sprintf(lang.CmdPackageDeployInvalidCLIVersionWarn, cliVersion)}, nil
	}
	if cliSemVer.LessThan(lastNonBreakingSemVer) {
		warning := fmt.Sprintf(
			lang.CmdPackageDeployValidateLastNonBreakingVersionWarn,
			cliVersion,
			lastNonBreakingVersion,
			lastNonBreakingVersion,
		)
		return []string{warning}, nil
	}
	return nil, nil
}
