// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package v1beta1 contains functions for converting between the public v1beta1 Zarf package and the internal generic representation.
package v1beta1

import (
	"maps"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/internal/git"
	"github.com/zarf-dev/zarf/src/pkg/transform"
)

// PackageFromV1beta1 converts a v1beta1 Package to the internal generic representation.
func PackageFromV1beta1(pkg v1beta1.Package) api.Package {
	g := api.Package{
		APIVersion: pkg.APIVersion,
		Kind:       api.PackageKind(pkg.Kind),
		Metadata: api.PackageMetadata{
			Name:                     pkg.Metadata.Name,
			Description:              pkg.Metadata.Description,
			Version:                  pkg.Metadata.Version,
			Uncompressed:             pkg.Metadata.Uncompressed,
			Architecture:             pkg.Metadata.Architecture,
			Annotations:              pkg.Metadata.Annotations,
			PreventNamespaceOverride: pkg.Metadata.PreventNamespaceOverride,
		},
		Build: api.BuildData{
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
		},
		Values: api.Values{
			Files:  pkg.Values.Files,
			Schema: pkg.Values.Schema,
		},
		Documentation: pkg.Documentation,
	}

	for _, vr := range pkg.Build.VersionRequirements {
		g.Build.VersionRequirements = append(g.Build.VersionRequirements, api.VersionRequirement{
			Version: vr.Version,
			Reason:  vr.Reason,
		})
	}

	for _, c := range pkg.Components {
		g.Components = append(g.Components, componentToGeneric(c))
	}

	return g
}

func componentToGeneric(c v1beta1.Component) api.Component {
	gc := api.Component{
		Name:         c.Name,
		Description:  c.Description,
		Optional:     c.Optional,
		Service:      string(c.Service),
		Repositories: repositoriesToGeneric(c.Repositories),
		StateAccess:  stateAccessToGeneric(c.StateAccess),
		Target: api.ComponentTarget{
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
		gc.Files = append(gc.Files, api.File{
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
		gc.Images = append(gc.Images, api.Image{
			Name:   img.Name,
			Source: img.Source,
		})
	}

	for _, ia := range c.ImageArchives {
		gc.ImageArchives = append(gc.ImageArchives, api.ImageArchive{
			Path:   ia.Path,
			Images: ia.Images,
		})
	}

	return gc
}

func importToGeneric(imp v1beta1.ComponentImport) api.ComponentImport {
	out := api.ComponentImport{}
	for _, l := range imp.Local {
		out.Local = append(out.Local, api.ComponentImportLocal{Path: l.Path})
	}
	for _, r := range imp.Remote {
		out.Remote = append(out.Remote, api.ComponentImportRemote{URL: r.URL})
	}
	return out
}

func manifestToGeneric(m v1beta1.Manifest) api.Manifest {
	gm := api.Manifest{
		Name:             m.Name,
		Namespace:        m.Namespace,
		Files:            m.Files,
		SkipWait:         m.SkipWait,
		ServerSideApply:  string(m.ServerSideApply),
		EnableTemplating: m.EnableTemplating,
	}
	if m.Kustomize != nil {
		gm.Kustomize = &api.KustomizeManifest{
			Files:             m.Kustomize.Files,
			AllowAnyDirectory: m.Kustomize.AllowAnyDirectory,
			EnablePlugins:     m.Kustomize.EnablePlugins,
		}
	}
	return gm
}

func chartToGeneric(ch v1beta1.Chart) api.Chart {
	gc := api.Chart{
		Name:                 ch.Name,
		Namespace:            ch.Namespace,
		ReleaseName:          ch.ReleaseName,
		ValuesFiles:          valuesFilesToGeneric(ch.ValuesFiles),
		SkipSchemaValidation: ch.SkipSchemaValidation,
		ServerSideApply:      string(ch.ServerSideApply),
		SkipWait:             ch.SkipWait,
	}

	if ch.HelmRepository != nil {
		gc.HelmRepository = &api.HelmRepositorySource{
			Name:    ch.HelmRepository.Name,
			URL:     ch.HelmRepository.URL,
			Version: ch.HelmRepository.Version,
		}
	}
	if ch.Git != nil {
		gc.Git = &api.GitSource{
			URL:  ch.Git.URL,
			Path: ch.Git.Path,
			Ref:  gitRefToGeneric(ch.Git.Ref),
		}
	}
	if ch.Local != nil {
		gc.Local = &api.LocalSource{Path: ch.Local.Path}
	}
	if ch.OCI != nil {
		gc.OCI = &api.OCISource{
			URL: ch.OCI.URL,
			Ref: ociRefToGeneric(ch.OCI.Ref),
		}
	}

	for _, v := range ch.Values {
		gc.Values = append(gc.Values, api.ChartValue{
			SourcePath:   v.SourcePath,
			TargetPath:   v.TargetPath,
			ExcludePaths: v.ExcludePaths,
		})
	}

	return gc
}

func actionsToGeneric(a v1beta1.ComponentActions) api.ComponentActions {
	return api.ComponentActions{
		OnCreate: actionSetToGeneric(a.OnCreate),
		OnDeploy: actionSetToGeneric(a.OnDeploy),
		OnRemove: actionSetToGeneric(a.OnRemove),
	}
}

func actionSetToGeneric(s v1beta1.ComponentActionSet) api.ActionSet {
	return api.ActionSet{
		Defaults:  actionDefaultsToGeneric(s.Defaults),
		Before:    actionSliceToGeneric(s.Before),
		OnSuccess: actionSliceToGeneric(s.OnSuccess),
		OnFailure: actionSliceToGeneric(s.OnFailure),
	}
}

func actionDefaultsToGeneric(d *v1beta1.ComponentActionDefaults) api.ActionDefaults {
	if d == nil {
		return api.ActionDefaults{}
	}
	return api.ActionDefaults{
		Silent:          d.Silent,
		MaxTotalSeconds: int(d.MaxTotalSeconds),
		Retries:         int(d.Retries),
		Dir:             d.Dir,
		Env:             d.Env,
		Shell: api.Shell{
			Windows: d.Shell.Windows,
			Linux:   d.Shell.Linux,
			Darwin:  d.Shell.Darwin,
		},
	}
}

func actionSliceToGeneric(actions []v1beta1.ComponentAction) []api.Action {
	var out []api.Action
	for _, a := range actions {
		out = append(out, actionToGeneric(a))
	}
	return out
}

func actionToGeneric(a v1beta1.ComponentAction) api.Action {
	ga := api.Action{
		Silent:           a.Silent,
		MaxTotalSeconds:  int32PointerToIntPointer(a.MaxTotalSeconds),
		Retries:          int32PointerToIntPointer(a.Retries),
		Dir:              a.Dir,
		Env:              a.Env,
		Cmd:              a.Cmd,
		Description:      a.Description,
		Wait:             waitToGeneric(a.Wait),
		EnableTemplating: a.EnableTemplating,
	}

	for _, sv := range a.SetValues {
		ga.SetValues = append(ga.SetValues, api.SetValue{
			Key:  sv.Key,
			Type: api.SetValueType(sv.Type),
		})
	}

	if a.Shell != nil {
		ga.Shell = &api.Shell{
			Windows: a.Shell.Windows,
			Linux:   a.Shell.Linux,
			Darwin:  a.Shell.Darwin,
		}
	}

	return ga
}

func waitToGeneric(w *v1beta1.ComponentActionWait) *api.ActionWait {
	if w == nil {
		return nil
	}
	gw := &api.ActionWait{}
	if w.Cluster != nil {
		gw.Cluster = &api.ActionWaitCluster{
			Kind:      w.Cluster.Kind,
			Name:      w.Cluster.Name,
			Namespace: w.Cluster.Namespace,
			Condition: api.WaitCondition{Expression: w.Cluster.Condition, Default: api.WaitForReadiness},
		}
	}
	if w.Network != nil {
		gw.Network = &api.ActionWaitNetwork{
			Protocol: w.Network.Protocol,
			Address:  w.Network.Address,
			Code:     int(w.Network.Code),
		}
	}
	return gw
}

// PackageToV1beta1 converts the internal generic representation to a v1beta1 Package.
func PackageToV1beta1(g api.Package) v1beta1.Package {
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
	isInit := g.Kind == api.ZarfInitConfig
	if isInit {
		pkg.Kind = v1beta1.ZarfPackageConfig
	}

	// v1beta1 treats an empty wait.cluster.condition as a kstatus readiness check, whereas v1alpha1
	// treated it as "wait until the resource exists". Backfill "exists" on migration so existing
	// packages keep their original behavior.
	migrateFromV1alpha1 := g.APIVersion == "" || g.APIVersion == v1alpha1.APIVersion

	for _, c := range g.Components {
		pkg.Components = append(pkg.Components, componentFromGeneric(c, isInit, migrateFromV1alpha1))
	}

	return pkg
}

func metadataFromGeneric(m api.PackageMetadata) v1beta1.PackageMetadata {
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

	meta.PreventNamespaceOverride = m.PreventNamespaceOverride

	return meta
}

func buildFromGeneric(b api.BuildData, _ api.PackageMetadata) v1beta1.BuildData {
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

	out.AggregateChecksum = b.AggregateChecksum

	for _, vr := range b.VersionRequirements {
		out.VersionRequirements = append(out.VersionRequirements, v1beta1.VersionRequirement{
			Version: vr.Version,
			Reason:  vr.Reason,
		})
	}

	return out
}

func componentFromGeneric(c api.Component, isInit, migrateFromV1alpha1 bool) v1beta1.Component {
	bc := v1beta1.Component{
		Name:        c.Name,
		Description: c.Description,
		Optional:    c.Optional,
		Selector: v1beta1.ComponentSelector{
			Architecture: c.Target.Architecture,
			Flavor:       c.Target.Flavor,
		},
		ComponentSpec: v1beta1.ComponentSpec{
			Repositories: repositoriesFromGeneric(c.Repositories),
			StateAccess:  stateAccessFromGeneric(c.StateAccess),
			Target: v1beta1.ComponentTarget{
				OS: c.Target.OS,
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

func serviceFromGeneric(c api.Component, isInit bool) v1beta1.Service {
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

func importFromGeneric(imp api.ComponentImport) v1beta1.ComponentImport {
	out := v1beta1.ComponentImport{}
	for _, l := range imp.Local {
		out.Local = append(out.Local, v1beta1.ComponentImportLocal{Path: l.Path})
	}
	for _, r := range imp.Remote {
		out.Remote = append(out.Remote, v1beta1.ComponentImportRemote{URL: r.URL})
	}
	return out
}

func manifestFromGeneric(m api.Manifest) v1beta1.Manifest {
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
	return bm
}

func chartFromGeneric(ch api.Chart) v1beta1.Chart {
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

	// The operational model has a single structured chart source.
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
	}

	return bc
}

func chartValuesFromGeneric(vals []api.ChartValue) []v1beta1.ChartValue {
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

func valuesFilesToGeneric(vfs []v1beta1.ValuesFile) []api.ValuesFile {
	var out []api.ValuesFile
	for _, vf := range vfs {
		out = append(out, api.ValuesFile{Path: vf.Path, EnableTemplating: vf.EnableTemplating})
	}
	return out
}

func valuesFilesFromGeneric(vfs []api.ValuesFile) []v1beta1.ValuesFile {
	var out []v1beta1.ValuesFile
	for _, vf := range vfs {
		out = append(out, v1beta1.ValuesFile{Path: vf.Path, EnableTemplating: vf.EnableTemplating})
	}
	return out
}

func actionsFromGeneric(a api.ComponentActions) v1beta1.ComponentActions {
	return v1beta1.ComponentActions{
		OnCreate: actionSetFromGeneric(a.OnCreate),
		OnDeploy: actionSetFromGeneric(a.OnDeploy),
		OnRemove: actionSetFromGeneric(a.OnRemove),
	}
}

func actionSetFromGeneric(s api.ActionSet) v1beta1.ComponentActionSet {
	return v1beta1.ComponentActionSet{
		Defaults: actionDefaultsFromGeneric(s.Defaults),
		Before:   actionSliceFromGeneric(s.Before),
		// v1beta1 has no After hook; fold the v1alpha1-preserved After actions into OnSuccess.
		OnSuccess: append(actionSliceFromGeneric(s.After), actionSliceFromGeneric(s.OnSuccess)...),
		OnFailure: actionSliceFromGeneric(s.OnFailure),
	}
}

func actionDefaultsFromGeneric(d api.ActionDefaults) *v1beta1.ComponentActionDefaults {
	defaults := &v1beta1.ComponentActionDefaults{
		Silent:          d.Silent,
		MaxTotalSeconds: int32(d.MaxTotalSeconds),
		Retries:         int32(d.Retries),
		Dir:             d.Dir,
		Env:             d.Env,
		Shell: v1beta1.Shell{
			Windows: d.Shell.Windows,
			Linux:   d.Shell.Linux,
			Darwin:  d.Shell.Darwin,
		},
	}
	return defaults
}

func actionSliceFromGeneric(actions []api.Action) []v1beta1.ComponentAction {
	var out []v1beta1.ComponentAction
	for _, a := range actions {
		out = append(out, actionFromGeneric(a))
	}
	return out
}

func actionFromGeneric(a api.Action) v1beta1.ComponentAction {
	ba := v1beta1.ComponentAction{
		Silent:           a.Silent,
		MaxTotalSeconds:  intPointerToInt32Pointer(a.MaxTotalSeconds),
		Retries:          intPointerToInt32Pointer(a.Retries),
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

func waitFromGeneric(w *api.ActionWait) *v1beta1.ComponentActionWait {
	if w == nil {
		return nil
	}
	bw := &v1beta1.ComponentActionWait{}
	if w.Cluster != nil {
		bw.Cluster = &v1beta1.ComponentActionWaitCluster{
			Kind:      w.Cluster.Kind,
			Name:      w.Cluster.Name,
			Namespace: w.Cluster.Namespace,
			Condition: w.Cluster.Condition.Expression,
		}
	}
	if w.Network != nil {
		bw.Network = &v1beta1.ComponentActionWaitNetwork{
			Protocol: w.Network.Protocol,
			Address:  w.Network.Address,
			Code:     int32(w.Network.Code),
		}
	}
	return bw
}

func int32PointerToIntPointer(in *int32) *int {
	if in == nil {
		return nil
	}
	out := int(*in)
	return &out
}

func intPointerToInt32Pointer(in *int) *int32 {
	if in == nil {
		return nil
	}
	out := int32(*in)
	return &out
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

func repositoriesToGeneric(in []v1beta1.Repository) []api.Repository {
	var out []api.Repository
	for _, r := range in {
		gr := api.Repository{URL: r.URL}
		if r.Ref != nil {
			gr.Ref = &api.GitRef{
				Tag:    r.Ref.Tag,
				Branch: r.Ref.Branch,
				Commit: r.Ref.Commit,
			}
		}
		out = append(out, gr)
	}
	return out
}

func repositoriesFromGeneric(in []api.Repository) []v1beta1.Repository {
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

func gitRefToGeneric(ref v1beta1.GitRef) *api.GitRef {
	if ref == (v1beta1.GitRef{}) {
		return nil
	}
	return &api.GitRef{
		Tag:    ref.Tag,
		Branch: ref.Branch,
		Commit: ref.Commit,
	}
}

func gitRefFromGeneric(ref *api.GitRef) v1beta1.GitRef {
	if ref == nil {
		return v1beta1.GitRef{}
	}
	return v1beta1.GitRef{
		Tag:    ref.Tag,
		Branch: ref.Branch,
		Commit: ref.Commit,
	}
}

func ociRefToGeneric(ref v1beta1.OCIRef) *api.OCIRef {
	if ref == (v1beta1.OCIRef{}) {
		return nil
	}
	return &api.OCIRef{
		Tag:    ref.Tag,
		Digest: ref.Digest,
	}
}

func ociRefFromGeneric(ref *api.OCIRef) v1beta1.OCIRef {
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
