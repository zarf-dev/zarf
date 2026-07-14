// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package v1alpha1 contains functions for converting between the public v1alpha1 Zarf package and the internal generic representation.
package v1alpha1

import (
	"strings"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/internal/api/types"
)

// ConvertToGeneric converts a v1alpha1 ZarfPackage to the internal generic representation.
func ConvertToGeneric(pkg v1alpha1.ZarfPackage) types.Package {
	g := types.Package{
		APIVersion: pkg.APIVersion,
		Kind:       string(pkg.Kind),
		Metadata: types.PackageMetadata{
			Name:                     pkg.Metadata.Name,
			Description:              pkg.Metadata.Description,
			Version:                  pkg.Metadata.Version,
			Uncompressed:             pkg.Metadata.Uncompressed,
			Architecture:             pkg.Metadata.Architecture,
			Annotations:              pkg.Metadata.Annotations,
			AllowNamespaceOverride:   pkg.Metadata.AllowNamespaceOverride,
			PreventNamespaceOverride: !pkg.AllowsNamespaceOverride(),
			URL:                      pkg.Metadata.URL,
			Image:                    pkg.Metadata.Image,
			YOLO:                     pkg.Metadata.YOLO,
			Authors:                  pkg.Metadata.Authors,
			Documentation:            pkg.Metadata.Documentation,
			Source:                   pkg.Metadata.Source,
			Vendor:                   pkg.Metadata.Vendor,
			AggregateChecksum:        pkg.Metadata.AggregateChecksum,
		},
		Build: types.BuildData{
			Hostname:                   pkg.Build.Terminal,
			User:                       pkg.Build.User,
			Architecture:               pkg.Build.Architecture,
			Timestamp:                  pkg.Build.Timestamp,
			Version:                    pkg.Build.Version,
			Migrations:                 pkg.Build.Migrations,
			RegistryOverrides:          pkg.Build.RegistryOverrides,
			Differential:               pkg.Build.Differential,
			DifferentialPackageVersion: pkg.Build.DifferentialPackageVersion,
			Flavor:                     pkg.Build.Flavor,
			Signed:                     pkg.Build.Signed,
			DifferentialMissing:        pkg.Build.DifferentialMissing,
			ProvenanceFiles:            pkg.Build.ProvenanceFiles,
			OriginalAPIVersion:         pkg.Build.GetOriginalAPIVersion(),
		},
		Values: types.Values{
			Files:  pkg.Values.Files,
			Schema: pkg.Values.Schema,
		},
		Documentation: pkg.Documentation,
		Variables:     interactiveVarsToGeneric(pkg.Variables),
		Constants:     constantsToGeneric(pkg.Constants),
	}

	for _, vr := range pkg.Build.VersionRequirements {
		g.Build.VersionRequirements = append(g.Build.VersionRequirements, types.VersionRequirement{
			Version: vr.Version,
			Reason:  vr.Reason,
		})
	}

	for _, c := range pkg.Components {
		g.Components = append(g.Components, componentToGeneric(c))
	}

	return g
}

func componentToGeneric(c v1alpha1.ZarfComponent) types.Component {
	gc := types.Component{
		Name:           c.Name,
		Description:    c.Description,
		Default:        c.Default,
		Optional:       !c.IsRequired(),
		Required:       c.Required,
		Group:          c.DeprecatedGroup,
		DataInjections: dataInjectionsToGeneric(c.DataInjections),
		HealthChecks:   healthChecksToGeneric(c.HealthChecks),
		Repositories:   c.Repos,
		StateAccess:    stateAccessToGeneric(c.StateAccess),
		Target: types.ComponentTarget{
			OS:           c.Only.LocalOS,
			Architecture: c.Only.Cluster.Architecture,
			Flavor:       c.Only.Flavor,
		},
		Distros: c.Only.Cluster.Distros,
		Import: types.ComponentImport{
			Name: c.Import.Name,
			Path: c.Import.Path,
			URL:  c.Import.URL,
		},
		Actions: actionsToGeneric(c.Actions),
	}

	for _, m := range c.Manifests {
		gc.Manifests = append(gc.Manifests, manifestToGeneric(m))
	}

	for _, ch := range c.Charts {
		gc.Charts = append(gc.Charts, chartToGeneric(ch))
	}

	for _, f := range c.Files {
		gc.Files = append(gc.Files, types.File{
			Source:           f.Source,
			Checksum:         f.Shasum,
			Destination:      f.Target,
			Executable:       f.Executable,
			Symlinks:         f.Symlinks,
			ExtractPath:      f.ExtractPath,
			EnableTemplating: derefBool(f.Template),
		})
	}

	for _, img := range c.Images {
		gc.Images = append(gc.Images, types.Image{Name: img})
	}

	for _, ia := range c.ImageArchives {
		gc.ImageArchives = append(gc.ImageArchives, types.ImageArchive{
			Path:   ia.Path,
			Images: ia.Images,
		})
	}

	return gc
}

func manifestToGeneric(m v1alpha1.ZarfManifest) types.Manifest {
	gm := types.Manifest{
		Name:             m.Name,
		Namespace:        m.Namespace,
		Files:            m.Files,
		SkipWait:         m.NoWait,
		ServerSideApply:  m.ServerSideApply,
		EnableTemplating: derefBool(m.Template),
		Template:         m.Template,
	}
	if len(m.Kustomizations) > 0 || m.KustomizeAllowAnyDirectory || m.EnableKustomizePlugins {
		gm.Kustomize = &types.KustomizeManifest{
			Files:             m.Kustomizations,
			AllowAnyDirectory: m.KustomizeAllowAnyDirectory,
			EnablePlugins:     m.EnableKustomizePlugins,
		}
	}
	return gm
}

func chartToGeneric(ch v1alpha1.ZarfChart) types.Chart {
	gc := types.Chart{
		Name:                 ch.Name,
		Namespace:            ch.Namespace,
		ReleaseName:          ch.ReleaseName,
		ValuesFiles:          ch.ValuesFiles,
		SkipSchemaValidation: ch.SchemaValidation != nil && !*ch.SchemaValidation,
		ServerSideApply:      ch.ServerSideApply,
		SkipWait:             ch.NoWait,
		URL:                  ch.URL,
		RepoName:             ch.RepoName,
		GitPath:              ch.GitPath,
		LocalPath:            ch.LocalPath,
		Version:              ch.Version,
		SchemaValidation:     ch.SchemaValidation,
		Variables:            chartVarsToGeneric(ch.Variables),
		Values:               chartValuesToGeneric(ch.Values),
		TemplatedValuesFiles: ch.TemplatedValuesFiles,
	}
	return gc
}

func chartValuesToGeneric(vals []v1alpha1.ZarfChartValue) []types.ChartValue {
	var out []types.ChartValue
	for _, v := range vals {
		out = append(out, types.ChartValue{
			SourcePath:   v.SourcePath,
			TargetPath:   v.TargetPath,
			ExcludePaths: v.ExcludePaths,
		})
	}
	return out
}

func actionsToGeneric(a v1alpha1.ZarfComponentActions) types.ComponentActions {
	return types.ComponentActions{
		OnCreate: actionSetToGeneric(a.OnCreate),
		OnDeploy: actionSetToGeneric(a.OnDeploy),
		OnRemove: actionSetToGeneric(a.OnRemove),
	}
}

func actionSetToGeneric(s v1alpha1.ZarfComponentActionSet) types.ComponentActionSet {
	defaults := types.ComponentActionDefaults{
		Silent:          s.Defaults.Mute,
		MaxTotalSeconds: int32(s.Defaults.MaxTotalSeconds),
		Retries:         int32(s.Defaults.MaxRetries),
		Dir:             s.Defaults.Dir,
		Env:             s.Defaults.Env,
		Shell: types.Shell{
			Windows: s.Defaults.Shell.Windows,
			Linux:   s.Defaults.Shell.Linux,
			Darwin:  s.Defaults.Shell.Darwin,
		},
	}

	return types.ComponentActionSet{
		Defaults:  defaults,
		Before:    actionSliceToGeneric(s.Before),
		After:     actionSliceToGeneric(s.After),
		OnSuccess: actionSliceToGeneric(s.OnSuccess),
		OnFailure: actionSliceToGeneric(s.OnFailure),
	}
}

func actionSliceToGeneric(actions []v1alpha1.ZarfComponentAction) []types.ComponentAction {
	var out []types.ComponentAction
	for _, a := range actions {
		out = append(out, actionToGeneric(a))
	}
	return out
}

func actionToGeneric(a v1alpha1.ZarfComponentAction) types.ComponentAction {
	ga := types.ComponentAction{
		Silent:                a.Mute,
		Dir:                   a.Dir,
		Env:                   a.Env,
		Cmd:                   a.Cmd,
		Description:           a.Description,
		Wait:                  waitToGeneric(a.Wait),
		EnableTemplating:      derefBool(a.Template),
		SetVariables:          varsToGeneric(a.SetVariables),
		DeprecatedSetVariable: a.DeprecatedSetVariable,
	}

	if a.MaxTotalSeconds != nil {
		v := int32(*a.MaxTotalSeconds)
		ga.MaxTotalSeconds = &v
	}
	if a.MaxRetries != nil {
		v := int32(*a.MaxRetries)
		ga.Retries = &v
	}

	for _, sv := range a.SetValues {
		ga.SetValues = append(ga.SetValues, types.SetValue{
			Key:   sv.Key,
			Value: sv.Value,
			Type:  string(sv.Type),
		})
	}

	if a.Shell != nil {
		ga.Shell = &types.Shell{
			Windows: a.Shell.Windows,
			Linux:   a.Shell.Linux,
			Darwin:  a.Shell.Darwin,
		}
	}

	return ga
}

func waitToGeneric(w *v1alpha1.ZarfComponentActionWait) *types.ComponentActionWait {
	if w == nil {
		return nil
	}
	gw := &types.ComponentActionWait{}
	if w.Cluster != nil {
		gw.Cluster = &types.ComponentActionWaitCluster{
			Kind:      w.Cluster.Kind,
			Name:      w.Cluster.Name,
			Namespace: w.Cluster.Namespace,
			Condition: w.Cluster.Condition,
		}
	}
	if w.Network != nil {
		gw.Network = &types.ComponentActionWaitNetwork{
			Protocol: w.Network.Protocol,
			Address:  w.Network.Address,
			Code:     int32(w.Network.Code),
		}
	}
	return gw
}

// ConvertFromGeneric converts the internal generic representation to a v1alpha1 ZarfPackage.
func ConvertFromGeneric(g types.Package) v1alpha1.ZarfPackage {
	// An empty source apiVersion is the implicit v1alpha1 form; preserve it so a v1alpha1
	// round-trip stays byte-for-byte lossless.
	apiVersion := v1alpha1.APIVersion
	if g.APIVersion == "" {
		apiVersion = ""
	}
	pkg := v1alpha1.ZarfPackage{
		APIVersion:    apiVersion,
		Kind:          v1alpha1.ZarfPackageKind(g.Kind),
		Metadata:      metadataFromGeneric(g.Metadata, g.Build),
		Build:         buildFromGeneric(g.Build),
		Values:        v1alpha1.ZarfValues{Files: g.Values.Files, Schema: g.Values.Schema},
		Documentation: g.Documentation,
		Variables:     interactiveVarsFromGeneric(g.Variables),
		Constants:     constantsFromGeneric(g.Constants),
	}

	if pkg.Kind == "" {
		pkg.Kind = v1alpha1.ZarfPackageConfig
	}

	// Only a v1alpha1 source carries a meaningful raw Required pointer; an empty apiVersion is the
	// implicit v1alpha1 form (see detectAPIVersion), so anything but an explicit v1beta1 apiVersion
	// is treated as v1alpha1 and its unset Required is preserved verbatim.
	fromV1alpha1 := g.APIVersion != v1beta1.APIVersion

	for _, c := range g.Components {
		pkg.Components = append(pkg.Components, componentFromGeneric(c, fromV1alpha1))
	}

	// A component providing a Zarf CLI service marks this as an init package.
	for _, c := range g.Components {
		if c.Service != "" {
			pkg.Kind = v1alpha1.ZarfInitConfig
			break
		}
	}

	return pkg
}

func metadataFromGeneric(m types.PackageMetadata, b types.BuildData) v1alpha1.ZarfMetadata {
	meta := v1alpha1.ZarfMetadata{
		Name:                   m.Name,
		Description:            m.Description,
		Version:                m.Version,
		Uncompressed:           m.Uncompressed,
		Architecture:           m.Architecture,
		AllowNamespaceOverride: m.AllowNamespaceOverride,
		URL:                    m.URL,
		Image:                  m.Image,
		YOLO:                   m.YOLO,
		Authors:                m.Authors,
		Documentation:          m.Documentation,
		Source:                 m.Source,
		Vendor:                 m.Vendor,
	}

	// AggregateChecksum: prefer the v1alpha1 native location, fall back to build (v1beta1 location).
	switch {
	case m.AggregateChecksum != "":
		meta.AggregateChecksum = m.AggregateChecksum
	case b.AggregateChecksum != "":
		meta.AggregateChecksum = b.AggregateChecksum
	}

	// Restore v1alpha1-only metadata fields from annotations if the generic fields are empty.
	// This handles the case where data originated from v1beta1 and the fields were stored as annotations.
	if m.Annotations != nil {
		restore := map[string]*string{
			"metadata.url":           &meta.URL,
			"metadata.image":         &meta.Image,
			"metadata.authors":       &meta.Authors,
			"metadata.documentation": &meta.Documentation,
			"metadata.source":        &meta.Source,
			"metadata.vendor":        &meta.Vendor,
		}
		annotations := make(map[string]string)
		for k, v := range m.Annotations {
			if target, ok := restore[k]; ok {
				if *target == "" {
					*target = v
				}
				continue
			}
			annotations[k] = v
		}
		if len(annotations) > 0 {
			meta.Annotations = annotations
		}
	}

	return meta
}

func buildFromGeneric(b types.BuildData) v1alpha1.ZarfBuildData {
	out := v1alpha1.ZarfBuildData{
		Terminal:                   b.Hostname,
		User:                       b.User,
		Architecture:               b.Architecture,
		Timestamp:                  b.Timestamp,
		Version:                    b.Version,
		Migrations:                 b.Migrations,
		RegistryOverrides:          b.RegistryOverrides,
		Differential:               b.Differential,
		DifferentialPackageVersion: b.DifferentialPackageVersion,
		DifferentialMissing:        b.DifferentialMissing,
		Flavor:                     b.Flavor,
		Signed:                     b.Signed,
		ProvenanceFiles:            b.ProvenanceFiles,
	}

	for _, vr := range b.VersionRequirements {
		out.VersionRequirements = append(out.VersionRequirements, v1alpha1.VersionRequirement{
			Version: vr.Version,
			Reason:  vr.Reason,
		})
	}

	return out
}

func componentFromGeneric(c types.Component, fromV1alpha1 bool) v1alpha1.ZarfComponent {
	ac := v1alpha1.ZarfComponent{
		Name:            c.Name,
		Description:     c.Description,
		Default:         c.Default,
		Required:        requiredFromGeneric(c.Optional, c.Required, fromV1alpha1),
		DeprecatedGroup: c.Group,
		DataInjections:  dataInjectionsFromGeneric(c.DataInjections),
		HealthChecks:    healthChecksFromGeneric(c.HealthChecks),
		Repos:           c.Repositories,
		StateAccess:     stateAccessFromGeneric(c.StateAccess),
		Only: v1alpha1.ZarfComponentOnlyTarget{
			LocalOS: c.Target.OS,
			Cluster: v1alpha1.ZarfComponentOnlyCluster{
				Architecture: c.Target.Architecture,
				Distros:      c.Distros,
			},
			Flavor: c.Target.Flavor,
		},
		Import: v1alpha1.ZarfComponentImport{
			Name: c.Import.Name,
			Path: c.Import.Path,
			URL:  c.Import.URL,
		},
		Actions: actionsFromGeneric(c.Actions),
	}

	// If the v1alpha1 single-path import fields are empty but the v1beta1 lists have one entry, project it back.
	if ac.Import.Path == "" && len(c.Import.Local) > 0 {
		ac.Import.Path = c.Import.Local[0].Path
	}
	if ac.Import.URL == "" && len(c.Import.Remote) > 0 {
		ac.Import.URL = c.Import.Remote[0].URL
	}

	for _, m := range c.Manifests {
		ac.Manifests = append(ac.Manifests, manifestFromGeneric(m))
	}

	for _, ch := range c.Charts {
		ac.Charts = append(ac.Charts, chartFromGeneric(ch))
	}

	for _, f := range c.Files {
		af := v1alpha1.ZarfFile{
			Source:      f.Source,
			Shasum:      f.Checksum,
			Target:      f.Destination,
			Executable:  f.Executable,
			Symlinks:    f.Symlinks,
			ExtractPath: f.ExtractPath,
		}
		if f.EnableTemplating {
			t := true
			af.Template = &t
		}
		ac.Files = append(ac.Files, af)
	}

	for _, img := range c.Images {
		ac.Images = append(ac.Images, img.Name)
	}

	for _, ia := range c.ImageArchives {
		ac.ImageArchives = append(ac.ImageArchives, v1alpha1.ImageArchive{
			Path:   ia.Path,
			Images: ia.Images,
		})
	}

	return ac
}

// requiredFromGeneric maps the generic representation back to the v1alpha1 Required pointer.
// A v1alpha1 source carries its original Required verbatim, so an unset (nil) value survives the
// round-trip losslessly. A v1beta1 source has no Required, so it is derived by inverting Optional.
func requiredFromGeneric(optional bool, required *bool, fromV1alpha1 bool) *bool {
	if fromV1alpha1 {
		return required
	}
	v := !optional
	return &v
}

func manifestFromGeneric(m types.Manifest) v1alpha1.ZarfManifest {
	am := v1alpha1.ZarfManifest{
		Name:            m.Name,
		Namespace:       m.Namespace,
		Files:           m.Files,
		ServerSideApply: m.ServerSideApply,
		NoWait:          m.SkipWait,
		Template:        m.Template,
	}
	if m.Kustomize != nil {
		am.Kustomizations = m.Kustomize.Files
		am.KustomizeAllowAnyDirectory = m.Kustomize.AllowAnyDirectory
		am.EnableKustomizePlugins = m.Kustomize.EnablePlugins
	}
	if am.Template == nil && m.EnableTemplating {
		t := true
		am.Template = &t
	}
	return am
}

func chartFromGeneric(ch types.Chart) v1alpha1.ZarfChart {
	ac := v1alpha1.ZarfChart{
		Name:                 ch.Name,
		Namespace:            ch.Namespace,
		ReleaseName:          ch.ReleaseName,
		ValuesFiles:          ch.ValuesFiles,
		ServerSideApply:      ch.ServerSideApply,
		NoWait:               ch.SkipWait,
		URL:                  ch.URL,
		RepoName:             ch.RepoName,
		GitPath:              ch.GitPath,
		LocalPath:            ch.LocalPath,
		Version:              ch.Version,
		Variables:            chartVarsFromGeneric(ch.Variables),
		TemplatedValuesFiles: ch.TemplatedValuesFiles,
	}

	// Prefer preserved v1alpha1 SchemaValidation; otherwise derive from SkipSchemaValidation.
	if ch.SchemaValidation != nil {
		ac.SchemaValidation = ch.SchemaValidation
	} else if ch.SkipSchemaValidation {
		f := false
		ac.SchemaValidation = &f
	}

	// If flat fields are empty but structured sources are populated, project them onto the flat fields.
	if ac.URL == "" && ac.LocalPath == "" {
		switch {
		case ch.HelmRepository != nil && ch.HelmRepository.URL != "":
			ac.URL = ch.HelmRepository.URL
			ac.RepoName = ch.HelmRepository.Name
			if ac.Version == "" {
				ac.Version = ch.HelmRepository.Version
			}
		case ch.OCI != nil && ch.OCI.URL != "":
			ac.URL = ch.OCI.URL
			if ac.Version == "" {
				ac.Version = ch.OCI.Version
			}
		case ch.Git != nil && ch.Git.URL != "":
			gitURL := ch.Git.URL
			if idx := strings.LastIndex(gitURL, "@"); idx > 0 {
				if ac.Version == "" {
					ac.Version = gitURL[idx+1:]
				}
				gitURL = gitURL[:idx]
			}
			ac.URL = gitURL
			ac.GitPath = ch.Git.Path
		case ch.Local != nil && ch.Local.Path != "":
			ac.LocalPath = ch.Local.Path
		}
	}

	for _, v := range ch.Values {
		ac.Values = append(ac.Values, v1alpha1.ZarfChartValue{
			SourcePath:   v.SourcePath,
			TargetPath:   v.TargetPath,
			ExcludePaths: v.ExcludePaths,
		})
	}

	return ac
}

func actionsFromGeneric(a types.ComponentActions) v1alpha1.ZarfComponentActions {
	return v1alpha1.ZarfComponentActions{
		OnCreate: actionSetFromGeneric(a.OnCreate),
		OnDeploy: actionSetFromGeneric(a.OnDeploy),
		OnRemove: actionSetFromGeneric(a.OnRemove),
	}
}

func actionSetFromGeneric(s types.ComponentActionSet) v1alpha1.ZarfComponentActionSet {
	defaults := v1alpha1.ZarfComponentActionDefaults{
		Mute:            s.Defaults.Silent,
		MaxTotalSeconds: int(s.Defaults.MaxTotalSeconds),
		MaxRetries:      int(s.Defaults.Retries),
		Dir:             s.Defaults.Dir,
		Env:             s.Defaults.Env,
		Shell: v1alpha1.Shell{
			Windows: s.Defaults.Shell.Windows,
			Linux:   s.Defaults.Shell.Linux,
			Darwin:  s.Defaults.Shell.Darwin,
		},
	}

	return v1alpha1.ZarfComponentActionSet{
		Defaults:  defaults,
		Before:    actionSliceFromGeneric(s.Before),
		After:     actionSliceFromGeneric(s.After),
		OnSuccess: actionSliceFromGeneric(s.OnSuccess),
		OnFailure: actionSliceFromGeneric(s.OnFailure),
	}
}

func actionSliceFromGeneric(actions []types.ComponentAction) []v1alpha1.ZarfComponentAction {
	var out []v1alpha1.ZarfComponentAction
	for _, a := range actions {
		out = append(out, actionFromGeneric(a))
	}
	return out
}

func actionFromGeneric(a types.ComponentAction) v1alpha1.ZarfComponentAction {
	aa := v1alpha1.ZarfComponentAction{
		Mute:                  a.Silent,
		Dir:                   a.Dir,
		Env:                   a.Env,
		Cmd:                   a.Cmd,
		Description:           a.Description,
		Wait:                  waitFromGeneric(a.Wait),
		SetVariables:          varsFromGeneric(a.SetVariables),
		DeprecatedSetVariable: a.DeprecatedSetVariable,
	}

	if a.MaxTotalSeconds != nil {
		v := int(*a.MaxTotalSeconds)
		aa.MaxTotalSeconds = &v
	}
	if a.Retries != nil {
		v := int(*a.Retries)
		aa.MaxRetries = &v
	}
	if a.EnableTemplating {
		t := true
		aa.Template = &t
	}

	for _, sv := range a.SetValues {
		aa.SetValues = append(aa.SetValues, v1alpha1.SetValue{
			Key:   sv.Key,
			Value: sv.Value,
			Type:  v1alpha1.SetValueType(sv.Type),
		})
	}

	if a.Shell != nil {
		aa.Shell = &v1alpha1.Shell{
			Windows: a.Shell.Windows,
			Linux:   a.Shell.Linux,
			Darwin:  a.Shell.Darwin,
		}
	}

	return aa
}

func waitFromGeneric(w *types.ComponentActionWait) *v1alpha1.ZarfComponentActionWait {
	if w == nil {
		return nil
	}
	aw := &v1alpha1.ZarfComponentActionWait{}
	if w.Cluster != nil {
		aw.Cluster = &v1alpha1.ZarfComponentActionWaitCluster{
			Kind:      w.Cluster.Kind,
			Name:      w.Cluster.Name,
			Namespace: w.Cluster.Namespace,
			Condition: w.Cluster.Condition,
		}
	}
	if w.Network != nil {
		aw.Network = &v1alpha1.ZarfComponentActionWaitNetwork{
			Protocol: w.Network.Protocol,
			Address:  w.Network.Address,
			Code:     int(w.Network.Code),
		}
	}
	return aw
}

func derefBool(p *bool) bool {
	if p == nil {
		return false
	}
	return *p
}

func variableToGeneric(v v1alpha1.Variable) types.Variable {
	return types.Variable{
		Name:       v.Name,
		Sensitive:  v.Sensitive,
		AutoIndent: v.AutoIndent,
		Pattern:    v.Pattern,
		Type:       types.VariableType(v.Type),
	}
}

func variableFromGeneric(v types.Variable) v1alpha1.Variable {
	return v1alpha1.Variable{
		Name:       v.Name,
		Sensitive:  v.Sensitive,
		AutoIndent: v.AutoIndent,
		Pattern:    v.Pattern,
		Type:       v1alpha1.VariableType(v.Type),
	}
}

func varsToGeneric(in []v1alpha1.Variable) []types.Variable {
	var out []types.Variable
	for _, v := range in {
		out = append(out, variableToGeneric(v))
	}
	return out
}

func varsFromGeneric(in []types.Variable) []v1alpha1.Variable {
	var out []v1alpha1.Variable
	for _, v := range in {
		out = append(out, variableFromGeneric(v))
	}
	return out
}

func interactiveVarsToGeneric(in []v1alpha1.InteractiveVariable) []types.InteractiveVariable {
	var out []types.InteractiveVariable
	for _, v := range in {
		out = append(out, types.InteractiveVariable{
			Variable:    variableToGeneric(v.Variable),
			Description: v.Description,
			Default:     v.Default,
			Prompt:      v.Prompt,
		})
	}
	return out
}

func interactiveVarsFromGeneric(in []types.InteractiveVariable) []v1alpha1.InteractiveVariable {
	var out []v1alpha1.InteractiveVariable
	for _, v := range in {
		out = append(out, v1alpha1.InteractiveVariable{
			Variable:    variableFromGeneric(v.Variable),
			Description: v.Description,
			Default:     v.Default,
			Prompt:      v.Prompt,
		})
	}
	return out
}

func constantsToGeneric(in []v1alpha1.Constant) []types.Constant {
	var out []types.Constant
	for _, c := range in {
		out = append(out, types.Constant{
			Name:        c.Name,
			Value:       c.Value,
			Description: c.Description,
			AutoIndent:  c.AutoIndent,
			Pattern:     c.Pattern,
		})
	}
	return out
}

func constantsFromGeneric(in []types.Constant) []v1alpha1.Constant {
	var out []v1alpha1.Constant
	for _, c := range in {
		out = append(out, v1alpha1.Constant{
			Name:        c.Name,
			Value:       c.Value,
			Description: c.Description,
			AutoIndent:  c.AutoIndent,
			Pattern:     c.Pattern,
		})
	}
	return out
}

func chartVarsToGeneric(in []v1alpha1.ZarfChartVariable) []types.ZarfChartVariable {
	var out []types.ZarfChartVariable
	for _, v := range in {
		out = append(out, types.ZarfChartVariable{Name: v.Name, Description: v.Description, Path: v.Path})
	}
	return out
}

func chartVarsFromGeneric(in []types.ZarfChartVariable) []v1alpha1.ZarfChartVariable {
	var out []v1alpha1.ZarfChartVariable
	for _, v := range in {
		out = append(out, v1alpha1.ZarfChartVariable{Name: v.Name, Description: v.Description, Path: v.Path})
	}
	return out
}

func dataInjectionsToGeneric(in []v1alpha1.ZarfDataInjection) []types.ZarfDataInjection {
	var out []types.ZarfDataInjection
	for _, d := range in {
		out = append(out, types.ZarfDataInjection{
			Source: d.Source,
			Target: types.ZarfContainerTarget{
				Namespace: d.Target.Namespace,
				Selector:  d.Target.Selector,
				Container: d.Target.Container,
				Path:      d.Target.Path,
			},
			Compress: d.Compress,
		})
	}
	return out
}

func dataInjectionsFromGeneric(in []types.ZarfDataInjection) []v1alpha1.ZarfDataInjection {
	var out []v1alpha1.ZarfDataInjection
	for _, d := range in {
		out = append(out, v1alpha1.ZarfDataInjection{
			Source: d.Source,
			Target: v1alpha1.ZarfContainerTarget{
				Namespace: d.Target.Namespace,
				Selector:  d.Target.Selector,
				Container: d.Target.Container,
				Path:      d.Target.Path,
			},
			Compress: d.Compress,
		})
	}
	return out
}

func healthChecksToGeneric(in []v1alpha1.NamespacedObjectKindReference) []types.NamespacedObjectKindReference {
	var out []types.NamespacedObjectKindReference
	for _, h := range in {
		out = append(out, types.NamespacedObjectKindReference{
			APIVersion: h.APIVersion,
			Kind:       h.Kind,
			Namespace:  h.Namespace,
			Name:       h.Name,
		})
	}
	return out
}

func healthChecksFromGeneric(in []types.NamespacedObjectKindReference) []v1alpha1.NamespacedObjectKindReference {
	var out []v1alpha1.NamespacedObjectKindReference
	for _, h := range in {
		out = append(out, v1alpha1.NamespacedObjectKindReference{
			APIVersion: h.APIVersion,
			Kind:       h.Kind,
			Namespace:  h.Namespace,
			Name:       h.Name,
		})
	}
	return out
}

func stateAccessToGeneric(in []v1alpha1.StateAccessKey) []string {
	var out []string
	for _, s := range in {
		out = append(out, string(s))
	}
	return out
}

func stateAccessFromGeneric(in []string) []v1alpha1.StateAccessKey {
	var out []v1alpha1.StateAccessKey
	for _, s := range in {
		out = append(out, v1alpha1.StateAccessKey(s))
	}
	return out
}
