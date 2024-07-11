// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package deprecated handles package deprecations and migrations
package deprecated

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/pkg/variables"
	"github.com/defenseunicorns/zarf/src/types"
)

func migrateSetVariableToSetVariables(c types.ZarfComponent) (types.ZarfComponent, string) {
	hasSetVariable := false
	for i := range c.Actions {
		if c.Actions[i].DeprecatedSetVariable != "" && len(c.Actions[i].SetVariables) < 1 {
			hasSetVariable = true
			c.Actions[i].SetVariables = []variables.Variable{
				{
					Name:      c.Actions[i].DeprecatedSetVariable,
					Sensitive: false,
				},
			}
		}
	}
	if hasSetVariable {
		return c, fmt.Sprintf("Component %q is using setVariable in actions which will be removed in Zarf v1.0.0. Please migrate to the list form of setVariables.", c.Name)
	}
	return c, ""
}

func clearSetVariables(c types.ZarfComponent) types.ZarfComponent {
	for i := range c.Actions {
		c.Actions[i].DeprecatedSetVariable = ""
	}
	return c
}
