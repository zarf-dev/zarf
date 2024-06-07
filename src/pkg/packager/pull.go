// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"context"
	"fmt"
)

// Pull pulls a Zarf package and saves it as a compressed tarball.
func (p *Packager) Pull(ctx context.Context) (err error) {
	if p.cfg.PkgOpts.OptionalComponents != "" {
		return fmt.Errorf("pull does not support optional components")
	}

	_, err = p.source.Collect(ctx, p.cfg.PullOpts.OutputDirectory)
	if err != nil {
		return err
	}

	return nil
}
