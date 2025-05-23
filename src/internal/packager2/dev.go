// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"context"
	"time"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/packager2/layout"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/state"
)

type DevDeployOptions struct {
	Airgap             bool
	Flavor             string
	RegistryURL        string
	RegistryOverrides  map[string]string
	CreateSetVariables map[string]string
	DeploySetVariables map[string]string
	OptionalComponents string
	Architecture       string
	Timeout            time.Duration
	Retries            int
}

// DevDeploy creates + deploys a package in one shot
func DevDeploy(ctx context.Context, packagePath string, opts DevDeployOptions) error {

	l := logger.From(ctx)
	start := time.Now()
	config.CommonOptions.Confirm = true

	pkg, err := layout.LoadPackageDefinition(ctx, packagePath, opts.Flavor, opts.CreateSetVariables)
	if err != nil {
		return err
	}

	// If not building for airgap, strip out all images and repos
	if !opts.Airgap {
		for idx := range pkg.Components {
			pkg.Components[idx].Images = []string{}
			pkg.Components[idx].Repos = []string{}
		}
	}

	createOpt := layout.AssembleOptions{
		Flavor:            opts.Flavor,
		RegistryOverrides: opts.RegistryOverrides,
		SkipSBOM:          true,
		OCIConcurrency:    config.CommonOptions.OCIConcurrency,
	}

	pkgLayout, err := layout.AssemblePackage(ctx, pkg, packagePath, createOpt)
	if err != nil {
		return err
	}
	defer pkgLayout.Cleanup()

	variableConfig, err := getPopulatedVariableConfig(ctx, pkgLayout.Pkg, opts.DeploySetVariables)
	if err != nil {
		return err
	}

	l.Info("starting package dev deploy", "name", pkgLayout.Pkg.Metadata.Name)

	var d deployer
	d.vc = variableConfig
	if !opts.Airgap {
		pkgLayout.Pkg.Metadata.YOLO = true
		defaultState, err := state.Default()
		if err != nil {
			return err
		}
		// Set default builtin values so they exist in case any helm charts rely on them
		defaultState.RegistryInfo.Address = opts.RegistryURL
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
