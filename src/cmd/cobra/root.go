// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cobra holds shell tab-completion helpers shared by Zarf's cobra
// commands, returning "value\tdescription" pairs for use in
// cobra.RegisterFlagCompletionFunc callbacks.
package cobra

import (
	"fmt"

	"github.com/zarf-dev/zarf/src/config"
)

// osArchAMD64Desc and osArchARM64Desc are the shell completion descriptions
// shown alongside each supported --architecture value.
const (
	osArchAMD64Desc = "the x86-64, 64-bit AMD, architecture"
	osArchARM64Desc = "the 64-bit ARM architecture"
)

// GetRootArchitectureCobraCompression returns the valid --architecture
// values as "value\tdescription" pairs, ready to return from a cobra
// RegisterFlagCompletionFunc callback for shell tab completion.
func GetRootArchitectureCobraCompression() []string {
	return []string{
		fmt.Sprintf("%s\t%s", string(config.OSArchAMD64), osArchAMD64Desc),
		fmt.Sprintf("%s\t%s", string(config.OSArchARM64), osArchARM64Desc),
	}
}
