// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package types contains all the types used by Zarf.
package types

import (
	"context"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

// PackagerConfig is the main struct that the packager uses to hold high-level options.
type PackagerConfig struct {
	// Context provides deadlines, cancellations, and values throughout the API.
	// NOTE(mkcp): Storing ctx on structs is not recommended, but this is intended as a temporary workaround.
	Context context.Context

	// CreateOpts tracks the user-defined options used to create the package
	CreateOpts ZarfCreateOptions

	// PkgOpts tracks user-defined options
	PkgOpts ZarfPackageOptions

	// DeployOpts tracks user-defined values for the active deployment
	DeployOpts ZarfDeployOptions

	// MirrorOpts tracks user-defined values for the active mirror
	MirrorOpts ZarfMirrorOptions

	// InitOpts tracks user-defined values for the active Zarf initialization.
	InitOpts ZarfInitOptions

	// InspectOpts tracks user-defined options used to inspect the package
	InspectOpts ZarfInspectOptions

	// PublishOpts tracks user-defined options used to publish the package
	PublishOpts ZarfPublishOptions

	// PullOpts tracks user-defined options used to pull packages
	PullOpts ZarfPullOptions

	// FindImagesOpts tracks user-defined options used to find images
	FindImagesOpts ZarfFindImagesOptions

	// GenerateOpts tracks user-defined values for package generation.
	GenerateOpts ZarfGenerateOptions

	// The package data
	Pkg v1alpha1.ZarfPackage
}
