// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package oci

import "path/filepath"

var (
	// ZarfPackageIndexPath is the path to the index.json file in the OCI package.
	ZarfPackageIndexPath = filepath.Join("images", "index.json")
	// ZarfPackageLayoutPath is the path to the oci-layout file in the OCI package.
	ZarfPackageLayoutPath = filepath.Join("images", "oci-layout")
	// ZarfPackageImagesBlobsDir is the path to the directory containing the image blobs in the OCI package.
	ZarfPackageImagesBlobsDir = filepath.Join("images", "blobs", "sha256")
)
