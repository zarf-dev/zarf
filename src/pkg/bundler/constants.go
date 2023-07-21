// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

const (
	ZarfBundleYAML          = "zarf-bundle.yaml"
	ZarfBundleYAMLSignature = "zarf-bundle.yaml.sig"
	ZarfBundlePrefix        = "zarf-bundle-"
)

var (
	// BundleAlwaysPull is a list of paths that will always be pulled from the remote repository.
	BundleAlwaysPull = []string{ZarfBundleYAML, ZarfBundleYAMLSignature}
)
