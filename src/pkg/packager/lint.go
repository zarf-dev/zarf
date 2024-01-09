// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package packager contains functions for interacting with, managing and deploying Zarf packages.
package packager

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/pkg/packager/lint"
)

// Lint pulls resources from a package (images, git repositories, etc) and pushes them to remotes in the air gap without deploying them
func (p *Packager) Lint() (err error) {

	if err := p.setVariableMapInConfig(); err != nil {
		return fmt.Errorf("unable to set the active variables: %w", err)
	}

	lint.Validate(p.cfg)

	return nil
}
