// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package common handles command configuration across all commands
package common

// SetBaseDirectory sets base directory on package config when given in args
func SetBaseDirectory(args []string) string {
	if len(args) > 0 {
		return args[0]
	} else {
		return "."
	}
}
