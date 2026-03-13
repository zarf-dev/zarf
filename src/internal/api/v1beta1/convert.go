// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package v1beta1 contains functions for validating and converting the public v1beta1 Zarf package
package v1beta1

import (
	"strings"
	"time"

	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/internal/api/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConvertFromGeneric converts the internal generic representation to a v1beta1 ZarfPackage.
func ConvertFromGeneric(g types.ZarfPackage) v1beta1.ZarfPackage {
	pkg := v1beta1.ZarfPackage{
		APIVersion: v1beta1.APIVersion,
		Kind:       v1beta1.ZarfPackageKind(g.Kind),
		Metadata:   convertMetadata(g.Metadata),
		Build:      convertBuild(g.Build, g.Metadata),
		Values: v1beta1.ZarfValues{
			Files:  g.Values.Files,
			Schema: g.Values.Schema,
		},
		Documentation: g.Documentation,
	}

	for _, c := range g.Constants {
		pkg.Constants = append(pkg.Constants, v1beta1.Constant{
			Name:        c.Name,
			Value:       c.Value,
			Description: c.Description,
			AutoIndent:  c.AutoIndent,
			Pattern:     c.Pattern,
		})
	}

	for _, v := range g.Variables {
		pkg.Variables = append(pkg.Variables, v1beta1.InteractiveVariable{
			Variable: v1beta1.Variable{
				Name:       v.Name,
				Sensitive:  v.Sensitive,
				AutoIndent: v.AutoIndent,
				Pattern:    v.Pattern,
				Type:       v1beta1.VariableType(v.Type),
			},
			Description: v.Description,
			Default:     v.Default,
			Prompt:      v.Prompt,
		})
	}

	for _, c := range g.Components {
		pkg.Components = append(pkg.Components, convertComponent(c))
	}

	return pkg
}

func convertMetadata(m types.ZarfMetadata) v1beta1.ZarfMetadata {
	meta := v1beta1.ZarfMetadata{
		Name:                   m.Name,
		Description:            m.Description,
		Version:                m.Version,
		Uncompressed:           m.Uncompressed,
		Architecture:           m.Architecture,
		Annotations:            m.Annotations,
		AllowNamespaceOverride: derefBoolOr(m.AllowNamespaceOverride, true),
	}

	// Migrate v1alpha1-only metadata fields into annotations.
	extras := map[string]string{
		"metadata.url":           m.URL,
		"metadata.image":         m.Image,
		"metadata.authors":       m.Authors,
		"metadata.documentation": m.Documentation,
		"metadata.source":        m.Source,
		"metadata.vendor":        m.Vendor,
	}
	for k, v := range extras {
		if v == "" {
			continue
		}
		if meta.Annotations == nil {
			meta.Annotations = make(map[string]string)
		}
		meta.Annotations[k] = v
	}

	return meta
}

func convertBuild(b types.ZarfBuildData, m types.ZarfMetadata) v1beta1.ZarfBuildData {
	out := v1beta1.ZarfBuildData{
		Terminal:                   b.Terminal,
		User:                       b.User,
		Architecture:               b.Architecture,
		Timestamp:                  b.Timestamp,
		Version:                    b.Version,
		Migrations:                 b.Migrations,
		RegistryOverrides:          b.RegistryOverrides,
		Differential:               b.Differential,
		DifferentialPackageVersion: b.DifferentialPackageVersion,
		Flavor:                     b.Flavor,
		Signed:                     b.Signed,
		ProvenanceFiles:            b.ProvenanceFiles,
	}

	// AggregateChecksum lives in metadata in v1alpha1, build in v1beta1.
	if b.AggregateChecksum != "" {
		out.AggregateChecksum = b.AggregateChecksum
	} else if m.AggregateChecksum != "" {
		out.AggregateChecksum = m.AggregateChecksum
	}

	for _, vr := range b.VersionRequirements {
		out.VersionRequirements = append(out.VersionRequirements, v1beta1.VersionRequirement{
			Version: vr.Version,
			Reason:  vr.Reason,
		})
	}

	return out
}

func convertComponent(c types.ZarfComponent) v1beta1.ZarfComponent {
	gc := v1beta1.ZarfComponent{
		Name:        c.Name,
		Description: c.Description,
		Optional:    convertRequiredToOptional(c.Required),
		Only: v1beta1.ZarfComponentOnlyTarget{
			LocalOS: c.Only.LocalOS,
			Cluster: v1beta1.ZarfComponentOnlyCluster{
				Architecture: c.Only.Cluster.Architecture,
				Distros:      c.Only.Cluster.Distros,
			},
			Flavor: c.Only.Flavor,
		},
		Import: v1beta1.ZarfComponentImport{
			Path: c.Import.Path,
			URL:  c.Import.URL,
		},
		Features: v1beta1.ZarfComponentFeatures{
			IsRegistry: c.Features.IsRegistry,
			IsAgent:    c.Features.IsAgent,
		},
		Repos:   c.Repos,
		Actions: convertActions(c.Actions),
	}

	if c.Features.Injector != nil {
		gc.Features.Injector = &v1beta1.Injector{
			Enabled: c.Features.Injector.Enabled,
		}
		if c.Features.Injector.Values != nil {
			gc.Features.Injector.Values = &v1beta1.InjectorValues{
				Tolerations: c.Features.Injector.Values.Tolerations,
			}
		}
	}

	for _, m := range c.Manifests {
		gc.Manifests = append(gc.Manifests, convertManifest(m))
	}

	for _, ch := range c.Charts {
		gc.Charts = append(gc.Charts, convertChart(ch))
	}

	for _, f := range c.Files {
		gc.Files = append(gc.Files, v1beta1.ZarfFile{
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
		gc.Images = append(gc.Images, v1beta1.ZarfImage{
			Name:   img.Name,
			Source: img.Source,
		})
	}

	for _, ia := range c.ImageArchives {
		gc.ImageArchives = append(gc.ImageArchives, v1beta1.ImageArchive{
			Path:   ia.Path,
			Images: ia.Images,
		})
	}

	// Preserve v1alpha1-only fields via private shims for lossless round-tripping.
	gc.SetDataInjections(c.DataInjections)

	return gc
}

// convertRequiredToOptional inverts the v1alpha1 Required *bool into v1beta1 Optional *bool.
// v1alpha1 Required=nil → not required → Optional=nil (default false in v1beta1 means required)
// Wait — v1alpha1 Required=nil means "not required" but v1beta1 Optional=nil means "not optional" (required).
// So Required=nil needs to become Optional=true if the component was truly optional.
// However, the v1alpha1 default is Required=nil meaning "not required" only when Default is false.
// The safest mapping: Required=true → Optional=nil (required), Required=false/nil → Optional=true (optional).
// But actually the semantics differ: in v1alpha1, Required=nil + Default=false means the component
// prompts the user. In v1beta1, Optional=nil means required (no prompt). We preserve Required
// directly and let the caller interpret the v1alpha1 Default/Required/Group semantics.
func convertRequiredToOptional(required *bool) *bool {
	if required == nil {
		return nil
	}
	inverted := !*required
	return &inverted
}

func convertManifest(m types.ZarfManifest) v1beta1.ZarfManifest {
	bm := v1beta1.ZarfManifest{
		Name:                       m.Name,
		Namespace:                  m.Namespace,
		Files:                      m.Files,
		KustomizeAllowAnyDirectory: m.KustomizeAllowAnyDirectory,
		Kustomizations:             m.Kustomizations,
		ServerSideApply:            m.ServerSideApply,
		Template:                   m.Template,
	}

	// Invert NoWait → Wait. If the user explicitly set Wait already, prefer that.
	if m.Wait != nil {
		bm.Wait = m.Wait
	} else if m.NoWait {
		f := false
		bm.Wait = &f
	}

	return bm
}

func convertChart(ch types.ZarfChart) v1beta1.ZarfChart {
	bc := v1beta1.ZarfChart{
		Name:             ch.Name,
		Namespace:        ch.Namespace,
		ReleaseName:      ch.ReleaseName,
		ValuesFiles:      ch.ValuesFiles,
		SchemaValidation: ch.SchemaValidation,
		ServerSideApply:  ch.ServerSideApply,
		Values:           convertChartValues(ch.Values),
	}

	// Invert NoWait → Wait.
	if ch.Wait != nil {
		bc.Wait = ch.Wait
	} else if ch.NoWait {
		f := false
		bc.Wait = &f
	}

	// Convert flat v1alpha1 chart source fields into structured v1beta1 sources.
	// If structured sources are already populated (from a v1beta1 origin), use them directly.
	if ch.HelmRepo != (types.HelmRepoSource{}) {
		bc.HelmRepo = v1beta1.HelmRepoSource{
			Name:    ch.HelmRepo.Name,
			URL:     ch.HelmRepo.URL,
			Version: ch.HelmRepo.Version,
		}
	} else if ch.Git != (types.GitRepoSource{}) {
		bc.Git = v1beta1.GitRepoSource{
			URL:  ch.Git.URL,
			Path: ch.Git.Path,
		}
	} else if ch.Local != (types.LocalRepoSource{}) {
		bc.Local = v1beta1.LocalRepoSource{
			Path: ch.Local.Path,
		}
	} else if ch.OCI != (types.OCISource{}) {
		bc.OCI = v1beta1.OCISource{
			URL:     ch.OCI.URL,
			Version: ch.OCI.Version,
		}
	} else if ch.URL != "" {
		// Infer source type from v1alpha1 flat fields.
		switch {
		case ch.LocalPath != "":
			bc.Local = v1beta1.LocalRepoSource{
				Path: ch.LocalPath,
			}
		case strings.HasPrefix(ch.URL, "oci://"):
			bc.OCI = v1beta1.OCISource{
				URL:     ch.URL,
				Version: ch.Version,
			}
		case ch.GitPath != "" || isGitURL(ch.URL):
			bc.Git = v1beta1.GitRepoSource{
				URL:  ch.URL,
				Path: ch.GitPath,
			}
		default:
			bc.HelmRepo = v1beta1.HelmRepoSource{
				Name:    ch.RepoName,
				URL:     ch.URL,
				Version: ch.Version,
			}
		}
	} else if ch.LocalPath != "" {
		bc.Local = v1beta1.LocalRepoSource{
			Path: ch.LocalPath,
		}
	}

	// Preserve the v1alpha1 flat version via the private shim for lossless round-tripping.
	if ch.Version != "" {
		bc.SetDeprecatedVersion(ch.Version)
	}

	return bc
}

func convertChartValues(vals []types.ZarfChartValue) []v1beta1.ZarfChartValue {
	var out []v1beta1.ZarfChartValue
	for _, v := range vals {
		out = append(out, v1beta1.ZarfChartValue{
			SourcePath: v.SourcePath,
			TargetPath: v.TargetPath,
		})
	}
	return out
}

func convertActions(a types.ZarfComponentActions) v1beta1.ZarfComponentActions {
	return v1beta1.ZarfComponentActions{
		OnCreate: convertActionSet(a.OnCreate),
		OnDeploy: convertActionSet(a.OnDeploy),
		OnRemove: convertActionSet(a.OnRemove),
	}
}

func convertActionSet(s types.ZarfComponentActionSet) v1beta1.ZarfComponentActionSet {
	after := convertActionSlice(s.After)
	// Merge v1alpha1 OnSuccess into After.
	after = append(after, convertActionSlice(s.OnSuccess)...)

	return v1beta1.ZarfComponentActionSet{
		Defaults:  convertActionDefaults(s.Defaults),
		Before:    convertActionSlice(s.Before),
		After:     after,
		OnFailure: convertActionSlice(s.OnFailure),
	}
}

func convertActionDefaults(d types.ZarfComponentActionDefaults) v1beta1.ZarfComponentActionDefaults {
	out := v1beta1.ZarfComponentActionDefaults{
		Mute: d.Mute,
		Dir:  d.Dir,
		Env:  d.Env,
		Shell: v1beta1.Shell{
			Windows: d.Shell.Windows,
			Linux:   d.Shell.Linux,
			Darwin:  d.Shell.Darwin,
		},
	}

	// Prefer the structured Duration if present, otherwise convert MaxTotalSeconds.
	if d.Timeout != nil {
		out.Timeout = d.Timeout
	} else if d.MaxTotalSeconds > 0 {
		dur := metav1.Duration{Duration: time.Duration(d.MaxTotalSeconds) * time.Second}
		out.Timeout = &dur
	}

	// Prefer Retries if set, otherwise convert MaxRetries.
	if d.Retries > 0 {
		out.Retries = d.Retries
	} else if d.MaxRetries > 0 {
		out.Retries = d.MaxRetries
	}

	return out
}

func convertActionSlice(actions []types.ZarfComponentAction) []v1beta1.ZarfComponentAction {
	var out []v1beta1.ZarfComponentAction
	for _, a := range actions {
		out = append(out, convertAction(a))
	}
	return out
}

func convertAction(a types.ZarfComponentAction) v1beta1.ZarfComponentAction {
	ba := v1beta1.ZarfComponentAction{
		Mute:        a.Mute,
		Dir:         a.Dir,
		Env:         a.Env,
		Cmd:         a.Cmd,
		Description: a.Description,
		Wait:        convertWait(a.Wait),
		Template:    a.Template,
		Retries:     a.Retries,
	}

	// Prefer structured Duration if present, otherwise convert MaxTotalSeconds.
	if a.Timeout != nil {
		ba.Timeout = a.Timeout
	} else if a.MaxTotalSeconds != nil {
		dur := metav1.Duration{Duration: time.Duration(*a.MaxTotalSeconds) * time.Second}
		ba.Timeout = &dur
	}

	// Prefer Retries if already set, otherwise convert MaxRetries.
	if ba.Retries == 0 && a.MaxRetries != nil {
		ba.Retries = *a.MaxRetries
	}

	for _, v := range a.SetVariables {
		ba.SetVariables = append(ba.SetVariables, v1beta1.Variable{
			Name:       v.Name,
			Sensitive:  v.Sensitive,
			AutoIndent: v.AutoIndent,
			Pattern:    v.Pattern,
			Type:       v1beta1.VariableType(v.Type),
		})
	}

	// Fold DeprecatedSetVariable into SetVariables if it was set.
	if a.DeprecatedSetVariable != "" {
		ba.SetVariables = append(ba.SetVariables, v1beta1.Variable{
			Name: a.DeprecatedSetVariable,
		})
	}

	for _, sv := range a.SetValues {
		ba.SetValues = append(ba.SetValues, v1beta1.SetValue{
			Key:   sv.Key,
			Value: sv.Value,
			Type:  v1beta1.SetValueType(sv.Type),
		})
	}

	if a.Shell != nil {
		ba.Shell = &v1beta1.Shell{
			Windows: a.Shell.Windows,
			Linux:   a.Shell.Linux,
			Darwin:  a.Shell.Darwin,
		}
	}

	return ba
}

func convertWait(w *types.ZarfComponentActionWait) *v1beta1.ZarfComponentActionWait {
	if w == nil {
		return nil
	}
	bw := &v1beta1.ZarfComponentActionWait{}
	if w.Cluster != nil {
		bw.Cluster = &v1beta1.ZarfComponentActionWaitCluster{
			Kind:      w.Cluster.Kind,
			Name:      w.Cluster.Name,
			Namespace: w.Cluster.Namespace,
			Condition: w.Cluster.Condition,
		}
	}
	if w.Network != nil {
		bw.Network = &v1beta1.ZarfComponentActionWaitNetwork{
			Protocol: w.Network.Protocol,
			Address:  w.Network.Address,
			Code:     w.Network.Code,
		}
	}
	return bw
}

func isGitURL(url string) bool {
	return strings.HasSuffix(url, ".git") ||
		strings.Contains(url, "github.com") ||
		strings.Contains(url, "gitlab.com")
}

func derefBoolOr(p *bool, def bool) bool {
	if p != nil {
		return *p
	}
	return def
}
