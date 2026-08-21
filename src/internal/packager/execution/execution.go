// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package execution translates package schemas into packager runtime models.
package execution

import (
	"time"

	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/packager/actions"
)

// Component contains the resources for preforming actions on a component
// TODO, convert charts, images, repos, and healthchecks into generic non-versioned types
type Component struct {
	Name           string
	Manifests      []v1alpha1.ZarfManifest
	Charts         []v1alpha1.ZarfChart
	DataInjections []v1alpha1.ZarfDataInjection
	Files          []v1alpha1.ZarfFile
	Images         []string
	ImageArchives  []v1alpha1.ImageArchive
	Repos          []string
	HealthChecks   []v1alpha1.NamespacedObjectKindReference
	StateAccess    []v1alpha1.StateAccessKey
	Actions        ComponentActions
}

// RequiresCluster reports whether this component needs Kubernetes access.
func (c Component) RequiresCluster() bool {
	return len(c.Images) > 0 || len(c.ImageArchives) > 0 || len(c.Charts) > 0 || len(c.Manifests) > 0 || len(c.Repos) > 0 || len(c.DataInjections) > 0 || len(c.HealthChecks) > 0
}

// GetImages returns direct images and images included in archives.
func (c Component) GetImages() []string {
	images := append([]string{}, c.Images...)
	for _, archive := range c.ImageArchives {
		images = append(images, archive.Images...)
	}
	return images
}

// ComponentActions contains runtime action sets for package lifecycle operations.
type ComponentActions struct {
	OnCreate ActionSet
	OnDeploy ActionSet
	OnRemove ActionSet
}

// ActionSet owns lifecycle ordering; actions executes individual action lists.
type ActionSet struct {
	Before    actions.ActionList
	After     actions.ActionList
	OnSuccess actions.ActionList
	OnFailure actions.ActionList
}

// Components projects a definition using the schema it was authored in.
func Components(definition api.PackageDefinition) []Component {
	alpha := definition.AsV1alpha1()
	components := make([]Component, len(alpha.Components))
	for i, component := range alpha.Components {
		components[i] = componentFromAlpha(component)
	}

	if definition.OriginalAPIVersion() == v1beta1.APIVersion {
		beta := definition.AsV1beta1()
		for i, component := range beta.Components {
			components[i].Actions = ComponentActions{
				OnCreate: actionSetFromV1Beta1(component.Actions.OnCreate),
				OnDeploy: actionSetFromV1Beta1(component.Actions.OnDeploy),
				OnRemove: actionSetFromV1Beta1(component.Actions.OnRemove),
			}
		}
		return components
	}

	for i, component := range alpha.Components {
		components[i].Actions = ComponentActions{
			OnCreate: actionSetFromV1Alpha1(component.Actions.OnCreate),
			OnDeploy: actionSetFromV1Alpha1(component.Actions.OnDeploy),
			OnRemove: actionSetFromV1Alpha1(component.Actions.OnRemove),
		}
	}
	return components
}

func componentFromAlpha(component v1alpha1.ZarfComponent) Component {
	return Component{Name: component.Name, Manifests: component.Manifests, Charts: component.Charts, DataInjections: component.DataInjections, Files: component.Files, Images: component.Images, ImageArchives: component.ImageArchives, Repos: component.Repos, HealthChecks: component.HealthChecks, StateAccess: component.StateAccess}
}

func actionSetFromV1Alpha1(set v1alpha1.ZarfComponentActionSet) ActionSet {
	defaults := actions.Config{Silent: set.Defaults.Mute, Timeout: time.Duration(set.Defaults.MaxTotalSeconds) * time.Second, Retries: set.Defaults.MaxRetries, Dir: set.Defaults.Dir, Env: set.Defaults.Env, Shell: alphaShell(set.Defaults.Shell)}
	list := func(in []v1alpha1.ZarfComponentAction) actions.ActionList {
		out := actions.ActionList{Defaults: defaults}
		for _, action := range in {
			out.Actions = append(out.Actions, alphaAction(action))
		}
		return out
	}
	return ActionSet{Before: list(set.Before), After: list(set.After), OnSuccess: list(set.OnSuccess), OnFailure: list(set.OnFailure)}
}

func actionSetFromV1Beta1(set v1beta1.ComponentActionSet) ActionSet {
	defaults := actions.Config{}
	if set.Defaults != nil {
		defaults = actions.Config{Silent: set.Defaults.Silent, Timeout: time.Duration(set.Defaults.MaxTotalSeconds) * time.Second, Retries: int(set.Defaults.Retries), Dir: set.Defaults.Dir, Env: set.Defaults.Env, Shell: betaShell(set.Defaults.Shell)}
	}
	list := func(in []v1beta1.ComponentAction) actions.ActionList {
		out := actions.ActionList{Defaults: defaults}
		for _, action := range in {
			out.Actions = append(out.Actions, betaAction(action))
		}
		return out
	}
	return ActionSet{Before: list(set.Before), OnSuccess: list(set.OnSuccess), OnFailure: list(set.OnFailure)}
}

func alphaAction(action v1alpha1.ZarfComponentAction) actions.Action {
	out := actions.Action{Silent: action.Mute, Dir: action.Dir, Env: action.Env, Cmd: action.Cmd, Description: action.Description, ShouldTemplate: action.ShouldTemplate(), SetVariable: action.DeprecatedSetVariable}
	if action.MaxTotalSeconds != nil {
		timeout := time.Duration(*action.MaxTotalSeconds) * time.Second
		out.Timeout = &timeout
	}
	if action.MaxRetries != nil {
		retries := *action.MaxRetries
		out.Retries = &retries
	}
	if action.Shell != nil {
		shell := alphaShell(*action.Shell)
		out.Shell = &shell
	}
	for _, value := range action.SetValues {
		out.SetValues = append(out.SetValues, actions.ValueOutput{Key: value.Key, Type: actions.ValueOutputType(value.Type)})
	}
	for _, variable := range action.SetVariables {
		out.SetVariables = append(out.SetVariables, actions.VariableOutput{Name: variable.Name, Sensitive: variable.Sensitive, AutoIndent: variable.AutoIndent, Pattern: variable.Pattern, Type: string(variable.Type)})
	}
	if action.Wait != nil {
		out.Wait = alphaWait(action.Wait)
	}
	return out
}

func betaAction(action v1beta1.ComponentAction) actions.Action {
	out := actions.Action{Silent: action.Silent, Dir: action.Dir, Env: action.Env, Cmd: action.Cmd, Description: action.Description, ShouldTemplate: action.EnableTemplating}
	if action.MaxTotalSeconds != nil {
		timeout := time.Duration(*action.MaxTotalSeconds) * time.Second
		out.Timeout = &timeout
	}
	if action.Retries != nil {
		retries := int(*action.Retries)
		out.Retries = &retries
	}
	if action.Shell != nil {
		shell := betaShell(*action.Shell)
		out.Shell = &shell
	}
	for _, value := range action.SetValues {
		out.SetValues = append(out.SetValues, actions.ValueOutput{Key: value.Key, Type: actions.ValueOutputType(value.Type)})
	}
	if action.Wait != nil {
		out.Wait = betaWait(action.Wait)
	}
	return out
}

func alphaWait(waitCfg *v1alpha1.ZarfComponentActionWait) *actions.Wait {
	out := &actions.Wait{}
	if waitCfg.Cluster != nil {
		condition := waitCfg.Cluster.Condition
		if condition == "" {
			condition = "exists"
		}
		out.Cluster = &actions.ClusterWait{Kind: waitCfg.Cluster.Kind, Name: waitCfg.Cluster.Name, Namespace: waitCfg.Cluster.Namespace, Condition: condition}
	}
	if waitCfg.Network != nil {
		out.Network = &actions.NetworkWait{Protocol: waitCfg.Network.Protocol, Address: waitCfg.Network.Address, Code: waitCfg.Network.Code}
	}
	return out
}

func betaWait(waitCfg *v1beta1.ComponentActionWait) *actions.Wait {
	out := &actions.Wait{}
	if waitCfg.Cluster != nil {
		out.Cluster = &actions.ClusterWait{Kind: waitCfg.Cluster.Kind, Name: waitCfg.Cluster.Name, Namespace: waitCfg.Cluster.Namespace, Condition: waitCfg.Cluster.Condition, DefaultReady: true}
	}
	if waitCfg.Network != nil {
		out.Network = &actions.NetworkWait{Protocol: waitCfg.Network.Protocol, Address: waitCfg.Network.Address, Code: int(waitCfg.Network.Code)}
	}
	return out
}

func alphaShell(shell v1alpha1.Shell) actions.Shell {
	return actions.Shell{Windows: shell.Windows, Linux: shell.Linux, Darwin: shell.Darwin}
}

func betaShell(shell v1beta1.Shell) actions.Shell {
	return actions.Shell{Windows: shell.Windows, Linux: shell.Linux, Darwin: shell.Darwin}
}
