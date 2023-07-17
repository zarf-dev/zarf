// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package bundler contains functions for interacting with, managing and deploying Zarf bundles.
package bundler

// Deploy is different based on source
// depending on if source is a tarball or a OCI ref
// : if tarball
// : : create a new tarballProcessor from the tarball
// : : untar it into temp, but only the first package, making it look like packager's temp dir
// : : use b.p.Deploy() to deploy it
// : : variable scopes?
// : if OCI ref
// : : create a new OCIRemote from the OCI ref
// : : pull the package's layers into temp, making it look like packager's temp dir
// : : use b.p.Deploy() to deploy it
func (b *Bundler) Deploy() error {
	return nil
}
