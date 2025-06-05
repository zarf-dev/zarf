// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"context"

	"github.com/zarf-dev/zarf/src/internal/packager2"
)

// PullOptions declares optional configuration for a Pull operation.
type PullOptions struct {
	// SHASum uniquely identifies a package based on its contents.
	SHASum string
	// SkipSignatureValidation flags whether Pull should skip validating the signature.
	SkipSignatureValidation bool
	// Architecture is the package architecture.
	Architecture string
	// PublicKeyPath validates the create-time signage of a package.
	PublicKeyPath string
	// OCIConcurrency is the number of layers pulled in parallel
	OCIConcurrency int
	packager2.RemoteOptions
}

// Pull takes a source URL and destination directory and fetches the Zarf package from the given sources.
func Pull(ctx context.Context, source, destination string, opts PullOptions) (string, error) {
	return packager2.Pull(ctx, source, destination, packager2.PullOptions{
		SHASum:                  opts.SHASum,
		SkipSignatureValidation: opts.SkipSignatureValidation,
		Architecture:            opts.Architecture,
		PublicKeyPath:           opts.PublicKeyPath,
		OCIConcurrency:          opts.OCIConcurrency,
		RemoteOptions:           opts.RemoteOptions,
	})
}
