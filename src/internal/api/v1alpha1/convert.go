// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package v1alpha1

import (
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/api/types"
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

func derefIntOr(p *int, def int) int {
	if p != nil {
		return *p
	}
	return def
}
