// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/images"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/packager/assemble"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/value"
	"github.com/zarf-dev/zarf/src/types"
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
	// Values are values passed in at deploy time.
	Values value.Values
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
	// SkipArchitectureCheck allows application images to deploy to nodes with a different architecture.
	// The operator must configure emulation on the target nodes. Only applies in airgap mode.
	SkipArchitectureCheck bool
	// TakeOwnership adopts any pre-existing K8s resources into the Helm charts managed by Zarf
	TakeOwnership bool
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

	loadOpts := load.PackageOptions{
		DefinitionOptions: load.DefinitionOptions{
			Flavor:           opts.Flavor,
			SetVariables:     opts.CreateSetVariables,
			CachePath:        opts.CachePath,
			IsInteractive:    false,
			SkipVersionCheck: opts.SkipVersionCheck,
			RemoteOptions:    opts.RemoteOptions,
		},
	}
	loaded, err := load.Package(ctx, packagePath, loadOpts)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, loaded.Close())
	}()
	if opts.SkipArchitectureCheck && loaded.Definition.IsInitConfig() {
		return fmt.Errorf("skipping architecture checks is not supported for init packages")
	}

	// If not building for airgap, strip out all images and repos.
	if !opts.AirgapMode {
		loaded.Definition.RemoveImages()
		loaded.Definition.RemoveRepositories()
	}

	createOpts := assemble.AssembleOptions{
		Flavor:            opts.Flavor,
		RegistryOverrides: opts.RegistryOverrides,
		SkipSBOM:          true,
		OCIConcurrency:    opts.OCIConcurrency,
		CachePath:         opts.CachePath,
	}
	pkgLayout, err := assemble.AssemblePackage(ctx, loaded, createOpts)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, pkgLayout.Cleanup())
	}()
	l.Info("starting package dev deploy", "name", pkgLayout.Definition().Metadata.Name)

	result, err := Deploy(ctx, pkgLayout, DeployOptions{
		DeployConfig: DeployConfig{
			SetVariables:       opts.DeploySetVariables,
			Values:             opts.Values,
			OptionalComponents: opts.OptionalComponents,
			Connected:          !opts.AirgapMode,
		},
		Source:                packagePath,
		RegistryInfo:          state.RegistryInfo{Address: opts.RegistryURL},
		Timeout:               opts.Timeout,
		Retries:               opts.Retries,
		OCIConcurrency:        opts.OCIConcurrency,
		RemoteOptions:         opts.RemoteOptions,
		TakeOwnership:         opts.TakeOwnership,
		SkipArchitectureCheck: opts.SkipArchitectureCheck,
		SkipVersionCheck:      opts.SkipVersionCheck,
	})
	if err != nil {
		return err
	}

	if len(result.DeployedComponents) == 0 {
		l.Warn("No components were selected for deployment. Inspect the package to view the available components and select components interactively or by name with \"--components\"")
	}

	// Notify all the things about the successful deployment.
	l.Debug("dev deployment complete", "package", pkgLayout.Definition().Metadata.Name, "duration", time.Since(start))
	return nil
}
