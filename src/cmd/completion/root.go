// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package completion holds shell tab-completion helpers shared by Zarf's
// cobra commands, returning "value\tdescription" pairs for use in
// cobra.RegisterFlagCompletionFunc callbacks.
package completion

import (
	"fmt"

	"github.com/zarf-dev/zarf/src/config"
)

// Descriptions shown alongside each supported --architecture value.
const (
	osArchAMD64Desc = "the x86-64, 64-bit AMD, architecture"
	osArchARM64Desc = "the 64-bit ARM architecture"
	osArchRISCVDesc = "the 64-bit RISC-V architecture"
)

// Architectures returns the valid --architecture values. It must stay in step
// with image.ValidatePlatformArch, which rejects anything not listed here.
func Architectures() []string {
	return []string{
		fmt.Sprintf("%s\t%s", config.OSArchAMD64, osArchAMD64Desc),
		fmt.Sprintf("%s\t%s", config.OSArchARM64, osArchARM64Desc),
		fmt.Sprintf("%s\t%s", config.OSArchRISCV, osArchRISCVDesc),
	}
}
