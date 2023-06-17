// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

import "errors"

var (
	ErrBundlerNilConfig             = errors.New("bundler.New() called with nil config")
	ErrBundlerUnableToCreateTempDir = "bundler unable to create temp directory: %w"
	ErrBundlerNewOrDie              = "bundler unable to setup, bad config: %w"
	ErrBundlerFS                    = "error in BundlerFS operation: %w"
)
