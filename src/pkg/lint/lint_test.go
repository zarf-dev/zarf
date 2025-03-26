// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/test/testutil"
)

func TestLintError(t *testing.T) {
	t.Parallel()

	lintErr := &LintError{
		Findings: []PackageFinding{
			{
				Severity: SevWarn,
			},
		},
	}
	require.Equal(t, "linting error found 1 instance(s)", lintErr.Error())
	require.True(t, lintErr.OnlyWarnings())

	lintErr = &LintError{
		Findings: []PackageFinding{
			{
				Severity: SevWarn,
			},
			{
				Severity: SevErr,
			},
		},
	}
	require.Equal(t, "linting error found 2 instance(s)", lintErr.Error())
	require.False(t, lintErr.OnlyWarnings())
}

func TestLintComponents(t *testing.T) {
	t.Run("Test composable components with bad path", func(t *testing.T) {
		t.Parallel()
		zarfPackage := v1alpha1.ZarfPackage{
			Components: []v1alpha1.ZarfComponent{
				{
					Import: v1alpha1.ZarfComponentImport{Path: "bad-path"},
				},
			},
			Metadata: v1alpha1.ZarfMetadata{Name: "test-zarf-package"},
		}

		_, err := lintComponents(context.Background(), zarfPackage, "", nil)
		require.Error(t, err)
	})
}
func TestFillObjTemplate(t *testing.T) {
	SetVariables := map[string]string{
		"KEY1": "value1",
		"KEY2": "value2",
	}

	component := v1alpha1.ZarfComponent{
		Images: []string{
			fmt.Sprintf("%s%s###", v1alpha1.ZarfPackageTemplatePrefix, "KEY1"),
			fmt.Sprintf("%s%s###", v1alpha1.ZarfPackageVariablePrefix, "KEY2"),
			fmt.Sprintf("%s%s###", v1alpha1.ZarfPackageTemplatePrefix, "KEY3"),
		},
	}

	findings, err := templateZarfObj(&component, SetVariables)
	require.NoError(t, err)
	expectedFindings := []PackageFinding{
		{
			Severity:    SevWarn,
			Description: "package template KEY3 is not set and won't be evaluated during lint",
		},
		{
			Severity:    SevWarn,
			Description: fmt.Sprintf(lang.PkgValidateTemplateDeprecation, "KEY2", "KEY2", "KEY2"),
		},
	}
	expectedComponent := v1alpha1.ZarfComponent{
		Images: []string{
			"value1",
			"value2",
			fmt.Sprintf("%s%s###", v1alpha1.ZarfPackageTemplatePrefix, "KEY3"),
		},
	}
	require.ElementsMatch(t, expectedFindings, findings)
	require.Equal(t, expectedComponent, component)
}

func TestLintPackageWithImports(t *testing.T) {
	ZarfSchema = testutil.LoadSchema(t, "../../../zarf.schema.json")
	setVariables := map[string]string{
		"BUSYBOX_IMAGE": "latest",
	}
	ctx := context.Background()
	findings := []PackageFinding{
		// unset exists in both the root and imported package
		{
			YqPath:              "",
			Description:         "package template UNSET is not set and won't be evaluated during lint",
			Item:                "",
			PackageNameOverride: "linted-import",
			PackagePathOverride: "linted-import",
			Severity:            SevWarn,
		},
		{
			YqPath:              "",
			Description:         "package template UNSET is not set and won't be evaluated during lint",
			Item:                "",
			PackageNameOverride: "lint",
			PackagePathOverride: ".",
			Severity:            SevWarn,
		},
		// Test imported skeleton package lints properly
		{
			YqPath:              ".components.[0].images.[0]",
			Description:         "Image not pinned with digest",
			Item:                "ghcr.io/zarf-dev/doom-game:0.0.1",
			PackageNameOverride: "dos-games",
			PackagePathOverride: "oci://ghcr.io/zarf-dev/packages/dos-games:1.2.0",
			Severity:            SevWarn,
		},
		// Test local import lints properly
		{
			YqPath:              ".components.[1].images.[0]",
			Description:         "Image not pinned with digest",
			Item:                "busybox:latest",
			PackageNameOverride: "linted-import",
			PackagePathOverride: "linted-import",
			Severity:            SevWarn,
		},
		// Test flavors
		{
			YqPath:              ".components.[4].images.[0]",
			Description:         "Image not pinned with digest",
			Item:                "image-in-good-flavor-component:unpinned",
			PackageNameOverride: "lint",
			PackagePathOverride: ".",
			Severity:            SevWarn,
		},
	}
	cwd, err := os.Getwd()
	require.NoError(t, err)
	// TODO @austinabro321: remove this and parallelize the test once changing the working directory is no longer required
	defer func() {
		require.NoError(t, os.Chdir(cwd))
	}()
	err = Validate(ctx, "testdata/lint-with-imports", "good-flavor", setVariables)
	var lintErr *LintError
	require.ErrorAs(t, err, &lintErr)
	require.ElementsMatch(t, findings, lintErr.Findings)
}
