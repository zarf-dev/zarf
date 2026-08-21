// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package execution

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/packager/actions"
)

func TestComponents_AlphaActions(t *testing.T) {
	template := true
	timeout := 7
	retries := 2
	definition := api.NewPackageDefinitionFromV1alpha1(v1alpha1.ZarfPackage{
		Components: []v1alpha1.ZarfComponent{{
			Name: "alpha",
			Actions: v1alpha1.ZarfComponentActions{OnDeploy: v1alpha1.ZarfComponentActionSet{
				Defaults: v1alpha1.ZarfComponentActionDefaults{Mute: true, MaxTotalSeconds: 5, MaxRetries: 1, Shell: v1alpha1.Shell{Linux: "bash"}},
				Before:   []v1alpha1.ZarfComponentAction{{Cmd: "before", Template: &template, MaxTotalSeconds: &timeout, MaxRetries: &retries, SetValues: []v1alpha1.SetValue{{Key: ".output", Type: v1alpha1.SetValueJSON}}, SetVariables: []v1alpha1.Variable{{Name: "OUTPUT", Pattern: "^[a-z]+$"}}}},
				After:    []v1alpha1.ZarfComponentAction{{Cmd: "after"}},
			}},
		}},
	})

	component := Components(definition)[0]
	before := component.Actions.OnDeploy.Before
	require.Equal(t, actions.Config{Silent: true, Timeout: 5 * time.Second, Retries: 1, Shell: actions.Shell{Linux: "bash"}}, before.Defaults)
	require.Len(t, before.Actions, 1)
	require.True(t, before.Actions[0].ShouldTemplate)
	require.Equal(t, 7*time.Second, *before.Actions[0].Timeout)
	require.Equal(t, 2, *before.Actions[0].Retries)
	require.Equal(t, actions.ValueOutputJSON, before.Actions[0].SetValues[0].Type)
	require.Equal(t, "OUTPUT", before.Actions[0].SetVariables[0].Name)
	require.Equal(t, "after", component.Actions.OnDeploy.After.Actions[0].Cmd)
}

func TestComponents_BetaActions(t *testing.T) {
	template := true
	timeout := int32(7)
	retries := int32(2)
	definition := api.NewPackageDefinitionFromV1beta1(v1beta1.Package{
		Components: []v1beta1.Component{{
			Name: "beta",
			ComponentSpec: v1beta1.ComponentSpec{
				Actions: v1beta1.ComponentActions{OnDeploy: v1beta1.ComponentActionSet{
					Before:    []v1beta1.ComponentAction{{Cmd: "before", EnableTemplating: template, MaxTotalSeconds: &timeout, Retries: &retries, SetValues: []v1beta1.SetValue{{Key: ".output", Type: v1beta1.SetValueYAML}}}},
					OnSuccess: []v1beta1.ComponentAction{{Cmd: "success"}},
				}},
			},
		}},
	})

	component := Components(definition)[0]
	before := component.Actions.OnDeploy.Before
	require.Equal(t, actions.Config{}, before.Defaults, "nil beta defaults must be safe")
	require.True(t, before.Actions[0].ShouldTemplate)
	require.Equal(t, 7*time.Second, *before.Actions[0].Timeout)
	require.Equal(t, 2, *before.Actions[0].Retries)
	require.Equal(t, actions.ValueOutputYAML, before.Actions[0].SetValues[0].Type)
	require.Empty(t, component.Actions.OnDeploy.After.Actions, "beta has no after phase")
	require.Equal(t, "success", component.Actions.OnDeploy.OnSuccess.Actions[0].Cmd, "success actions must not be duplicated or reordered")
}

func TestComponents_WaitDefaultsRemainVersionSpecific(t *testing.T) {
	alpha := api.NewPackageDefinitionFromV1alpha1(v1alpha1.ZarfPackage{Components: []v1alpha1.ZarfComponent{{Actions: v1alpha1.ZarfComponentActions{OnDeploy: v1alpha1.ZarfComponentActionSet{Before: []v1alpha1.ZarfComponentAction{{Wait: &v1alpha1.ZarfComponentActionWait{Cluster: &v1alpha1.ZarfComponentActionWaitCluster{Kind: "pod"}}}}}}}}})
	beta := api.NewPackageDefinitionFromV1beta1(v1beta1.Package{Components: []v1beta1.Component{{ComponentSpec: v1beta1.ComponentSpec{Actions: v1beta1.ComponentActions{OnDeploy: v1beta1.ComponentActionSet{Before: []v1beta1.ComponentAction{{Wait: &v1beta1.ComponentActionWait{Cluster: &v1beta1.ComponentActionWaitCluster{Kind: "pod"}}}}}}}}}})

	alphaWait := Components(alpha)[0].Actions.OnDeploy.Before.Actions[0].Wait.Cluster
	betaWait := Components(beta)[0].Actions.OnDeploy.Before.Actions[0].Wait.Cluster
	require.Equal(t, actions.DefaultConditionExists, alphaWait.DefaultCondition)
	require.Empty(t, alphaWait.Condition)
	require.Equal(t, actions.DefaultConditionReady, betaWait.DefaultCondition)
	require.Empty(t, betaWait.Condition)
}

func TestComponents_PreservesExplicitWaitConditions(t *testing.T) {
	alpha := api.NewPackageDefinitionFromV1alpha1(v1alpha1.ZarfPackage{Components: []v1alpha1.ZarfComponent{{Actions: v1alpha1.ZarfComponentActions{OnDeploy: v1alpha1.ZarfComponentActionSet{Before: []v1alpha1.ZarfComponentAction{{Wait: &v1alpha1.ZarfComponentActionWait{Cluster: &v1alpha1.ZarfComponentActionWaitCluster{Kind: "pod", Condition: "Ready"}}}}}}}}})
	beta := api.NewPackageDefinitionFromV1beta1(v1beta1.Package{Components: []v1beta1.Component{{ComponentSpec: v1beta1.ComponentSpec{Actions: v1beta1.ComponentActions{OnDeploy: v1beta1.ComponentActionSet{Before: []v1beta1.ComponentAction{{Wait: &v1beta1.ComponentActionWait{Cluster: &v1beta1.ComponentActionWaitCluster{Kind: "pod", Condition: "Ready"}}}}}}}}}})

	require.Equal(t, "Ready", Components(alpha)[0].Actions.OnDeploy.Before.Actions[0].Wait.Cluster.Condition)
	require.Equal(t, "Ready", Components(beta)[0].Actions.OnDeploy.Before.Actions[0].Wait.Cluster.Condition)
}
