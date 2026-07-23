// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package v1beta1 contains functions for converting between the public v1beta1 Zarf package and the internal generic representation.
package v1beta1

import (
	"maps"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/internal/api/types"
	"github.com/zarf-dev/zarf/src/internal/git"
	"github.com/zarf-dev/zarf/src/pkg/transform"
)

// ConvertToGeneric converts a v1beta1 Package to the internal generic representation.
func ConvertToGeneric(pkg v1beta1.Package) types.Package {
	// Carry the equivalent v1alpha1 AllowNamespaceOverride pointer so conversions to v1alpha1
	// produce an explicit value rather than relying on a projection.
	allowNamespaceOverride := !pkg.Metadata.PreventNamespaceOverride
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
			AllowNamespaceOverride:   &allowNamespaceOverride,
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
			OriginalAPIVersion:         pkg.Build.GetOriginalAPIVersion(),
		},
		Values: types.Values{
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
		Repositories: repositoriesToGeneric(c.Repositories),
		StateAccess:  stateAccessToGeneric(c.StateAccess),
		Target: types.ComponentTarget{
			OS:           c.Target.OS,
			Architecture: c.Selector.Architecture,
			Flavor:       c.Selector.Flavor,
		},
		Import:  importToGeneric(c.Import),
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
			Checksum:         f.Checksum,
			Destination:      f.Destination,
			Executable:       f.Executable,
			Symlinks:         f.Symlinks,
			ExtractPath:      f.ExtractPath,
			EnableTemplating: f.EnableTemplating,
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
		Name:             m.Name,
		Namespace:        m.Namespace,
		Files:            m.Files,
		SkipWait:         m.SkipWait,
		ServerSideApply:  string(m.ServerSideApply),
		EnableTemplating: m.EnableTemplating,
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
		ValuesFiles:          valuesFilesToGeneric(ch.ValuesFiles),
		SkipSchemaValidation: ch.SkipSchemaValidation,
		ServerSideApply:      string(ch.ServerSideApply),
		SkipWait:             ch.SkipWait,
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
			Ref:  gitRefToGeneric(ch.Git.Ref),
		}
	}
	if ch.Local != nil {
		gc.Local = &types.LocalSource{Path: ch.Local.Path}
	}
	if ch.OCI != nil {
		gc.OCI = &types.OCISource{
			URL: ch.OCI.URL,
			Ref: ociRefToGeneric(ch.OCI.Ref),
		}
	}

	for _, v := range ch.Values {
		gc.Values = append(gc.Values, types.ChartValue{
			SourcePath:   v.SourcePath,
			TargetPath:   v.TargetPath,
			ExcludePaths: v.ExcludePaths,
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
		Silent:           a.Silent,
		MaxTotalSeconds:  a.MaxTotalSeconds,
		Retries:          a.Retries,
		Dir:              a.Dir,
		Env:              a.Env,
		Cmd:              a.Cmd,
		Description:      a.Description,
		Wait:             waitToGeneric(a.Wait),
		EnableTemplating: a.EnableTemplating,
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
	// Component services are only inferred for packages that were init configs.
	isInit := string(pkg.Kind) == "ZarfInitConfig"
	if isInit {
		pkg.Kind = v1beta1.ZarfPackageConfig
	}

	// v1beta1 treats an empty wait.cluster.condition as a kstatus readiness check, whereas v1alpha1
	// treated it as "wait until the resource exists". Backfill "exists" on migration so existing
	// packages keep their original behavior.
	migrateFromV1alpha1 := g.Build.OriginalAPIVersion == v1alpha1.APIVersion

	for _, c := range g.Components {
		pkg.Components = append(pkg.Components, componentFromGeneric(c, isInit, migrateFromV1alpha1))
	}

	return pkg
}

func metadataFromGeneric(m types.PackageMetadata) v1beta1.PackageMetadata {
	var annotations map[string]string
	if m.Annotations != nil {
		annotations = make(map[string]string, len(m.Annotations))
		maps.Copy(annotations, m.Annotations)
	}
	meta := v1beta1.PackageMetadata{
		Name:         m.Name,
		Description:  m.Description,
		Version:      m.Version,
		Uncompressed: m.Uncompressed,
		Architecture: m.Architecture,
		Annotations:  annotations,
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
		// Don't clobber an annotation the author already set on a reserved metadata.* key; their
		// explicit value wins over the migrated field.
		if _, exists := meta.Annotations[k]; exists {
			continue
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

	// Preserve the apiVersion the package was originally read from across the conversion.
	out.SetOriginalAPIVersion(b.OriginalAPIVersion)

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

	return out
}

func componentFromGeneric(c types.Component, isInit, migrateFromV1alpha1 bool) v1beta1.Component {
	bc := v1beta1.Component{
		Name:        c.Name,
		Description: c.Description,
		Optional:    optionalFromGeneric(c.Optional, c.Required),
		ComponentSpec: v1beta1.ComponentSpec{
			Repositories: repositoriesFromGeneric(c.Repositories),
			StateAccess:  stateAccessFromGeneric(c.StateAccess),
			Target: v1beta1.ComponentTarget{
				OS: c.Target.OS,
			},
			Selector: v1beta1.ComponentSelector{
				Architecture: c.Target.Architecture,
				Flavor:       c.Target.Flavor,
			},
			Import:  importFromGeneric(c.Import),
			Service: serviceFromGeneric(c, isInit),
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
			Source:           f.Source,
			Checksum:         f.Checksum,
			Destination:      f.Destination,
			Executable:       f.Executable,
			Symlinks:         f.Symlinks,
			ExtractPath:      f.ExtractPath,
			EnableTemplating: f.EnableTemplating,
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

	if migrateFromV1alpha1 {
		backfillWaitExists(&bc.Actions)
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

// optionalFromGeneric resolves the v1beta1 Optional flag from the generic representation.
// A v1alpha1-sourced package carries an explicit Required pointer, which wins; otherwise Optional
// flows through (the v1alpha1 layer already folds an unset required into Optional).
func optionalFromGeneric(optional bool, required *bool) bool {
	if required != nil {
		return !*required
	}
	return optional
}

func serviceFromGeneric(c types.Component, isInit bool) v1beta1.Service {
	if c.Service != "" {
		return v1beta1.Service(c.Service)
	}
	// Services only exist on init packages, so don't infer them otherwise.
	if !isInit {
		return ""
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
		Name:             m.Name,
		Namespace:        m.Namespace,
		Files:            m.Files,
		SkipWait:         m.SkipWait,
		ServerSideApply:  v1beta1.ServerSideApplyMode(m.ServerSideApply),
		EnableTemplating: m.EnableTemplating,
	}
	if m.Kustomize != nil {
		bm.Kustomize = &v1beta1.KustomizeManifest{
			Files:             m.Kustomize.Files,
			AllowAnyDirectory: m.Kustomize.AllowAnyDirectory,
			EnablePlugins:     m.Kustomize.EnablePlugins,
		}
	}
	// v1alpha1 Template *bool maps onto EnableTemplating when it is explicitly true.
	if !bm.EnableTemplating && m.Template != nil && *m.Template {
		bm.EnableTemplating = true
	}
	return bm
}

func chartFromGeneric(ch types.Chart) v1beta1.Chart {
	bc := v1beta1.Chart{
		Name:                 ch.Name,
		Namespace:            ch.Namespace,
		ReleaseName:          ch.ReleaseName,
		ValuesFiles:          valuesFilesFromGeneric(ch.ValuesFiles),
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
		bc.Git = &v1beta1.GitSource{
			URL:  ch.Git.URL,
			Path: ch.Git.Path,
			Ref:  gitRefFromGeneric(ch.Git.Ref),
		}
	case ch.Local != nil:
		bc.Local = &v1beta1.LocalSource{Path: ch.Local.Path}
	case ch.OCI != nil:
		bc.OCI = &v1beta1.OCISource{
			URL: ch.OCI.URL,
			Ref: ociRefFromGeneric(ch.OCI.Ref),
		}
	case ch.URL != "":
		switch {
		case strings.HasPrefix(ch.URL, "oci://"):
			bc.OCI = &v1beta1.OCISource{URL: ch.URL, Ref: v1beta1.OCIRef{Tag: ch.Version}}
		case ch.GitPath != "" || isGitURL(ch.URL):
			gitURL := ch.URL
			refStr := ""
			if urlNoRef, r, err := transform.GitURLSplitRef(ch.URL); err == nil {
				gitURL = urlNoRef
				refStr = r
			}
			if refStr == "" && ch.Version != "" {
				refStr = ch.Version
			}
			bc.Git = &v1beta1.GitSource{URL: gitURL, Path: ch.GitPath, Ref: classifyGitRef(refStr)}
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
			SourcePath:   v.SourcePath,
			TargetPath:   v.TargetPath,
			ExcludePaths: v.ExcludePaths,
		})
	}
	return out
}

func valuesFilesToGeneric(vfs []v1beta1.ValuesFile) []types.ValuesFile {
	var out []types.ValuesFile
	for _, vf := range vfs {
		out = append(out, types.ValuesFile{Path: vf.Path, EnableTemplating: vf.EnableTemplating})
	}
	return out
}

func valuesFilesFromGeneric(vfs []types.ValuesFile) []v1beta1.ValuesFile {
	var out []v1beta1.ValuesFile
	for _, vf := range vfs {
		out = append(out, v1beta1.ValuesFile{Path: vf.Path, EnableTemplating: vf.EnableTemplating})
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
		Before: actionSliceFromGeneric(s.Before),
		// v1beta1 has no After hook; fold the v1alpha1-preserved After actions into OnSuccess.
		OnSuccess: append(actionSliceFromGeneric(s.After), actionSliceFromGeneric(s.OnSuccess)...),
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
		Silent:           a.Silent,
		MaxTotalSeconds:  a.MaxTotalSeconds,
		Retries:          a.Retries,
		Dir:              a.Dir,
		Env:              a.Env,
		Cmd:              a.Cmd,
		Description:      a.Description,
		Wait:             waitFromGeneric(a.Wait),
		EnableTemplating: a.EnableTemplating,
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

// backfillWaitExists sets any action wait.cluster.condition left empty to "exists", preserving
// v1alpha1 wait semantics. Health-check-derived waits are appended after this runs and keep an
// empty condition so they use v1beta1 kstatus readiness checks.
func backfillWaitExists(actions *v1beta1.ComponentActions) {
	for _, set := range []*v1beta1.ComponentActionSet{&actions.OnCreate, &actions.OnDeploy, &actions.OnRemove} {
		for _, slice := range [][]v1beta1.ComponentAction{set.Before, set.OnSuccess, set.OnFailure} {
			for k := range slice {
				if w := slice[k].Wait; w != nil && w.Cluster != nil && w.Cluster.Condition == "" {
					w.Cluster.Condition = "exists"
				}
			}
		}
	}
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
	gitURLNoRef, _, err := transform.GitURLSplitRef(url)
	if err != nil {
		return false
	}
	return strings.HasSuffix(gitURLNoRef, ".git")
}

func repositoriesToGeneric(in []v1beta1.Repository) []types.Repository {
	var out []types.Repository
	for _, r := range in {
		gr := types.Repository{URL: r.URL}
		if r.Ref != nil {
			gr.Ref = &types.GitRef{
				Tag:    r.Ref.Tag,
				Branch: r.Ref.Branch,
				Commit: r.Ref.Commit,
			}
		}
		out = append(out, gr)
	}
	return out
}

func repositoriesFromGeneric(in []types.Repository) []v1beta1.Repository {
	var out []v1beta1.Repository
	for _, r := range in {
		br := v1beta1.Repository{URL: r.URL}
		if r.Ref != nil {
			br.Ref = &v1beta1.GitRef{
				Tag:    r.Ref.Tag,
				Branch: r.Ref.Branch,
				Commit: r.Ref.Commit,
			}
		} else {
			// v1alpha1 repos embed the ref in the URL; split it for v1beta1.
			if urlNoRef, refStr, err := transform.GitURLSplitRef(r.URL); err == nil && refStr != "" {
				br.URL = urlNoRef
				ref := classifyGitRef(refStr)
				br.Ref = &ref
			}
		}
		out = append(out, br)
	}
	return out
}

func stateAccessToGeneric(in []v1beta1.StateAccessKey) []string {
	var out []string
	for _, s := range in {
		out = append(out, string(s))
	}
	return out
}

func stateAccessFromGeneric(in []string) []v1beta1.StateAccessKey {
	var out []v1beta1.StateAccessKey
	for _, s := range in {
		out = append(out, v1beta1.StateAccessKey(s))
	}
	return out
}

func gitRefToGeneric(ref v1beta1.GitRef) *types.GitRef {
	if ref == (v1beta1.GitRef{}) {
		return nil
	}
	return &types.GitRef{
		Tag:    ref.Tag,
		Branch: ref.Branch,
		Commit: ref.Commit,
	}
}

func gitRefFromGeneric(ref *types.GitRef) v1beta1.GitRef {
	if ref == nil {
		return v1beta1.GitRef{}
	}
	return v1beta1.GitRef{
		Tag:    ref.Tag,
		Branch: ref.Branch,
		Commit: ref.Commit,
	}
}

func ociRefToGeneric(ref v1beta1.OCIRef) *types.OCIRef {
	if ref == (v1beta1.OCIRef{}) {
		return nil
	}
	return &types.OCIRef{
		Tag:    ref.Tag,
		Digest: ref.Digest,
	}
}

func ociRefFromGeneric(ref *types.OCIRef) v1beta1.OCIRef {
	if ref == nil {
		return v1beta1.OCIRef{}
	}
	return v1beta1.OCIRef{
		Tag:    ref.Tag,
		Digest: ref.Digest,
	}
}

func classifyGitRef(ref string) v1beta1.GitRef {
	if ref == "" {
		return v1beta1.GitRef{}
	}
	if plumbing.IsHash(ref) {
		return v1beta1.GitRef{Commit: ref}
	}
	parsed := string(git.ParseRef(ref))
	if branch, ok := strings.CutPrefix(parsed, "refs/heads/"); ok {
		return v1beta1.GitRef{Branch: branch}
	}
	return v1beta1.GitRef{Tag: strings.TrimPrefix(parsed, "refs/tags/")}
}
