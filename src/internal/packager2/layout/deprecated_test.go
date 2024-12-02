// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
)

func TestMigrateDeprecated(t *testing.T) {
	t.Parallel()

	pkg := v1alpha1.ZarfPackage{
		Components: []v1alpha1.ZarfComponent{
			{
				DeprecatedScripts: v1alpha1.DeprecatedZarfComponentScripts{
					Retry:   true,
					Prepare: []string{"p"},
					Before:  []string{"b"},
					After:   []string{"a"},
				},
				Actions: v1alpha1.ZarfComponentActions{
					OnCreate: v1alpha1.ZarfComponentActionSet{
						After: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "after",
							},
						},
						Before: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "before",
							},
						},
						OnSuccess: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "on-success",
							},
						},
						OnFailure: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "on-failure",
							},
						},
					},
					OnDeploy: v1alpha1.ZarfComponentActionSet{
						After: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "after",
							},
						},
						Before: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "before",
							},
						},
						OnSuccess: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "on-success",
							},
						},
						OnFailure: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "on-failure",
							},
						},
					},
					OnRemove: v1alpha1.ZarfComponentActionSet{
						After: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "after",
							},
						},
						Before: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "before",
							},
						},
						OnSuccess: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "on-success",
							},
						},
						OnFailure: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "on-failure",
							},
						},
					},
				},
			},
		},
	}
	migratedPkg, _ := migrateDeprecated(pkg)

	expectedPkg := v1alpha1.ZarfPackage{
		Build: v1alpha1.ZarfBuildData{
			LastNonBreakingVersion: LastNonBreakingVersion,
			Migrations: []string{
				ScriptsToActionsMigrated,
				PluralizeSetVariable,
			},
		},
		Components: []v1alpha1.ZarfComponent{
			{
				DeprecatedScripts: v1alpha1.DeprecatedZarfComponentScripts{
					Retry:   true,
					Prepare: []string{"p"},
					Before:  []string{"b"},
					After:   []string{"a"},
				},
				Actions: v1alpha1.ZarfComponentActions{
					OnCreate: v1alpha1.ZarfComponentActionSet{
						Defaults: v1alpha1.ZarfComponentActionDefaults{
							Mute:       true,
							MaxRetries: math.MaxInt,
						},
						Before: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "before",
								SetVariables: []v1alpha1.Variable{
									{
										Name: "before",
									},
								},
							},
							{
								Cmd: "p",
							},
						},
						After: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "after",
								SetVariables: []v1alpha1.Variable{
									{
										Name: "after",
									},
								},
							},
						},
						OnSuccess: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "on-success",
								SetVariables: []v1alpha1.Variable{
									{
										Name: "on-success",
									},
								},
							},
						},
						OnFailure: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "on-failure",
								SetVariables: []v1alpha1.Variable{
									{
										Name: "on-failure",
									},
								},
							},
						},
					},
					OnDeploy: v1alpha1.ZarfComponentActionSet{
						Defaults: v1alpha1.ZarfComponentActionDefaults{
							Mute:       true,
							MaxRetries: math.MaxInt,
						},
						Before: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "before",
								SetVariables: []v1alpha1.Variable{
									{
										Name: "before",
									},
								},
							},
							{
								Cmd: "b",
							},
						},
						After: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "after",
								SetVariables: []v1alpha1.Variable{
									{
										Name: "after",
									},
								},
							},
							{
								Cmd: "a",
							},
						},
						OnSuccess: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "on-success",
								SetVariables: []v1alpha1.Variable{
									{
										Name: "on-success",
									},
								},
							},
						},
						OnFailure: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "on-failure",
								SetVariables: []v1alpha1.Variable{
									{
										Name: "on-failure",
									},
								},
							},
						},
					},
					OnRemove: v1alpha1.ZarfComponentActionSet{
						Before: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "before",
								SetVariables: []v1alpha1.Variable{
									{
										Name: "before",
									},
								},
							},
						},
						After: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "after",
								SetVariables: []v1alpha1.Variable{
									{
										Name: "after",
									},
								},
							},
						},
						OnSuccess: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "on-success",
								SetVariables: []v1alpha1.Variable{
									{
										Name: "on-success",
									},
								},
							},
						},
						OnFailure: []v1alpha1.ZarfComponentAction{
							{
								DeprecatedSetVariable: "on-failure",
								SetVariables: []v1alpha1.Variable{
									{
										Name: "on-failure",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	require.Equal(t, expectedPkg, migratedPkg)
}
