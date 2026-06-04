// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package v1beta1 contains functions for converting between the public v1beta1 Zarf package and the internal generic representation.
package v1beta1

import (
	"strings"

	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/internal/api/types"
)

// ConvertToGeneric converts a v1beta1 Package to the internal generic representation.
func ConvertToGeneric(pkg v1beta1.Package) types.Package {
	// Preserve an already-recorded original across multi-hop conversions; otherwise this is the original.
	originalAPIVersion := pkg.Build.GetOriginalAPIVersion()
	if originalAPIVersion == "" {
		originalAPIVersion = pkg.APIVersion
	}
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
			PreventNamespaceOverride: pkg.Metadata.PreventNamespaceOverride,
		},
		Build: types.BuildData{
			Hostname:                   pkg.Build.Hostname,
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
			ProvenanceFiles:            pkg.Build.ProvenanceFiles,
			AggregateChecksum:          pkg.Build.AggregateChecksum,
			OriginalAPIVersion:         originalAPIVersion,
		},
		Values: types.Values{
			Files:  pkg.Values.Files,
			Schema: pkg.Values.Schema,
		},
		Documentation: pkg.Documentation,
		Variables:     deprecatedVarsToGeneric(pkg.GetDeprecatedVariables()),
		Constants:     deprecatedConstantsToGeneric(pkg.GetDeprecatedConstants()),
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

func componentToGeneric(c v1beta1.Component) types.Component {
	gc := types.Component{
		Name:         c.Name,
		Description:  c.Description,
		Optional:     c.Optional,
		Service:      string(c.Service),
		Repositories: c.Repositories,
		Target: types.ComponentTarget{
			OS:           c.Target.OS,
			Architecture: c.Target.Architecture,
			Flavor:       c.Target.Flavor,
		},
		Import:         importToGeneric(c.Import),
		Actions:        actionsToGeneric(c.Actions),
		DataInjections: deprecatedDataInjectionsToGeneric(c.GetDeprecatedDataInjections()),
	}

	for _, m := range c.Manifests {
		gc.Manifests = append(gc.Manifests, manifestToGeneric(m))
	}

	for _, ch := range c.Charts {
		gc.Charts = append(gc.Charts, chartToGeneric(ch))
	}

	for _, f := range c.Files {
		gc.Files = append(gc.Files, types.File{
			Source:       f.Source,
			Checksum:     f.Checksum,
			Destination:  f.Destination,
			Executable:   f.Executable,
			Symlinks:     f.Symlinks,
			ExtractPath:  f.ExtractPath,
			EnableValues: f.EnableValues,
		})
	}

	for _, img := range c.Images {
		gc.Images = append(gc.Images, types.Image{
			Name:   img.Name,
			Source: img.Source,
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

func importToGeneric(imp v1beta1.ComponentImport) types.ComponentImport {
	out := types.ComponentImport{}
	for _, l := range imp.Local {
		out.Local = append(out.Local, types.ComponentImportLocal{Path: l.Path})
	}
	for _, r := range imp.Remote {
		out.Remote = append(out.Remote, types.ComponentImportRemote{URL: r.URL})
	}
	return out
}

func manifestToGeneric(m v1beta1.Manifest) types.Manifest {
	gm := types.Manifest{
		Name:            m.Name,
		Namespace:       m.Namespace,
		Files:           m.Files,
		SkipWait:        m.SkipWait,
		ServerSideApply: string(m.ServerSideApply),
		EnableValues:    m.EnableValues,
	}
	if m.Kustomize != nil {
		gm.Kustomize = &types.KustomizeManifest{
			Files:             m.Kustomize.Files,
			AllowAnyDirectory: m.Kustomize.AllowAnyDirectory,
			EnablePlugins:     m.Kustomize.EnablePlugins,
		}
	}
	return gm
}

func chartToGeneric(ch v1beta1.Chart) types.Chart {
	gc := types.Chart{
		Name:                 ch.Name,
		Namespace:            ch.Namespace,
		ReleaseName:          ch.ReleaseName,
		ValuesFiles:          ch.ValuesFiles,
		SkipSchemaValidation: ch.SkipSchemaValidation,
		ServerSideApply:      string(ch.ServerSideApply),
		SkipWait:             ch.SkipWait,
		Version:              ch.GetDeprecatedVersion(),
		Variables:            deprecatedChartVarsToGeneric(ch.GetDeprecatedVariables()),
	}

	if ch.HelmRepository != nil {
		gc.HelmRepository = &types.HelmRepositorySource{
			Name:    ch.HelmRepository.Name,
			URL:     ch.HelmRepository.URL,
			Version: ch.HelmRepository.Version,
		}
	}
	if ch.Git != nil {
		gc.Git = &types.GitSource{
			URL:  ch.Git.URL,
			Path: ch.Git.Path,
		}
	}
	if ch.Local != nil {
		gc.Local = &types.LocalSource{Path: ch.Local.Path}
	}
	if ch.OCI != nil {
		gc.OCI = &types.OCISource{
			URL:     ch.OCI.URL,
			Version: ch.OCI.Version,
		}
	}

	for _, v := range ch.Values {
		gc.Values = append(gc.Values, types.ChartValue{
			SourcePath: v.SourcePath,
			TargetPath: v.TargetPath,
		})
	}

	return gc
}

func actionsToGeneric(a v1beta1.ComponentActions) types.ComponentActions {
	return types.ComponentActions{
		OnCreate: actionSetToGeneric(a.OnCreate),
		OnDeploy: actionSetToGeneric(a.OnDeploy),
		OnRemove: actionSetToGeneric(a.OnRemove),
	}
}

func actionSetToGeneric(s v1beta1.ComponentActionSet) types.ComponentActionSet {
	return types.ComponentActionSet{
		Defaults: types.ComponentActionDefaults{
			Silent:          s.Defaults.Silent,
			MaxTotalSeconds: s.Defaults.MaxTotalSeconds,
			Retries:         s.Defaults.Retries,
			Dir:             s.Defaults.Dir,
			Env:             s.Defaults.Env,
			Shell: types.Shell{
				Windows: s.Defaults.Shell.Windows,
				Linux:   s.Defaults.Shell.Linux,
				Darwin:  s.Defaults.Shell.Darwin,
			},
		},
		Before:    actionSliceToGeneric(s.Before),
		OnSuccess: actionSliceToGeneric(s.OnSuccess),
		OnFailure: actionSliceToGeneric(s.OnFailure),
	}
}

func actionSliceToGeneric(actions []v1beta1.ComponentAction) []types.ComponentAction {
	var out []types.ComponentAction
	for _, a := range actions {
		out = append(out, actionToGeneric(a))
	}
	return out
}

func actionToGeneric(a v1beta1.ComponentAction) types.ComponentAction {
	ga := types.ComponentAction{
		Silent:          a.Silent,
		MaxTotalSeconds: a.MaxTotalSeconds,
		Retries:         a.Retries,
		Dir:             a.Dir,
		Env:             a.Env,
		Cmd:             a.Cmd,
		Description:     a.Description,
		Wait:            waitToGeneric(a.Wait),
		EnableValues:    a.EnableValues,
		SetVariables:    deprecatedSetVarsToGeneric(a.GetDeprecatedSetVariables()),
	}

	for _, sv := range a.SetValues {
		ga.SetValues = append(ga.SetValues, types.SetValue{
			Key:  sv.Key,
			Type: string(sv.Type),
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

func waitToGeneric(w *v1beta1.ComponentActionWait) *types.ComponentActionWait {
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
			Code:     w.Network.Code,
		}
	}
	return gw
}

// ConvertFromGeneric converts the internal generic representation to a v1beta1 Package.
func ConvertFromGeneric(g types.Package) v1beta1.Package {
	pkg := v1beta1.Package{
		APIVersion:    v1beta1.APIVersion,
		Kind:          v1beta1.PackageKind(g.Kind),
		Metadata:      metadataFromGeneric(g.Metadata),
		Build:         buildFromGeneric(g.Build, g.Metadata),
		Values:        v1beta1.Values{Files: g.Values.Files, Schema: g.Values.Schema},
		Documentation: g.Documentation,
	}

	if pkg.Kind == "" {
		pkg.Kind = v1beta1.ZarfPackageConfig
	}

	// v1beta1 has no Kind ZarfInitConfig; collapse the v1alpha1 init kind into the normal package kind.
	if string(pkg.Kind) == "ZarfInitConfig" {
		pkg.Kind = v1beta1.ZarfPackageConfig
	}

	for _, c := range g.Components {
		pkg.Components = append(pkg.Components, componentFromGeneric(c))
	}

	return pkg
}

func metadataFromGeneric(m types.PackageMetadata) v1beta1.PackageMetadata {
	meta := v1beta1.PackageMetadata{
		Name:         m.Name,
		Description:  m.Description,
		Version:      m.Version,
		Uncompressed: m.Uncompressed,
		Architecture: m.Architecture,
		Annotations:  m.Annotations,
	}

	// Map v1alpha1 AllowNamespaceOverride (*bool, default allow) onto v1beta1 PreventNamespaceOverride (bool, default allow).
	if m.AllowNamespaceOverride != nil {
		meta.PreventNamespaceOverride = !*m.AllowNamespaceOverride
	} else {
		meta.PreventNamespaceOverride = m.PreventNamespaceOverride
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

func buildFromGeneric(b types.BuildData, m types.PackageMetadata) v1beta1.BuildData {
	out := v1beta1.BuildData{
		Hostname:                   b.Hostname,
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
	switch {
	case b.AggregateChecksum != "":
		out.AggregateChecksum = b.AggregateChecksum
	case m.AggregateChecksum != "":
		out.AggregateChecksum = m.AggregateChecksum
	}

	for _, vr := range b.VersionRequirements {
		out.VersionRequirements = append(out.VersionRequirements, v1beta1.VersionRequirement{
			Version: vr.Version,
			Reason:  vr.Reason,
		})
	}

	out.SetOriginalAPIVersion(b.OriginalAPIVersion)

	return out
}

func componentFromGeneric(c types.Component) v1beta1.Component {
	bc := v1beta1.Component{
		Name:        c.Name,
		Description: c.Description,
		Optional:    optionalFromGeneric(c.Optional, c.Required),
		ComponentSpec: v1beta1.ComponentSpec{
			Repositories: c.Repositories,
			Target: v1beta1.ComponentTarget{
				OS:           c.Target.OS,
				Architecture: c.Target.Architecture,
				Flavor:       c.Target.Flavor,
			},
			Import:  importFromGeneric(c.Import),
			Service: serviceFromGeneric(c),
			Actions: actionsFromGeneric(c.Actions),
		},
	}

	for _, m := range c.Manifests {
		bc.Manifests = append(bc.Manifests, manifestFromGeneric(m))
	}

	for _, ch := range c.Charts {
		bc.Charts = append(bc.Charts, chartFromGeneric(ch))
	}

	for _, f := range c.Files {
		bc.Files = append(bc.Files, v1beta1.File{
			Source:       f.Source,
			Checksum:     f.Checksum,
			Destination:  f.Destination,
			Executable:   f.Executable,
			Symlinks:     f.Symlinks,
			ExtractPath:  f.ExtractPath,
			EnableValues: f.EnableValues,
		})
	}

	for _, img := range c.Images {
		bc.Images = append(bc.Images, v1beta1.Image{
			Name:   img.Name,
			Source: img.Source,
		})
	}

	for _, ia := range c.ImageArchives {
		bc.ImageArchives = append(bc.ImageArchives, v1beta1.ImageArchive{
			Path:   ia.Path,
			Images: ia.Images,
		})
	}

	// Convert v1alpha1 HealthChecks into onDeploy onSuccess wait actions.
	for _, hc := range c.HealthChecks {
		bc.Actions.OnDeploy.OnSuccess = append(bc.Actions.OnDeploy.OnSuccess, v1beta1.ComponentAction{
			Wait: &v1beta1.ComponentActionWait{
				Cluster: &v1beta1.ComponentActionWaitCluster{
					Kind:      healthCheckKind(hc.Kind, hc.APIVersion),
					Name:      hc.Name,
					Namespace: hc.Namespace,
				},
			},
		})
	}

	return bc
}

// optionalFromGeneric maps the v1alpha1 Required *bool and v1beta1 Optional bool onto a single v1beta1 Optional bool.
// v1alpha1: Required=nil/false → Optional=true; Required=true → Optional=false.
// v1beta1: Optional flows through directly when Required is nil.
func optionalFromGeneric(optional bool, required *bool) bool {
	if required != nil {
		return !*required
	}
	return optional
}

func serviceFromGeneric(c types.Component) v1beta1.Service {
	if c.Service != "" {
		return v1beta1.Service(c.Service)
	}
	// Infer the v1beta1 Service from well-known v1alpha1 component names.
	switch c.Name {
	case "zarf-registry":
		return v1beta1.ServiceRegistry
	case "zarf-seed-registry":
		return v1beta1.ServiceSeedRegistry
	case "zarf-injector":
		return v1beta1.ServiceInjector
	case "zarf-agent":
		return v1beta1.ServiceAgent
	case "git-server":
		return v1beta1.ServiceGitServer
	}
	return ""
}

func importFromGeneric(imp types.ComponentImport) v1beta1.ComponentImport {
	out := v1beta1.ComponentImport{}
	for _, l := range imp.Local {
		out.Local = append(out.Local, v1beta1.ComponentImportLocal{Path: l.Path})
	}
	for _, r := range imp.Remote {
		out.Remote = append(out.Remote, v1beta1.ComponentImportRemote{URL: r.URL})
	}
	// Promote v1alpha1 single-import fields when no structured imports are present.
	if len(out.Local) == 0 && imp.Path != "" {
		out.Local = append(out.Local, v1beta1.ComponentImportLocal{Path: imp.Path})
	}
	if len(out.Remote) == 0 && imp.URL != "" {
		out.Remote = append(out.Remote, v1beta1.ComponentImportRemote{URL: imp.URL})
	}
	return out
}

func manifestFromGeneric(m types.Manifest) v1beta1.Manifest {
	bm := v1beta1.Manifest{
		Name:            m.Name,
		Namespace:       m.Namespace,
		Files:           m.Files,
		SkipWait:        m.SkipWait,
		ServerSideApply: v1beta1.ServerSideApplyMode(m.ServerSideApply),
		EnableValues:    m.EnableValues,
	}
	if m.Kustomize != nil {
		bm.Kustomize = &v1beta1.KustomizeManifest{
			Files:             m.Kustomize.Files,
			AllowAnyDirectory: m.Kustomize.AllowAnyDirectory,
			EnablePlugins:     m.Kustomize.EnablePlugins,
		}
	}
	// v1alpha1 Template *bool maps onto EnableValues when it is explicitly true.
	if !bm.EnableValues && m.Template != nil && *m.Template {
		bm.EnableValues = true
	}
	return bm
}

func chartFromGeneric(ch types.Chart) v1beta1.Chart {
	bc := v1beta1.Chart{
		Name:                 ch.Name,
		Namespace:            ch.Namespace,
		ReleaseName:          ch.ReleaseName,
		ValuesFiles:          ch.ValuesFiles,
		SkipSchemaValidation: ch.SkipSchemaValidation,
		ServerSideApply:      v1beta1.ServerSideApplyMode(ch.ServerSideApply),
		SkipWait:             ch.SkipWait,
		Values:               chartValuesFromGeneric(ch.Values),
	}

	// Use the structured sources if present; otherwise infer from v1alpha1 flat fields.
	switch {
	case ch.HelmRepository != nil:
		bc.HelmRepository = &v1beta1.HelmRepositorySource{
			Name:    ch.HelmRepository.Name,
			URL:     ch.HelmRepository.URL,
			Version: ch.HelmRepository.Version,
		}
	case ch.Git != nil:
		bc.Git = &v1beta1.GitSource{URL: ch.Git.URL, Path: ch.Git.Path}
	case ch.Local != nil:
		bc.Local = &v1beta1.LocalSource{Path: ch.Local.Path}
	case ch.OCI != nil:
		bc.OCI = &v1beta1.OCISource{URL: ch.OCI.URL, Version: ch.OCI.Version}
	case ch.URL != "":
		switch {
		case strings.HasPrefix(ch.URL, "oci://"):
			bc.OCI = &v1beta1.OCISource{URL: ch.URL, Version: ch.Version}
		case ch.GitPath != "" || isGitURL(ch.URL):
			gitURL := ch.URL
			if ch.Version != "" && !strings.Contains(ch.URL, "@") {
				gitURL += "@" + ch.Version
			}
			bc.Git = &v1beta1.GitSource{URL: gitURL, Path: ch.GitPath}
		default:
			bc.HelmRepository = &v1beta1.HelmRepositorySource{
				Name:    ch.RepoName,
				URL:     ch.URL,
				Version: ch.Version,
			}
		}
	case ch.LocalPath != "":
		bc.Local = &v1beta1.LocalSource{Path: ch.LocalPath}
	}

	// v1alpha1 SchemaValidation *bool: nil/true → SkipSchemaValidation=false; explicit false → true.
	if !bc.SkipSchemaValidation && ch.SchemaValidation != nil && !*ch.SchemaValidation {
		bc.SkipSchemaValidation = true
	}

	return bc
}

func chartValuesFromGeneric(vals []types.ChartValue) []v1beta1.ChartValue {
	var out []v1beta1.ChartValue
	for _, v := range vals {
		out = append(out, v1beta1.ChartValue{
			SourcePath: v.SourcePath,
			TargetPath: v.TargetPath,
		})
	}
	return out
}

func actionsFromGeneric(a types.ComponentActions) v1beta1.ComponentActions {
	return v1beta1.ComponentActions{
		OnCreate: actionSetFromGeneric(a.OnCreate),
		OnDeploy: actionSetFromGeneric(a.OnDeploy),
		OnRemove: actionSetFromGeneric(a.OnRemove),
	}
}

func actionSetFromGeneric(s types.ComponentActionSet) v1beta1.ComponentActionSet {
	return v1beta1.ComponentActionSet{
		Defaults: v1beta1.ComponentActionDefaults{
			Silent:          s.Defaults.Silent,
			MaxTotalSeconds: s.Defaults.MaxTotalSeconds,
			Retries:         s.Defaults.Retries,
			Dir:             s.Defaults.Dir,
			Env:             s.Defaults.Env,
			Shell: v1beta1.Shell{
				Windows: s.Defaults.Shell.Windows,
				Linux:   s.Defaults.Shell.Linux,
				Darwin:  s.Defaults.Shell.Darwin,
			},
		},
		Before:    actionSliceFromGeneric(s.Before),
		OnSuccess: actionSliceFromGeneric(s.OnSuccess),
		OnFailure: actionSliceFromGeneric(s.OnFailure),
	}
}

func actionSliceFromGeneric(actions []types.ComponentAction) []v1beta1.ComponentAction {
	var out []v1beta1.ComponentAction
	for _, a := range actions {
		out = append(out, actionFromGeneric(a))
	}
	return out
}

func actionFromGeneric(a types.ComponentAction) v1beta1.ComponentAction {
	ba := v1beta1.ComponentAction{
		Silent:          a.Silent,
		MaxTotalSeconds: a.MaxTotalSeconds,
		Retries:         a.Retries,
		Dir:             a.Dir,
		Env:             a.Env,
		Cmd:             a.Cmd,
		Description:     a.Description,
		Wait:            waitFromGeneric(a.Wait),
		EnableValues:    a.EnableValues,
	}

	for _, sv := range a.SetValues {
		ba.SetValues = append(ba.SetValues, v1beta1.SetValue{
			Key:  sv.Key,
			Type: v1beta1.SetValueType(sv.Type),
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

func waitFromGeneric(w *types.ComponentActionWait) *v1beta1.ComponentActionWait {
	if w == nil {
		return nil
	}
	bw := &v1beta1.ComponentActionWait{}
	if w.Cluster != nil {
		bw.Cluster = &v1beta1.ComponentActionWaitCluster{
			Kind:      w.Cluster.Kind,
			Name:      w.Cluster.Name,
			Namespace: w.Cluster.Namespace,
			Condition: w.Cluster.Condition,
		}
	}
	if w.Network != nil {
		bw.Network = &v1beta1.ComponentActionWaitNetwork{
			Protocol: w.Network.Protocol,
			Address:  w.Network.Address,
			Code:     w.Network.Code,
		}
	}
	return bw
}

// healthCheckKind returns the wait-for kind string for a v1alpha1 health check.
// For resources with a group (e.g. APIVersion "apps/v1"), the format is <kind>.<version>.<group>.
// For core resources with no group (e.g. APIVersion "v1"), the kind is returned as-is.
func healthCheckKind(kind, apiVersion string) string {
	group, version, found := strings.Cut(apiVersion, "/")
	if !found {
		return kind
	}
	return kind + "." + version + "." + group
}

func isGitURL(url string) bool {
	if idx := strings.LastIndex(url, "@"); idx > 0 {
		url = url[:idx]
	}
	return strings.HasSuffix(url, ".git")
}

func deprecatedVarToGeneric(v v1beta1.Variable) types.Variable {
	return types.Variable{
		Name:       v.Name,
		Sensitive:  v.Sensitive,
		AutoIndent: v.AutoIndent,
		Pattern:    v.Pattern,
		Type:       types.VariableType(v.Type),
	}
}

func deprecatedVarsToGeneric(in []v1beta1.InteractiveVariable) []types.InteractiveVariable {
	var out []types.InteractiveVariable
	for _, v := range in {
		out = append(out, types.InteractiveVariable{
			Variable:    deprecatedVarToGeneric(v.Variable),
			Description: v.Description,
			Default:     v.Default,
			Prompt:      v.Prompt,
		})
	}
	return out
}

func deprecatedConstantsToGeneric(in []v1beta1.Constant) []types.Constant {
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

func deprecatedChartVarsToGeneric(in []v1beta1.ZarfChartVariable) []types.ZarfChartVariable {
	var out []types.ZarfChartVariable
	for _, v := range in {
		out = append(out, types.ZarfChartVariable{Name: v.Name, Description: v.Description, Path: v.Path})
	}
	return out
}

func deprecatedSetVarsToGeneric(in []v1beta1.Variable) []types.Variable {
	var out []types.Variable
	for _, v := range in {
		out = append(out, deprecatedVarToGeneric(v))
	}
	return out
}

func deprecatedDataInjectionsToGeneric(in []v1beta1.ZarfDataInjection) []types.ZarfDataInjection {
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
