// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"context"
	"github.com/zarf-dev/zarf/src/internal/packager2"
	"github.com/zarf-dev/zarf/src/pkg/packager/filters"
)

// PullOptions declares optional configuration for a Pull operation.
type PullOptions struct {
	// SHASum uniquely identifies a package based on its contents.
	SHASum string
	// SkipSignatureValidation flags whether Pull should skip validating the signature.
	SkipSignatureValidation bool
	// Architecture is the package architecture.
	Architecture string
	// Filters describes a Filter strategy to include or exclude certain components from the package.
	Filters filters.ComponentFilterStrategy
	// PublicKeyPath validates the create-time signage of a package.
	PublicKeyPath string
}

// Pull takes a source URL and destination directory and fetches the Zarf package from the given sources.
func Pull(ctx context.Context, source, destination string, opts PullOptions) error {
	return packager2.Pull(ctx, source, destination, packager2.PullOptions{
		SHASum:                  opts.SHASum,
		SkipSignatureValidation: opts.SkipSignatureValidation,
		Architecture:            opts.Architecture,
		Filters:                 opts.Filters,
		PublicKeyPath:           opts.PublicKeyPath,
	})
}
