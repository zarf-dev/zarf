// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

// TestUsageTemplateFlagsSectionMatchesCobra guards the assumption behind
// groupedFlagUsageTemplate: that Cobra's default usage template still contains the
// exact flags fragment we replace. If Cobra changes it, this fails so we re-sync
// defaultFlagsSection rather than silently losing flag grouping.
func TestUsageTemplateFlagsSectionMatchesCobra(t *testing.T) {
	t.Parallel()

	// Make sure Cobras default usage template hasn't changed
	cobraDefault := (&cobra.Command{}).UsageTemplate()
	require.Contains(t, cobraDefault, defaultFlagsSection)

	got := groupedFlagUsageTemplate(cobraDefault)
	require.Contains(t, got, groupedFlagsSection)
	require.NotContains(t, got, defaultFlagsSection)
}

func TestVerifyFlagsAreGrouped(t *testing.T) {
	t.Parallel()

	v := newTestViper()
	var f packageVerifyFlags
	fs := newVerifyFlagSet(v, &f)

	fs.VisitAll(func(flag *pflag.Flag) {
		require.Equal(t, []string{verifyFlagGroupTitle}, flag.Annotations[flagGroupAnnotation],
			"flag %q should belong to the verification group", flag.Name)
	})
}

func TestGroupedFlagUsageRendering(t *testing.T) {
	// Not parallel: setupGroupedFlagUsage registers template helpers via the global
	// cobra.AddTemplateFunc, which is not safe to race with other tests.

	v := newTestViper()
	var f packageVerifyFlags
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool("confirm", false, "an ungrouped flag")
	cmd.Flags().AddFlagSet(newVerifyFlagSet(v, &f))

	setupGroupedFlagUsage(cmd)
	usage := cmd.UsageString()

	flagsIdx := strings.Index(usage, "Flags:")
	groupIdx := strings.Index(usage, verifyFlagGroupTitle+":")
	require.NotEqual(t, -1, flagsIdx, "usage should have a Flags section")
	require.Less(t, flagsIdx, groupIdx, "the grouped section should render after the ungrouped Flags block")

	// Ungrouped flags render under the default "Flags:" heading; grouped flags only
	// under their own titled section.
	ungrouped := usage[flagsIdx:groupIdx]
	require.Contains(t, ungrouped, "--confirm")
	require.NotContains(t, ungrouped, "--certificate-identity")

	grouped := usage[groupIdx:]
	require.Contains(t, grouped, "--certificate-identity")
	require.NotContains(t, grouped, "--confirm")
}

func TestGroupedFlagSectionsEmptyWithoutGroups(t *testing.T) {
	t.Parallel()

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.Bool("confirm", false, "an ungrouped flag")

	require.Empty(t, groupedFlagSections(fs))
	require.Contains(t, ungroupedFlagUsages(fs), "--confirm")
}
