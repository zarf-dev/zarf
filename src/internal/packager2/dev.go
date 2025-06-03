// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"context"
	"errors"
	"runtime"
	"time"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/packager2/filters"
	"github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/internal/packager2/load"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/state"
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
	RegistryOverrides map[string]string
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
}

// DevDeploy creates + deploys a package in one shot
func DevDeploy(ctx context.Context, packagePath string, opts DevDeployOptions) (err error) {
	l := logger.From(ctx)
	start := time.Now()
	config.CommonOptions.Confirm = true

	pkg, err := load.PackageDefinition(ctx, packagePath, opts.Flavor, opts.CreateSetVariables)
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
			pkg.Components[idx].Repos = []string{}
		}
	}

	createOpt := layout.AssembleLayoutOptions{
		Flavor:            opts.Flavor,
		RegistryOverrides: opts.RegistryOverrides,
		SkipSBOM:          true,
		OCIConcurrency:    config.CommonOptions.OCIConcurrency,
	}

	pkgLayout, err := layout.AssemblePackage(ctx, pkg, packagePath, createOpt)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, pkgLayout.Cleanup())
	}()

	variableConfig, err := getPopulatedVariableConfig(ctx, pkgLayout.Pkg, opts.DeploySetVariables)
	if err != nil {
		return err
	}

	l.Info("starting package dev deploy", "name", pkgLayout.Pkg.Metadata.Name)

	var d deployer
	d.vc = variableConfig
	if !opts.AirgapMode {
		pkgLayout.Pkg.Metadata.YOLO = true
		// Set default builtin values so they exist in case any helm charts rely on them
		defaultState, err := state.Default()
		if err != nil {
			return err
		}
		if opts.RegistryURL != "" {
			defaultState.RegistryInfo.Address = opts.RegistryURL
		}
		d.s = defaultState
	} else {
		d.hpaModified = false
		// Reset registry HPA scale down whether an error occurs or not
		defer d.resetRegistryHPA(ctx)
	}

	// Get a list of all the components we are deploying and actually deploy them
	deployedComponents, err := d.deployComponents(ctx, pkgLayout, DeployOpts{
		SetVariables:          opts.DeploySetVariables,
		Timeout:               opts.Timeout,
		Retries:               opts.Retries,
		OCIConcurrency:        config.CommonOptions.OCIConcurrency,
		PlainHTTP:             config.CommonOptions.PlainHTTP,
		InsecureTLSSkipVerify: config.CommonOptions.InsecureSkipTLSVerify,
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
