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

type (
	// Manifest holds the manifest information needed at runtime
	Manifest = v1alpha1.ZarfManifest
	// File holds the file information needed at runtime
	File = v1alpha1.ZarfFile
	// NamespacedObjectKindReference holds health check object information needed at runtime
	NamespacedObjectKindReference = v1alpha1.NamespacedObjectKindReference
	// StateAccessKey identifies a state value needed at runtime
	StateAccessKey = v1alpha1.StateAccessKey

	// ImageArchive Holds the Image archive information needed at runtime
	// 	TODO: introduce generic images.Archive type in images package
	ImageArchive = v1alpha1.ImageArchive
	// Chart holds the Helm chart information needed at runtime
	// TODO: introduce generic helm.Chart type in Helm package
	Chart = v1alpha1.ZarfChart
	// DataInjection will stay type aliased to v1alpha1 since there is no v1beta1 equivalent.
	DataInjection = v1alpha1.ZarfDataInjection
)

// Component contains the resources needed to deploy a component.
type Component struct {
	Name           string
	Manifests      []Manifest
	Charts         []Chart
	DataInjections []DataInjection
	Files          []File
	Images         []string
	ImageArchives  []ImageArchive
	Repos          []string
	HealthChecks   []NamespacedObjectKindReference
	StateAccess    []StateAccessKey
	Actions        ComponentActions
}

// RequiresCluster reports whether this component needs Kubernetes access.
func (c Component) RequiresCluster() bool {
	return len(c.Images) > 0 ||
		len(c.ImageArchives) > 0 ||
		len(c.Charts) > 0 ||
		len(c.Manifests) > 0 ||
		len(c.Repos) > 0 ||
		len(c.DataInjections) > 0 ||
		len(c.HealthChecks) > 0
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
	Defaults  actions.Config
	Before    actions.ActionList
	After     actions.ActionList
	OnSuccess actions.ActionList
	OnFailure actions.ActionList
}

// Components returns generic components for execution logic
func Components(definition api.PackageDefinition) []Component {
	alpha := definition.AsV1alpha1()
	originalAPIVersion := definition.OriginalAPIVersion()
	components := make([]Component, len(alpha.Components))
	for i, component := range alpha.Components {
		components[i] = componentFromDefinition(component, originalAPIVersion)
	}
	return components
}

func componentFromDefinition(component v1alpha1.ZarfComponent, originalAPIVersion string) Component {
	return Component{
		Name:           component.Name,
		Manifests:      component.Manifests,
		Charts:         component.Charts,
		DataInjections: component.DataInjections,
		Files:          component.Files,
		Images:         component.Images,
		ImageArchives:  component.ImageArchives,
		Repos:          component.Repos,
		HealthChecks:   component.HealthChecks,
		StateAccess:    component.StateAccess,
		Actions: ComponentActions{
			OnCreate: actionSet(component.Actions.OnCreate, originalAPIVersion),
			OnDeploy: actionSet(component.Actions.OnDeploy, originalAPIVersion),
			OnRemove: actionSet(component.Actions.OnRemove, originalAPIVersion),
		},
	}
}

func actionSet(set v1alpha1.ZarfComponentActionSet, originalAPIVersion string) ActionSet {
	defaults := actions.Config{
		Silent:  set.Defaults.Mute,
		Timeout: time.Duration(set.Defaults.MaxTotalSeconds) * time.Second,
		Retries: set.Defaults.MaxRetries,
		Dir:     set.Defaults.Dir,
		Env:     set.Defaults.Env,
		Shell:   shellFromDefinition(set.Defaults.Shell),
	}
	list := func(in []v1alpha1.ZarfComponentAction) actions.ActionList {
		out := actions.ActionList{}
		for _, action := range in {
			out.Actions = append(out.Actions, actionFromDefinition(action, originalAPIVersion))
		}
		return out
	}
	return ActionSet{
		Defaults:  defaults,
		Before:    list(set.Before),
		After:     list(set.After),
		OnSuccess: list(set.OnSuccess),
		OnFailure: list(set.OnFailure),
	}
}

func actionFromDefinition(action v1alpha1.ZarfComponentAction, originalAPIVersion string) actions.Action {
	out := actions.Action{
		Silent:         action.Mute,
		Dir:            action.Dir,
		Env:            action.Env,
		Cmd:            action.Cmd,
		Description:    action.Description,
		ShouldTemplate: action.ShouldTemplate(),
		SetVariable:    action.DeprecatedSetVariable,
	}
	if action.MaxTotalSeconds != nil {
		timeout := time.Duration(*action.MaxTotalSeconds) * time.Second
		out.Timeout = &timeout
	}
	if action.MaxRetries != nil {
		retries := *action.MaxRetries
		out.Retries = &retries
	}
	if action.Shell != nil {
		shell := shellFromDefinition(*action.Shell)
		out.Shell = &shell
	}
	for _, value := range action.SetValues {
		out.SetValues = append(out.SetValues, actions.ValueOutput{
			Key:  value.Key,
			Type: actions.ValueOutputType(value.Type),
		})
	}
	for _, variable := range action.SetVariables {
		out.SetVariables = append(out.SetVariables, actions.VariableOutput{
			Name:       variable.Name,
			Sensitive:  variable.Sensitive,
			AutoIndent: variable.AutoIndent,
			Pattern:    variable.Pattern,
			Type:       string(variable.Type),
		})
	}
	if action.Wait != nil {
		out.Wait = waitFromDefinition(action.Wait, originalAPIVersion)
	}
	return out
}

func waitFromDefinition(waitCfg *v1alpha1.ZarfComponentActionWait, originalAPIVersion string) *actions.Wait {
	out := &actions.Wait{}
	if waitCfg.Cluster != nil {
		defaultCondition := actions.DefaultConditionExists
		if originalAPIVersion == v1beta1.APIVersion {
			defaultCondition = actions.DefaultConditionReady
		}
		out.Cluster = &actions.ClusterWait{
			Kind:             waitCfg.Cluster.Kind,
			Name:             waitCfg.Cluster.Name,
			Namespace:        waitCfg.Cluster.Namespace,
			Condition:        waitCfg.Cluster.Condition,
			DefaultCondition: defaultCondition,
		}
	}
	if waitCfg.Network != nil {
		out.Network = &actions.NetworkWait{
			Protocol: waitCfg.Network.Protocol,
			Address:  waitCfg.Network.Address,
			Code:     waitCfg.Network.Code,
		}
	}
	return out
}

func shellFromDefinition(shell v1alpha1.Shell) actions.Shell {
	return actions.Shell{
		Windows: shell.Windows,
		Linux:   shell.Linux,
		Darwin:  shell.Darwin,
	}
}
