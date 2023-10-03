// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/pkg/message"
)

// Pull pulls a Zarf package and saves it as a compressed tarball.
func (p *Packager) Pull() (err error) {
	if p.cfg.PkgOpts.OptionalComponents != "" {
		return fmt.Errorf("pull does not support optional components")
	}

	tb, err := p.source.Collect(p.cfg.PullOpts.OutputDirectory)
	if err != nil {
		return err
	}

	message.Infof("Pulled %q into %q", p.cfg.PkgOpts.PackageSource, tb)

	return nil
}
