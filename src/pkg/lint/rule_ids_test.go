// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package lint

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRuleIDs(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		id   string
		want string
	}{
		{RuleRepositoryUnpinned, "repository-unpinned"},
		{RuleImageReferenceInvalid, "image-reference-invalid"},
		{RuleImageUnpinned, "image-unpinned"},
		{RuleImageRegistryMissing, "image-registry-missing"},
		{RuleFileChecksumMissing, "file-checksum-missing"},
		{RuleImageArchiveDomain, "image-archive-domain"},
		{RuleValuesSchemaMissing, "values-schema-missing"},
		{RuleValuesUnused, "values-unused"},
		{RuleTemplateSyntax, "template-syntax"},
		{RuleTemplateInputMissing, "template-input-missing"},
		{RuleTemplateRender, "template-render"},
		{RuleValuesMapping, "values-mapping"},
		{RuleChartValuesSchema, "chart-values-schema"},
		{RuleChartSchemaSkipped, "chart-schema-skipped"},
	} {
		require.Equal(t, tc.want, tc.id)
		require.NoError(t, ValidateSkipRules([]string{tc.id}))
	}
}

func TestValidateSkipRules(t *testing.T) {
	t.Parallel()
	require.NoError(t, ValidateSkipRules(nil))
	require.NoError(t, ValidateSkipRules([]string{RuleImageUnpinned, RuleImageUnpinned}))
	for _, id := range []string{"unknown", "Image-unpinned", "image-*"} {
		require.ErrorContains(t, ValidateSkipRules([]string{RuleImageUnpinned, id}), "unknown lint rule")
	}
}
