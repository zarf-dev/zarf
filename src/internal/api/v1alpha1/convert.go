// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package v1alpha1

import (
	"math"
	"strings"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/api/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConvertToGeneric converts a v1alpha1 ZarfPackage to the internal generic representation.
func ConvertToGeneric(pkg v1alpha1.ZarfPackage) types.ZarfPackage {
	g := types.ZarfPackage{
		APIVersion: pkg.APIVersion,
		Kind:       string(pkg.Kind),
		Metadata: types.ZarfMetadata{
			Name:                   pkg.Metadata.Name,
			Description:            pkg.Metadata.Description,
			Version:                pkg.Metadata.Version,
			Uncompressed:           pkg.Metadata.Uncompressed,
			Architecture:           pkg.Metadata.Architecture,
			Annotations:            pkg.Metadata.Annotations,
			AllowNamespaceOverride: pkg.Metadata.AllowNamespaceOverride,
			URL:                    pkg.Metadata.URL,
			Image:                  pkg.Metadata.Image,
			YOLO:                   pkg.Metadata.YOLO,
			Authors:                pkg.Metadata.Authors,
			Documentation:          pkg.Metadata.Documentation,
			Source:                 pkg.Metadata.Source,
			Vendor:                 pkg.Metadata.Vendor,
			AggregateChecksum:      pkg.Metadata.AggregateChecksum,
		},
		Build: types.ZarfBuildData{
			Terminal:                   pkg.Build.Terminal,
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
			APIVersion:                 pkg.Build.APIVersion,
			DifferentialMissing:        pkg.Build.DifferentialMissing,
			ProvenanceFiles:            pkg.Build.ProvenanceFiles,
		},
		Values: types.ZarfValues{
			Files:  pkg.Values.Files,
			Schema: pkg.Values.Schema,
		},
		Documentation: pkg.Documentation,
	}

	for _, vr := range pkg.Build.VersionRequirements {
		g.Build.VersionRequirements = append(g.Build.VersionRequirements, types.VersionRequirement{
			Version: vr.Version,
			Reason:  vr.Reason,
		})
	}

	for _, c := range pkg.Constants {
		g.Constants = append(g.Constants, types.Constant{
			Name:        c.Name,
			Value:       c.Value,
			Description: c.Description,
			AutoIndent:  c.AutoIndent,
			Pattern:     c.Pattern,
		})
	}

	for _, v := range pkg.Variables {
		g.Variables = append(g.Variables, types.InteractiveVariable{
			Variable: types.Variable{
				Name:       v.Name,
				Sensitive:  v.Sensitive,
				AutoIndent: v.AutoIndent,
				Pattern:    v.Pattern,
				Type:       string(v.Type),
			},
			Description: v.Description,
			Default:     v.Default,
			Prompt:      v.Prompt,
		})
	}

	for _, c := range pkg.Components {
		g.Components = append(g.Components, convertV1Alpha1Component(c))
	}

	return g
}

func convertV1Alpha1Component(c v1alpha1.ZarfComponent) types.ZarfComponent {
	gc := types.ZarfComponent{
		Name:           c.Name,
		Description:    c.Description,
		Default:        c.Default,
		Required:       c.Required,
		Group:          c.DeprecatedGroup,
		DataInjections: c.DataInjections,
		HealthChecks:   c.HealthChecks,
		Repos:          c.Repos,
		Features:       inferFeaturesFromName(c.Name),
		Only: types.ZarfComponentOnlyTarget{
			LocalOS: c.Only.LocalOS,
			Cluster: types.ZarfComponentOnlyCluster{
				Architecture: c.Only.Cluster.Architecture,
				Distros:      c.Only.Cluster.Distros,
			},
			Flavor: c.Only.Flavor,
		},
		Import: types.ZarfComponentImport{
			Name: c.Import.Name,
			Path: c.Import.Path,
			URL:  c.Import.URL,
		},
		Actions: convertV1Alpha1Actions(c.Actions),
	}

	for _, m := range c.Manifests {
		gc.Manifests = append(gc.Manifests, types.ZarfManifest{
			Name:                       m.Name,
			Namespace:                  m.Namespace,
			Files:                      m.Files,
			KustomizeAllowAnyDirectory: m.KustomizeAllowAnyDirectory,
			Kustomizations:             m.Kustomizations,
			ServerSideApply:            m.ServerSideApply,
			Template:                   m.Template,
			NoWait:                     m.NoWait,
			EnableKustomizePlugins:     m.EnableKustomizePlugins,
		})
	}

	for _, ch := range c.Charts {
		gc.Charts = append(gc.Charts, types.ZarfChart{
			Name:             ch.Name,
			Namespace:        ch.Namespace,
			ReleaseName:      ch.ReleaseName,
			ValuesFiles:      ch.ValuesFiles,
			SchemaValidation: ch.SchemaValidation,
			ServerSideApply:  ch.ServerSideApply,
			NoWait:           ch.NoWait,
			URL:              ch.URL,
			RepoName:         ch.RepoName,
			GitPath:          ch.GitPath,
			LocalPath:        ch.LocalPath,
			Version:          ch.Version,
			Values:           convertV1Alpha1ChartValues(ch.Values),
		})
	}

	for _, f := range c.Files {
		gc.Files = append(gc.Files, types.ZarfFile{
			Source:      f.Source,
			Shasum:      f.Shasum,
			Target:      f.Target,
			Executable:  f.Executable,
			Symlinks:    f.Symlinks,
			ExtractPath: f.ExtractPath,
			Template:    f.Template,
		})
	}

	for _, img := range c.Images {
		gc.Images = append(gc.Images, types.ZarfImage{
			Name: img,
		})
	}

	for _, ia := range c.ImageArchives {
		gc.ImageArchives = append(gc.ImageArchives, types.ImageArchive{
			Path:   ia.Path,
			Images: ia.Images,
		})
	}

	return gc
}

func convertV1Alpha1ChartValues(vals []v1alpha1.ZarfChartValue) []types.ZarfChartValue {
	var out []types.ZarfChartValue
	for _, v := range vals {
		out = append(out, types.ZarfChartValue{
			SourcePath: v.SourcePath,
			TargetPath: v.TargetPath,
		})
	}
	return out
}

func convertV1Alpha1Actions(a v1alpha1.ZarfComponentActions) types.ZarfComponentActions {
	return types.ZarfComponentActions{
		OnCreate: convertV1Alpha1ActionSet(a.OnCreate),
		OnDeploy: convertV1Alpha1ActionSet(a.OnDeploy),
		OnRemove: convertV1Alpha1ActionSet(a.OnRemove),
	}
}

func convertV1Alpha1ActionSet(s v1alpha1.ZarfComponentActionSet) types.ZarfComponentActionSet {
	return types.ZarfComponentActionSet{
		Defaults: types.ZarfComponentActionDefaults{
			Mute:            s.Defaults.Mute,
			MaxTotalSeconds: s.Defaults.MaxTotalSeconds,
			MaxRetries:      s.Defaults.MaxRetries,
			Dir:             s.Defaults.Dir,
			Env:             s.Defaults.Env,
			Shell: types.Shell{
				Windows: s.Defaults.Shell.Windows,
				Linux:   s.Defaults.Shell.Linux,
				Darwin:  s.Defaults.Shell.Darwin,
			},
		},
		Before:    convertV1Alpha1ActionSlice(s.Before),
		After:     convertV1Alpha1ActionSlice(s.After),
		OnSuccess: convertV1Alpha1ActionSlice(s.OnSuccess),
		OnFailure: convertV1Alpha1ActionSlice(s.OnFailure),
	}
}

func convertV1Alpha1ActionSlice(actions []v1alpha1.ZarfComponentAction) []types.ZarfComponentAction {
	var out []types.ZarfComponentAction
	for _, a := range actions {
		out = append(out, convertV1Alpha1Action(a))
	}
	return out
}

func convertV1Alpha1Action(a v1alpha1.ZarfComponentAction) types.ZarfComponentAction {
	ga := types.ZarfComponentAction{
		Mute:                  a.Mute,
		Retries:               derefIntOr(a.MaxRetries, 0),
		Dir:                   a.Dir,
		Env:                   a.Env,
		Cmd:                   a.Cmd,
		Description:           a.Description,
		Wait:                  convertV1Alpha1Wait(a.Wait),
		Template:              a.Template,
		MaxTotalSeconds:       a.MaxTotalSeconds,
		MaxRetries:            a.MaxRetries,
		DeprecatedSetVariable: a.DeprecatedSetVariable,
	}

	for _, v := range a.SetVariables {
		ga.SetVariables = append(ga.SetVariables, types.Variable{
			Name:       v.Name,
			Sensitive:  v.Sensitive,
			AutoIndent: v.AutoIndent,
			Pattern:    v.Pattern,
			Type:       string(v.Type),
		})
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

func convertV1Alpha1Wait(w *v1alpha1.ZarfComponentActionWait) *types.ZarfComponentActionWait {
	if w == nil {
		return nil
	}
	gw := &types.ZarfComponentActionWait{}
	if w.Cluster != nil {
		gw.Cluster = &types.ZarfComponentActionWaitCluster{
			Kind:      w.Cluster.Kind,
			Name:      w.Cluster.Name,
			Namespace: w.Cluster.Namespace,
			Condition: w.Cluster.Condition,
		}
	}
	if w.Network != nil {
		gw.Network = &types.ZarfComponentActionWaitNetwork{
			Protocol: w.Network.Protocol,
			Address:  w.Network.Address,
			Code:     w.Network.Code,
		}
	}
	return gw
}

// inferFeaturesFromName infers v1beta1-style Features from v1alpha1 component names.
// v1alpha1 has no Features field; these well-known component names encode the same semantics.
func inferFeaturesFromName(name string) types.ZarfComponentFeatures {
	switch name {
	case "zarf-registry", "zarf-injector":
		return types.ZarfComponentFeatures{IsRegistry: true}
	case "zarf-seed-registry":
		return types.ZarfComponentFeatures{
			IsRegistry: true,
			Injector:   &types.Injector{Enabled: true},
		}
	case "zarf-agent":
		return types.ZarfComponentFeatures{IsAgent: true}
	default:
		return types.ZarfComponentFeatures{}
	}
}

func derefIntOr(p *int, def int) int {
	if p != nil {
		return *p
	}
	return def
}

// ConvertFromGeneric converts the internal generic representation to a v1alpha1 ZarfPackage.
func ConvertFromGeneric(g types.ZarfPackage) v1alpha1.ZarfPackage {
	pkg := v1alpha1.ZarfPackage{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.ZarfPackageKind(g.Kind),
		Metadata:   convertGenericToV1Alpha1Metadata(g.Metadata, g.Build),
		Build:      convertGenericToV1Alpha1Build(g.Build),
		Values: v1alpha1.ZarfValues{
			Files:  g.Values.Files,
			Schema: g.Values.Schema,
		},
		Documentation: g.Documentation,
	}

	for _, c := range g.Constants {
		pkg.Constants = append(pkg.Constants, v1alpha1.Constant{
			Name:        c.Name,
			Value:       c.Value,
			Description: c.Description,
			AutoIndent:  c.AutoIndent,
			Pattern:     c.Pattern,
		})
	}

	for _, v := range g.Variables {
		pkg.Variables = append(pkg.Variables, v1alpha1.InteractiveVariable{
			Variable: v1alpha1.Variable{
				Name:       v.Name,
				Sensitive:  v.Sensitive,
				AutoIndent: v.AutoIndent,
				Pattern:    v.Pattern,
				Type:       v1alpha1.VariableType(v.Type),
			},
			Description: v.Description,
			Default:     v.Default,
			Prompt:      v.Prompt,
		})
	}

	for _, c := range g.Components {
		pkg.Components = append(pkg.Components, convertGenericToV1Alpha1Component(c))
	}

	return pkg
}

func convertGenericToV1Alpha1Metadata(m types.ZarfMetadata, b types.ZarfBuildData) v1alpha1.ZarfMetadata {
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

	// AggregateChecksum: prefer metadata (v1alpha1 native location), fall back to build (v1beta1 location).
	if m.AggregateChecksum != "" {
		meta.AggregateChecksum = m.AggregateChecksum
	} else if b.AggregateChecksum != "" {
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

func convertGenericToV1Alpha1Build(b types.ZarfBuildData) v1alpha1.ZarfBuildData {
	out := v1alpha1.ZarfBuildData{
		Terminal:                   b.Terminal,
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
		APIVersion:                 b.APIVersion,
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

func convertGenericToV1Alpha1Component(c types.ZarfComponent) v1alpha1.ZarfComponent {
	gc := v1alpha1.ZarfComponent{
		Name:            c.Name,
		Description:     c.Description,
		Default:         c.Default,
		Required:        convertOptionalToRequired(c.Optional, c.Required),
		DeprecatedGroup: c.Group,
		DataInjections:  c.DataInjections,
		HealthChecks:    c.HealthChecks,
		Repos:           c.Repos,
		Only: v1alpha1.ZarfComponentOnlyTarget{
			LocalOS: c.Only.LocalOS,
			Cluster: v1alpha1.ZarfComponentOnlyCluster{
				Architecture: c.Only.Cluster.Architecture,
				Distros:      c.Only.Cluster.Distros,
			},
			Flavor: c.Only.Flavor,
		},
		Import: v1alpha1.ZarfComponentImport{
			Name: c.Import.Name,
			Path: c.Import.Path,
			URL:  c.Import.URL,
		},
		Actions: convertGenericToV1Alpha1Actions(c.Actions),
	}

	for _, m := range c.Manifests {
		gc.Manifests = append(gc.Manifests, convertGenericToV1Alpha1Manifest(m))
	}

	for _, ch := range c.Charts {
		gc.Charts = append(gc.Charts, convertGenericToV1Alpha1Chart(ch))
	}

	for _, f := range c.Files {
		gc.Files = append(gc.Files, v1alpha1.ZarfFile{
			Source:      f.Source,
			Shasum:      f.Shasum,
			Target:      f.Target,
			Executable:  f.Executable,
			Symlinks:    f.Symlinks,
			ExtractPath: f.ExtractPath,
			Template:    f.Template,
		})
	}

	for _, img := range c.Images {
		gc.Images = append(gc.Images, img.Name)
	}

	for _, ia := range c.ImageArchives {
		gc.ImageArchives = append(gc.ImageArchives, v1alpha1.ImageArchive{
			Path:   ia.Path,
			Images: ia.Images,
		})
	}

	return gc
}

// convertOptionalToRequired maps back from the generic superset to v1alpha1 Required.
// If the original Required is preserved (from a v1alpha1 origin), use it directly.
// If only Optional is set (from a v1beta1 origin), invert it.
func convertOptionalToRequired(optional *bool, required *bool) *bool {
	if required != nil {
		return required
	}
	if optional == nil {
		return nil
	}
	inverted := !*optional
	return &inverted
}

func convertGenericToV1Alpha1Manifest(m types.ZarfManifest) v1alpha1.ZarfManifest {
	am := v1alpha1.ZarfManifest{
		Name:                       m.Name,
		Namespace:                  m.Namespace,
		Files:                      m.Files,
		KustomizeAllowAnyDirectory: m.KustomizeAllowAnyDirectory,
		Kustomizations:             m.Kustomizations,
		ServerSideApply:            m.ServerSideApply,
		Template:                   m.Template,
		NoWait:                     m.NoWait,
		EnableKustomizePlugins:     m.EnableKustomizePlugins,
	}

	// Invert Wait → NoWait if Wait was explicitly set and NoWait wasn't already.
	if !am.NoWait && m.Wait != nil {
		am.NoWait = !*m.Wait
	}

	return am
}

func convertGenericToV1Alpha1Chart(ch types.ZarfChart) v1alpha1.ZarfChart {
	ac := v1alpha1.ZarfChart{
		Name:             ch.Name,
		Namespace:        ch.Namespace,
		ReleaseName:      ch.ReleaseName,
		ValuesFiles:      ch.ValuesFiles,
		SchemaValidation: ch.SchemaValidation,
		ServerSideApply:  ch.ServerSideApply,
		NoWait:           ch.NoWait,
		URL:              ch.URL,
		RepoName:         ch.RepoName,
		GitPath:          ch.GitPath,
		LocalPath:        ch.LocalPath,
		Version:          ch.Version,
	}

	// Invert Wait → NoWait if Wait was explicitly set and NoWait wasn't already.
	if !ac.NoWait && ch.Wait != nil {
		ac.NoWait = !*ch.Wait
	}

	// If flat fields are empty but structured sources are populated, convert back to flat.
	if ac.URL == "" && ac.LocalPath == "" {
		switch {
		case ch.HelmRepo.URL != "":
			ac.URL = ch.HelmRepo.URL
			ac.RepoName = ch.HelmRepo.Name
			if ac.Version == "" {
				ac.Version = ch.HelmRepo.Version
			}
		case ch.OCI.URL != "":
			ac.URL = ch.OCI.URL
			if ac.Version == "" {
				ac.Version = ch.OCI.Version
			}
		case ch.Git.URL != "":
			gitURL := ch.Git.URL
			if idx := strings.LastIndex(gitURL, "@"); idx > 0 {
				if ac.Version == "" {
					ac.Version = gitURL[idx+1:]
				}
				gitURL = gitURL[:idx]
			}
			ac.URL = gitURL
			ac.GitPath = ch.Git.Path
		case ch.Local.Path != "":
			ac.LocalPath = ch.Local.Path
		}
	}

	for _, v := range ch.Values {
		ac.Values = append(ac.Values, v1alpha1.ZarfChartValue{
			SourcePath: v.SourcePath,
			TargetPath: v.TargetPath,
		})
	}

	return ac
}

func convertGenericToV1Alpha1Actions(a types.ZarfComponentActions) v1alpha1.ZarfComponentActions {
	return v1alpha1.ZarfComponentActions{
		OnCreate: convertGenericToV1Alpha1ActionSet(a.OnCreate),
		OnDeploy: convertGenericToV1Alpha1ActionSet(a.OnDeploy),
		OnRemove: convertGenericToV1Alpha1ActionSet(a.OnRemove),
	}
}

func convertGenericToV1Alpha1ActionSet(s types.ZarfComponentActionSet) v1alpha1.ZarfComponentActionSet {
	return v1alpha1.ZarfComponentActionSet{
		Defaults: v1alpha1.ZarfComponentActionDefaults{
			Mute:            s.Defaults.Mute,
			MaxTotalSeconds: convertMaxTotalSeconds(s.Defaults.MaxTotalSeconds, s.Defaults.Timeout),
			MaxRetries:      convertMaxRetries(s.Defaults.MaxRetries, s.Defaults.Retries),
			Dir:             s.Defaults.Dir,
			Env:             s.Defaults.Env,
			Shell: v1alpha1.Shell{
				Windows: s.Defaults.Shell.Windows,
				Linux:   s.Defaults.Shell.Linux,
				Darwin:  s.Defaults.Shell.Darwin,
			},
		},
		Before:    convertGenericToV1Alpha1ActionSlice(s.Before),
		After:     convertGenericToV1Alpha1ActionSlice(s.After),
		OnSuccess: convertGenericToV1Alpha1ActionSlice(s.OnSuccess),
		OnFailure: convertGenericToV1Alpha1ActionSlice(s.OnFailure),
	}
}

// convertMaxTotalSeconds returns the v1alpha1 MaxTotalSeconds value.
// Prefers the preserved v1alpha1 value; falls back to converting the Duration.
func convertMaxTotalSeconds(v1alpha1Val int, timeout *metav1.Duration) int {
	if v1alpha1Val > 0 {
		return v1alpha1Val
	}
	if timeout != nil {
		secs := timeout.Duration.Seconds()
		if secs > math.MaxInt32 {
			return math.MaxInt32
		}
		return int(secs)
	}
	return 0
}

// convertMaxRetries returns the v1alpha1 MaxRetries value.
// Prefers the preserved v1alpha1 value; falls back to the generic Retries.
func convertMaxRetries(v1alpha1Val int, retries int) int {
	if v1alpha1Val > 0 {
		return v1alpha1Val
	}
	return retries
}

func convertGenericToV1Alpha1ActionSlice(actions []types.ZarfComponentAction) []v1alpha1.ZarfComponentAction {
	var out []v1alpha1.ZarfComponentAction
	for _, a := range actions {
		out = append(out, convertGenericToV1Alpha1Action(a))
	}
	return out
}

func convertGenericToV1Alpha1Action(a types.ZarfComponentAction) v1alpha1.ZarfComponentAction {
	ga := v1alpha1.ZarfComponentAction{
		Mute:                  a.Mute,
		MaxTotalSeconds:       a.MaxTotalSeconds,
		MaxRetries:            a.MaxRetries,
		Dir:                   a.Dir,
		Env:                   a.Env,
		Cmd:                   a.Cmd,
		Description:           a.Description,
		Wait:                  convertGenericToV1Alpha1Wait(a.Wait),
		Template:              a.Template,
		DeprecatedSetVariable: a.DeprecatedSetVariable,
	}

	// If preserved v1alpha1 MaxTotalSeconds is nil but we have a Duration, convert it.
	if ga.MaxTotalSeconds == nil && a.Timeout != nil {
		secs := int(a.Timeout.Duration.Seconds())
		ga.MaxTotalSeconds = &secs
	}

	// If preserved v1alpha1 MaxRetries is nil but we have Retries, convert it.
	if ga.MaxRetries == nil && a.Retries > 0 {
		ga.MaxRetries = &a.Retries
	}

	for _, v := range a.SetVariables {
		ga.SetVariables = append(ga.SetVariables, v1alpha1.Variable{
			Name:       v.Name,
			Sensitive:  v.Sensitive,
			AutoIndent: v.AutoIndent,
			Pattern:    v.Pattern,
			Type:       v1alpha1.VariableType(v.Type),
		})
	}

	for _, sv := range a.SetValues {
		ga.SetValues = append(ga.SetValues, v1alpha1.SetValue{
			Key:   sv.Key,
			Value: sv.Value,
			Type:  v1alpha1.SetValueType(sv.Type),
		})
	}

	if a.Shell != nil {
		ga.Shell = &v1alpha1.Shell{
			Windows: a.Shell.Windows,
			Linux:   a.Shell.Linux,
			Darwin:  a.Shell.Darwin,
		}
	}

	return ga
}

func convertGenericToV1Alpha1Wait(w *types.ZarfComponentActionWait) *v1alpha1.ZarfComponentActionWait {
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
			Code:     w.Network.Code,
		}
	}
	return aw
}
