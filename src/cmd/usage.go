// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// defaultFlagsSection is Cobra's default rendering of a command's local flags. We
// match it verbatim in cmd.UsageTemplate()
const defaultFlagsSection = "Flags:\n{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}"

// groupedFlagsSection renders local flags in two parts: flags without a group
// annotation, followed by one titled section per flag group (see flagGroupAnnotation).
const groupedFlagsSection = "Flags:\n{{ungroupedFlagUsages .LocalFlags | trimTrailingWhitespaces}}{{groupedFlagSections .LocalFlags}}{{end}}"

// setupGroupedFlagUsage registers the template helpers and installs a usage template
// derived from Cobra's default by swapping in grouped-flag rendering. Cobra propagates
// the template to all descendant commands.
func setupGroupedFlagUsage(cmd *cobra.Command) {
	cobra.AddTemplateFunc("ungroupedFlagUsages", ungroupedFlagUsages)
	cobra.AddTemplateFunc("groupedFlagSections", groupedFlagSections)
	cmd.SetUsageTemplate(groupedFlagUsageTemplate(cmd.UsageTemplate()))
}

func groupedFlagUsageTemplate(base string) string {
	return strings.Replace(base, defaultFlagsSection, groupedFlagsSection, 1)
}

// ungroupedFlagUsages renders the usage block for every flag in fs that is not
// assigned to a group, preserving Cobra's default column alignment.
func ungroupedFlagUsages(fs *pflag.FlagSet) string {
	ungrouped := pflag.NewFlagSet("", pflag.ContinueOnError)
	fs.VisitAll(func(f *pflag.Flag) {
		if _, ok := f.Annotations[flagGroupAnnotation]; !ok {
			ungrouped.AddFlag(f)
		}
	})
	return ungrouped.FlagUsages()
}

// groupedFlagSections renders one "<title>:" usage block per flag group present in
// fs, ordered by title. It returns an empty string when no flags are grouped.
func groupedFlagSections(fs *pflag.FlagSet) string {
	var titles []string
	fs.VisitAll(func(f *pflag.Flag) {
		if vals := f.Annotations[flagGroupAnnotation]; len(vals) > 0 && !slices.Contains(titles, vals[0]) {
			titles = append(titles, vals[0])
		}
	})
	slices.Sort(titles)

	var b strings.Builder
	for _, title := range titles {
		group := pflag.NewFlagSet("", pflag.ContinueOnError)
		fs.VisitAll(func(f *pflag.Flag) {
			if vals := f.Annotations[flagGroupAnnotation]; len(vals) > 0 && vals[0] == title {
				group.AddFlag(f)
			}
		})
		usages := strings.TrimRight(group.FlagUsages(), "\n")
		if usages == "" {
			continue
		}
		fmt.Fprintf(&b, "\n\n%s:\n%s", title, usages)
	}
	return b.String()
}
