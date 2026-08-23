// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"strings"

	"github.com/spf13/cobra"
)

// ReplaceCommandName recursively replaces all references of one string with another in the Example string
// code credit, deckhouse/deckhouse-cli
// https://github.com/deckhouse/deckhouse-cli/blob/7e0c1e743b16c82134a062985dde161178bd45f6/cmd/commands/utils.go#L25
func ReplaceCommandName(from, to string, c *cobra.Command) *cobra.Command {
	c.Example = strings.ReplaceAll(c.Example, from, to)
	for _, sub := range c.Commands() {
		ReplaceCommandName(from, to, sub)
	}
	return c
}
