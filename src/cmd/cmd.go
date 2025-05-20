// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

// setBaseDirectory sets the base directory. This is a directory with a zarf.yaml.
func setBaseDirectory(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return "."
}
