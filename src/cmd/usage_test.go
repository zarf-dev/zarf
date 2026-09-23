// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
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

func TestPackageSigningFlagsAreGrouped(t *testing.T) {
	t.Parallel()

	signingFlags := []string{
		"signing-key", "signing-key-pass", "keyless", "identity-token", "fulcio-url",
		"fulcio-auth-flow", "oidc-issuer", "oidc-client-id", "rekor-url", "tlog-upload", "tsa-server-url",
	}
	commands := map[string]*cobra.Command{
		"component-publish": newComponentPublishCommand(newTestViper()),
		"component-sign":    newComponentSignCommand(newTestViper()),
		"create":            newPackageCreateCommand(newTestViper()),
		"sign":              newPackageSignCommand(newTestViper()),
	}
	for name, cmd := range commands {
		t.Run(name, func(t *testing.T) {
			for _, name := range signingFlags {
				flag := cmd.Flags().Lookup(name)
				require.NotNilf(t, flag, "%s must register %s", cmd.Name(), name)
				require.Equal(t, []string{signingFlagGroupTitle}, flag.Annotations[flagGroupAnnotation])
			}
		})
	}
}

func TestPackageSigningFlagsMutuallyExclusive(t *testing.T) {
	t.Parallel()

	for _, factory := range []func(*viper.Viper) *cobra.Command{
		newComponentPublishCommand,
		newComponentSignCommand,
		newPackageCreateCommand,
		newPackageSignCommand,
	} {
		cmd := factory(newTestViper())
		require.NoError(t, cmd.Flags().Set("keyless", "true"))
		require.NoError(t, cmd.Flags().Set("signing-key", "key"))
		require.Error(t, cmd.ValidateFlagGroups())
	}

	create := newPackageCreateCommand(newTestViper())
	require.NoError(t, create.Flags().Set("keyless", "true"))
	require.NoError(t, create.Flags().Set("key", "key"))
	require.Error(t, create.ValidateFlagGroups())
}

func TestPackageSigningUsageGroups(t *testing.T) {
	commands := map[string]*cobra.Command{
		"component-publish": newComponentPublishCommand(newTestViper()),
		"component-sign":    newComponentSignCommand(newTestViper()),
		"create":            newPackageCreateCommand(newTestViper()),
		"sign":              newPackageSignCommand(newTestViper()),
	}
	for name, cmd := range commands {
		t.Run(name, func(t *testing.T) {
			setupGroupedFlagUsage(cmd)
			usage := cmd.UsageString()
			signingGroupIndex := strings.Index(usage, signingFlagGroupTitle+":")
			require.NotEqual(t, -1, signingGroupIndex)
			require.Contains(t, usage[signingGroupIndex:], "--signing-key")
			require.Contains(t, usage[signingGroupIndex:], "--keyless")

			defaultFlags := usage[strings.Index(usage, "Flags:"):signingGroupIndex]
			require.NotContains(t, defaultFlags, "--signing-key")
			require.NotContains(t, defaultFlags, "--keyless")
		})
	}
}

func TestPackagePublishSigningFlagsAreDeprecated(t *testing.T) {
	t.Parallel()

	cmd := newPackagePublishCommand(newTestViper())
	for _, name := range []string{"signing-key", "signing-key-pass"} {
		flag := cmd.Flags().Lookup(name)
		require.NotNilf(t, flag, "publish must register %s", name)
		require.Contains(t, flag.Deprecated, "package sign")
	}
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
