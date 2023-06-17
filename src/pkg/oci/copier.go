// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package oci contains functions for interacting with Zarf packages stored in OCI registries.
package oci

import "github.com/defenseunicorns/zarf/src/pkg/message"

type Copier struct {
	src OrasRemote
	dst OrasRemote
}

func (c *Copier) CopyPackage() error {
	message.Infof("Copying from %s to %s", c.src.Reference, c.dst.Reference)
	return nil
}
