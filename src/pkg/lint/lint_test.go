// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package lint contains functions for verifying zarf yaml files are valid
package lint

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/rules"
	"github.com/zarf-dev/zarf/src/types"
)

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

		createOpts := types.ZarfCreateOptions{Flavor: "", BaseDir: "."}
		_, err := lintComponents(context.Background(), zarfPackage, createOpts)
		require.Error(t, err)
	})
}
func TestFillComponentTemplate(t *testing.T) {
	createOpts := types.ZarfCreateOptions{
		SetVariables: map[string]string{
			"KEY1": "value1",
			"KEY2": "value2",
		},
	}

	component := v1alpha1.ZarfComponent{
		Images: []string{
			fmt.Sprintf("%s%s###", v1alpha1.ZarfPackageTemplatePrefix, "KEY1"),
			fmt.Sprintf("%s%s###", v1alpha1.ZarfPackageVariablePrefix, "KEY2"),
			fmt.Sprintf("%s%s###", v1alpha1.ZarfPackageTemplatePrefix, "KEY3"),
		},
	}

	findings, err := fillComponentTemplate(&component, createOpts)
	require.NoError(t, err)
	expectedFindings := []rules.PackageFinding{
		{
			Severity:    rules.SevWarn,
			Description: "There are templates that are not set and won't be evaluated during lint",
		},
		{
			Severity:    rules.SevWarn,
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
