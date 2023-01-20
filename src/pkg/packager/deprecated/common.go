// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package deprecated handles package deprecations and migrations
package deprecated

import (
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
)

// MigrateComponent runs all migrations on a component
func MigrateComponent(c types.ZarfComponent) types.ZarfComponent {
	message.Warnf("@JPERRY migrating the component: %s", c.Name)
	// Migrate scripts to actions
	c = migrateScriptsToActions(c)
	// Future migrations here
	return c
}
