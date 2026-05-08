// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"time"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	"github.com/zarf-dev/zarf/src/pkg/images"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/types"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
)

// DevDeployOptions are the optionalParameters to DevDeploy
type DevDeployOptions struct {
	// When true packs images and repos into the package and uses the cluster Zarf state
	// When false deploys package without repos or images and uses the default Zarf state
	AirgapMode bool
	// Flavor causes the package to only include components with a matching `.components[x].only.flavor` or no flavor `.components[x].only.flavor` specified
	Flavor string
	// RegistryURL allows for an override to the Zarf state registry URL when not in airgap mode. Important for setting the ###ZARF_REGISTRY### template
	RegistryURL string
	// RegistryOverrides overrides the basepath of an OCI image with a path to a different registry during package assembly
	RegistryOverrides []images.RegistryOverride
	// CreateSetVariables are for package templates
	CreateSetVariables map[string]string
	// DeploySetVariables are for package variables
	DeploySetVariables map[string]string
	// OptionalComponents to be deployed
	OptionalComponents string
	// Timeout for Helm operations
	Timeout time.Duration
	// Retries to preform for operations like git and image pushes
	Retries int
	// These fields are only used if in airgap mode as they are relevant to requests from the git-server / registry
	OCIConcurrency int
	CachePath      string
	// SkipVersionCheck skips version requirement validation
	SkipVersionCheck bool
	types.RemoteOptions
}

// DevDeploy creates + deploys a package in one shot
func DevDeploy(ctx context.Context, packagePath string, opts DevDeployOptions) (err error) {
	l := logger.From(ctx)
	start := time.Now()

	if opts.Retries == 0 {
		opts.Retries = config.ZarfDefaultRetries
	}
	if opts.Timeout == 0 {
		opts.Timeout = config.ZarfDefaultTimeout
	}

	opts.CachePath, err = utils.ResolveCachePath(opts.CachePath)
	if err != nil {
		return err
	}

	loadOpts := load.DefinitionOptions{
		Flavor:           opts.Flavor,
		SetVariables:     opts.CreateSetVariables,
		CachePath:        opts.CachePath,
		IsInteractive:    false,
		SkipVersionCheck: opts.SkipVersionCheck,
		RemoteOptions:    opts.RemoteOptions,
	}
	pkg, err := load.PackageDefinition(ctx, packagePath, loadOpts)
	if err != nil {
		return err
	}

	filter := filters.Combine(
		filters.ByLocalOS(runtime.GOOS),
		filters.ForDeploy(opts.OptionalComponents, false),
	)
	pkg.Components, err = filter.Apply(pkg)
	if err != nil {
		return err
	}

	// If not building for airgap, strip out all images and repos
	if !opts.AirgapMode {
		for idx := range pkg.Components {
			pkg.Components[idx].Images = []string{}
			pkg.Components[idx].ImageArchives = []v1alpha1.ImageArchive{}
			pkg.Components[idx].Repos = []string{}
		}
	}

	createOpts := layout.AssembleOptions{
		Flavor:            opts.Flavor,
		RegistryOverrides: opts.RegistryOverrides,
		SkipSBOM:          true,
		OCIConcurrency:    opts.OCIConcurrency,
		CachePath:         opts.CachePath,
	}
	pkgLayout, err := layout.AssemblePackage(ctx, pkg, packagePath, createOpts)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, pkgLayout.Cleanup())
	}()

	variableConfig, err := getPopulatedVariableConfig(ctx, pkgLayout.Pkg, opts.DeploySetVariables, false)
	if err != nil {
		return err
	}

	l.Info("starting package dev deploy", "name", pkgLayout.Pkg.Metadata.Name)

	var d deployer
	d.vc = variableConfig
	if !opts.AirgapMode {
		// Set default builtin values so they exist in case any helm charts rely on them
		d.s, err = state.Default()
		if err != nil {
			return err
		}

		requiresCluster := slices.ContainsFunc(pkgLayout.Pkg.Components, func(c v1alpha1.ZarfComponent) bool {
			return c.RequiresCluster()
		})
		if requiresCluster {
			d.c, err = cluster.NewWithWait(ctx)
			if err != nil {
				return err
			}
			clusterState, err := d.c.LoadState(ctx)
			if err != nil && !kerrors.IsNotFound(err) {
				return err
			}
			if clusterState != nil {
				d.s = clusterState
			}
		}

		if opts.RegistryURL != "" {
			d.s.RegistryInfo.Address = opts.RegistryURL
		}
	}

	// Get a list of all the components we are deploying and actually deploy them
	deployedComponents, err := d.deployComponents(ctx, pkgLayout, DeployOptions{
		SetVariables:   opts.DeploySetVariables,
		Timeout:        opts.Timeout,
		Retries:        opts.Retries,
		Connected:      !opts.AirgapMode,
		OCIConcurrency: opts.OCIConcurrency,
		RemoteOptions:  opts.RemoteOptions,
	})
	if err != nil {
		return err
	}

	if len(deployedComponents) == 0 {
		l.Warn("No components were selected for deployment.  Inspect the package to view the available components and select components interactively or by name with \"--components\"")
	}

	// Notify all the things about the successful deployment
	l.Debug("dev deployment complete", "package", pkgLayout.Pkg.Metadata.Name, "duration", time.Since(start))

	return nil
}
