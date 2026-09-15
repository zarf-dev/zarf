// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package v1alpha1 contains functions for converting between the public v1alpha1 Zarf package and the internal generic representation.
package v1alpha1

import (
	"maps"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/git"
	"github.com/zarf-dev/zarf/src/pkg/transform"
)

// PackageFromV1alpha1 converts a v1alpha1 ZarfPackage to the internal generic representation.
func PackageFromV1alpha1(pkg v1alpha1.ZarfPackage) api.Package {
	g := api.Package{
		APIVersion: pkg.APIVersion,
		Kind:       api.PackageKind(pkg.Kind),
		Metadata: api.PackageMetadata{
			Name:                     pkg.Metadata.Name,
			Description:              pkg.Metadata.Description,
			Version:                  pkg.Metadata.Version,
			Uncompressed:             pkg.Metadata.Uncompressed,
			Architecture:             pkg.Metadata.Architecture,
			Annotations:              metadataAnnotations(pkg.Metadata),
			PreventNamespaceOverride: !pkg.AllowsNamespaceOverride(),
			YOLO:                     pkg.Metadata.YOLO,
		},
		Build: api.BuildData{
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
			AggregateChecksum:          pkg.Metadata.AggregateChecksum,
		},
		Values: api.Values{
			Files:  pkg.Values.Files,
			Schema: pkg.Values.Schema,
		},
		Documentation: pkg.Documentation,
		Variables:     interactiveVarsToGeneric(pkg.Variables),
		Constants:     constantsToGeneric(pkg.Constants),
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

func componentToGeneric(c v1alpha1.ZarfComponent) api.Component {
	gc := api.Component{
		Name:              c.Name,
		Description:       c.Description,
		Default:           c.Default,
		Optional:          !c.IsRequired(),
		Group:             c.DeprecatedGroup,
		DataInjections:    dataInjectionsToGeneric(c.DataInjections),
		HealthChecks:      healthChecksToGeneric(c.HealthChecks),
		DeprecatedScripts: scriptsToGeneric(c.DeprecatedScripts),
		Repositories:      reposToGeneric(c.Repos),
		StateAccess:       stateAccessToGeneric(c.StateAccess),
		Target: api.ComponentTarget{
			OS:           c.Only.LocalOS,
			Architecture: c.Only.Cluster.Architecture,
			Flavor:       c.Only.Flavor,
		},
		Distros: c.Only.Cluster.Distros,
		Import:  api.ComponentImport{Name: c.Import.Name},
		Actions: actionsToGeneric(c.Actions),
	}
	if c.Import.Path != "" {
		gc.Import.Local = []api.ComponentImportLocal{{Path: c.Import.Path}}
	}
	if c.Import.URL != "" {
		gc.Import.Remote = []api.ComponentImportRemote{{URL: c.Import.URL}}
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
			Checksum:         f.Shasum,
			Destination:      f.Target,
			Executable:       f.Executable,
			Symlinks:         f.Symlinks,
			ExtractPath:      f.ExtractPath,
			EnableTemplating: derefBool(f.Template),
		})
	}

	for _, img := range c.Images {
		gc.Images = append(gc.Images, api.Image{Name: img})
	}

	for _, ia := range c.ImageArchives {
		gc.ImageArchives = append(gc.ImageArchives, api.ImageArchive{
			Path:   ia.Path,
			Images: ia.Images,
		})
	}

	return gc
}

func manifestToGeneric(m v1alpha1.ZarfManifest) api.Manifest {
	gm := api.Manifest{
		Name:             m.Name,
		Namespace:        m.Namespace,
		Files:            m.Files,
		SkipWait:         m.NoWait,
		ServerSideApply:  m.ServerSideApply,
		EnableTemplating: derefBool(m.Template),
	}
	if len(m.Kustomizations) > 0 || m.KustomizeAllowAnyDirectory || m.EnableKustomizePlugins {
		gm.Kustomize = &api.KustomizeManifest{
			Files:             m.Kustomizations,
			AllowAnyDirectory: m.KustomizeAllowAnyDirectory,
			EnablePlugins:     m.EnableKustomizePlugins,
		}
	}
	return gm
}

func chartToGeneric(ch v1alpha1.ZarfChart) api.Chart {
	gc := api.Chart{
		Name:                 ch.Name,
		Version:              ch.Version,
		Namespace:            ch.Namespace,
		ReleaseName:          ch.ReleaseName,
		ValuesFiles:          valuesFilesToGeneric(ch.ValuesFiles, ch.TemplatedValuesFiles),
		SkipSchemaValidation: ch.SchemaValidation != nil && !*ch.SchemaValidation,
		ServerSideApply:      ch.ServerSideApply,
		SkipWait:             ch.NoWait,
		Variables:            chartVarsToGeneric(ch.Variables),
		Values:               chartValuesToGeneric(ch.Values),
	}
	chartSourceToGeneric(&gc, ch)
	return gc
}

// chartSourceToGeneric projects v1alpha1's flat source fields into the single
// structured source used by the operational model.
func chartSourceToGeneric(chart *api.Chart, source v1alpha1.ZarfChart) {
	switch {
	case isGitURL(source.URL):
		gitURL, ref := source.URL, source.Version
		if url, parsedRef, err := transform.GitURLSplitRef(source.URL); err == nil {
			gitURL = url
			if parsedRef != "" {
				ref = parsedRef
			}
		}
		chart.Git = &api.GitSource{
			URL:  gitURL,
			Path: source.GitPath,
			Ref:  classifyGitRef(ref),
		}
	case strings.HasPrefix(source.URL, "oci://"):
		ociURL, ref := source.URL, &api.OCIRef{Tag: source.Version}
		if url, digest, found := strings.Cut(source.URL, "@sha256:"); found {
			ociURL, ref = url, &api.OCIRef{Digest: "sha256:" + digest}
		}
		chart.OCI = &api.OCISource{URL: ociURL, Ref: ref}
	case source.URL != "":
		chart.HelmRepository = &api.HelmRepositorySource{
			Name: source.RepoName, URL: source.URL, Version: source.Version,
		}
	case source.LocalPath != "":
		chart.Local = &api.LocalSource{Path: source.LocalPath}
	}
}

func isGitURL(url string) bool {
	gitURL, _, err := transform.GitURLSplitRef(url)
	return err == nil && strings.HasSuffix(gitURL, ".git")
}

func classifyGitRef(ref string) *api.GitRef {
	if ref == "" {
		return nil
	}
	if plumbing.IsHash(ref) {
		return &api.GitRef{Commit: ref}
	}
	parsed := string(git.ParseRef(ref))
	if branch, ok := strings.CutPrefix(parsed, "refs/heads/"); ok {
		return &api.GitRef{Branch: branch}
	}
	return &api.GitRef{Tag: strings.TrimPrefix(parsed, "refs/tags/")}
}

// valuesFilesToGeneric folds the v1alpha1 plain and templated values file lists into the generic
// object form, marking the templated entries with EnableTemplating.
func valuesFilesToGeneric(plain, templated []string) []api.ValuesFile {
	var out []api.ValuesFile
	for _, p := range plain {
		out = append(out, api.ValuesFile{Path: p})
	}
	for _, p := range templated {
		out = append(out, api.ValuesFile{Path: p, EnableTemplating: true})
	}
	return out
}

// valuesFilesFromGeneric splits the generic object form back into the v1alpha1 plain and templated
// lists based on EnableTemplating.
func valuesFilesFromGeneric(vfs []api.ValuesFile) (plain, templated []string) {
	for _, vf := range vfs {
		if vf.EnableTemplating {
			templated = append(templated, vf.Path)
		} else {
			plain = append(plain, vf.Path)
		}
	}
	return plain, templated
}

func chartValuesToGeneric(vals []v1alpha1.ZarfChartValue) []api.ChartValue {
	var out []api.ChartValue
	for _, v := range vals {
		out = append(out, api.ChartValue{
			SourcePath:   v.SourcePath,
			TargetPath:   v.TargetPath,
			ExcludePaths: v.ExcludePaths,
		})
	}
	return out
}

func actionsToGeneric(a v1alpha1.ZarfComponentActions) api.ComponentActions {
	return api.ComponentActions{
		OnCreate: actionSetToGeneric(a.OnCreate),
		OnDeploy: actionSetToGeneric(a.OnDeploy),
		OnRemove: actionSetToGeneric(a.OnRemove),
	}
}

func actionSetToGeneric(s v1alpha1.ZarfComponentActionSet) api.ActionSet {
	defaults := api.ActionDefaults{
		Silent:          s.Defaults.Mute,
		MaxTotalSeconds: s.Defaults.MaxTotalSeconds,
		Retries:         s.Defaults.MaxRetries,
		Dir:             s.Defaults.Dir,
		Env:             s.Defaults.Env,
		Shell: api.Shell{
			Windows: s.Defaults.Shell.Windows,
			Linux:   s.Defaults.Shell.Linux,
			Darwin:  s.Defaults.Shell.Darwin,
		},
	}

	return api.ActionSet{
		Defaults:  defaults,
		Before:    actionSliceToGeneric(s.Before),
		After:     actionSliceToGeneric(s.After),
		OnSuccess: actionSliceToGeneric(s.OnSuccess),
		OnFailure: actionSliceToGeneric(s.OnFailure),
	}
}

func actionSliceToGeneric(actions []v1alpha1.ZarfComponentAction) []api.Action {
	var out []api.Action
	for _, a := range actions {
		out = append(out, actionToGeneric(a))
	}
	return out
}

func actionToGeneric(a v1alpha1.ZarfComponentAction) api.Action {
	ga := api.Action{
		Silent:                a.Mute,
		Dir:                   a.Dir,
		Env:                   a.Env,
		Cmd:                   a.Cmd,
		Description:           a.Description,
		Wait:                  waitToGeneric(a.Wait),
		EnableTemplating:      derefBool(a.Template),
		SetVariables:          actionVariablesToGeneric(a.SetVariables),
		DeprecatedSetVariable: a.DeprecatedSetVariable,
	}

	if a.MaxTotalSeconds != nil {
		v := *a.MaxTotalSeconds
		ga.MaxTotalSeconds = &v
	}
	if a.MaxRetries != nil {
		v := *a.MaxRetries
		ga.Retries = &v
	}

	for _, sv := range a.SetValues {
		ga.SetValues = append(ga.SetValues, api.SetValue{
			Key:   sv.Key,
			Value: sv.Value,
			Type:  api.SetValueType(sv.Type),
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

func waitToGeneric(w *v1alpha1.ZarfComponentActionWait) *api.ActionWait {
	if w == nil {
		return nil
	}
	gw := &api.ActionWait{}
	if w.Cluster != nil {
		gw.Cluster = &api.ActionWaitCluster{
			Kind:      w.Cluster.Kind,
			Name:      w.Cluster.Name,
			Namespace: w.Cluster.Namespace,
			Condition: api.WaitCondition{Expression: w.Cluster.Condition, Default: api.WaitForExistence},
		}
	}
	if w.Network != nil {
		gw.Network = &api.ActionWaitNetwork{
			Protocol: w.Network.Protocol,
			Address:  w.Network.Address,
			Code:     w.Network.Code,
		}
	}
	return gw
}

// PackageToV1alpha1 converts the internal generic representation to a v1alpha1 ZarfPackage.
func PackageToV1alpha1(g api.Package) v1alpha1.ZarfPackage {
	// An absent v1alpha1 apiVersion predates the required field and must remain absent when the
	// operational package is written back out.
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

	for _, c := range g.Components {
		pkg.Components = append(pkg.Components, componentFromGeneric(c))
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

func metadataFromGeneric(m api.PackageMetadata, b api.BuildData) v1alpha1.ZarfMetadata {
	meta := v1alpha1.ZarfMetadata{
		Name:         m.Name,
		Description:  m.Description,
		Version:      m.Version,
		Uncompressed: m.Uncompressed,
		Architecture: m.Architecture,
		YOLO:         m.YOLO,
	}
	meta.AllowNamespaceOverride = boolPointer(!m.PreventNamespaceOverride)

	meta.AggregateChecksum = b.AggregateChecksum

	// v1alpha1-only metadata is stored as annotations in the operational model.
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

func metadataAnnotations(metadata v1alpha1.ZarfMetadata) map[string]string {
	annotations := maps.Clone(metadata.Annotations)
	for key, value := range map[string]string{
		"metadata.url":           metadata.URL,
		"metadata.image":         metadata.Image,
		"metadata.authors":       metadata.Authors,
		"metadata.documentation": metadata.Documentation,
		"metadata.source":        metadata.Source,
		"metadata.vendor":        metadata.Vendor,
	} {
		if value == "" {
			continue
		}
		if annotations == nil {
			annotations = make(map[string]string)
		}
		if _, exists := annotations[key]; !exists {
			annotations[key] = value
		}
	}
	return annotations
}

func buildFromGeneric(b api.BuildData) v1alpha1.ZarfBuildData {
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

func scriptsToGeneric(s v1alpha1.DeprecatedZarfComponentScripts) api.DeprecatedComponentScripts {
	return api.DeprecatedComponentScripts{
		ShowOutput:     s.ShowOutput,
		TimeoutSeconds: s.TimeoutSeconds,
		Retry:          s.Retry,
		Prepare:        s.Prepare,
		Before:         s.Before,
		After:          s.After,
	}
}

func scriptsFromGeneric(s api.DeprecatedComponentScripts) v1alpha1.DeprecatedZarfComponentScripts {
	return v1alpha1.DeprecatedZarfComponentScripts{
		ShowOutput:     s.ShowOutput,
		TimeoutSeconds: s.TimeoutSeconds,
		Retry:          s.Retry,
		Prepare:        s.Prepare,
		Before:         s.Before,
		After:          s.After,
	}
}

func componentFromGeneric(c api.Component) v1alpha1.ZarfComponent {
	ac := v1alpha1.ZarfComponent{
		Name:              c.Name,
		Description:       c.Description,
		Default:           c.Default,
		Required:          requiredFromGeneric(c.Optional),
		DeprecatedGroup:   c.Group,
		DataInjections:    dataInjectionsFromGeneric(c.DataInjections),
		HealthChecks:      healthChecksFromGeneric(c.HealthChecks),
		DeprecatedScripts: scriptsFromGeneric(c.DeprecatedScripts),
		Repos:             reposFromGeneric(c.Repositories),
		StateAccess:       stateAccessFromGeneric(c.StateAccess),
		Only: v1alpha1.ZarfComponentOnlyTarget{
			LocalOS: c.Target.OS,
			Cluster: v1alpha1.ZarfComponentOnlyCluster{
				Architecture: c.Target.Architecture,
				Distros:      c.Distros,
			},
			Flavor: c.Target.Flavor,
		},
		Import:  v1alpha1.ZarfComponentImport{Name: c.Import.Name},
		Actions: actionsFromGeneric(c.Actions),
	}

	if len(c.Import.Local) > 0 {
		ac.Import.Path = c.Import.Local[0].Path
	}
	if len(c.Import.Remote) > 0 {
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
			Template:    boolPointer(f.EnableTemplating),
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

// requiredFromGeneric maps the operational optional flag back to v1alpha1's inverse field.
func requiredFromGeneric(optional bool) *bool {
	return boolPointer(!optional)
}

func boolPointer(value bool) *bool {
	return &value
}

func manifestFromGeneric(m api.Manifest) v1alpha1.ZarfManifest {
	am := v1alpha1.ZarfManifest{
		Name:            m.Name,
		Namespace:       m.Namespace,
		Files:           m.Files,
		ServerSideApply: m.ServerSideApply,
		NoWait:          m.SkipWait,
		Template:        boolPointer(m.EnableTemplating),
	}
	if m.Kustomize != nil {
		am.Kustomizations = m.Kustomize.Files
		am.KustomizeAllowAnyDirectory = m.Kustomize.AllowAnyDirectory
		am.EnableKustomizePlugins = m.Kustomize.EnablePlugins
	}
	return am
}

func chartFromGeneric(ch api.Chart) v1alpha1.ZarfChart {
	ac := v1alpha1.ZarfChart{
		Name:             ch.Name,
		Version:          ch.Version,
		Namespace:        ch.Namespace,
		ReleaseName:      ch.ReleaseName,
		SchemaValidation: boolPointer(!ch.SkipSchemaValidation),
		ServerSideApply:  ch.ServerSideApply,
		NoWait:           ch.SkipWait,
		Variables:        chartVarsFromGeneric(ch.Variables),
	}
	ac.ValuesFiles, ac.TemplatedValuesFiles = valuesFilesFromGeneric(ch.ValuesFiles)

	switch {
	case ch.HelmRepository != nil && ch.HelmRepository.URL != "":
		ac.URL = ch.HelmRepository.URL
		ac.RepoName = ch.HelmRepository.Name
		ac.Version = ch.HelmRepository.Version
	case ch.OCI != nil && ch.OCI.URL != "":
		ac.URL = ch.OCI.URL
		if ch.OCI.Ref != nil {
			if ch.OCI.Ref.Tag != "" {
				ac.Version = ch.OCI.Ref.Tag
			} else if ch.OCI.Ref.Digest != "" {
				ac.URL = strings.TrimSuffix(ch.OCI.URL, "/") + "@" + ch.OCI.Ref.Digest
			}
		}
	case ch.Git != nil && ch.Git.URL != "":
		gitURL := ch.Git.URL
		if urlNoRef, _, err := transform.GitURLSplitRef(ch.Git.URL); err == nil {
			gitURL = urlNoRef
		}
		ref := flattenGitRef(ch.Git.Ref)
		// A ref distinct from Version must stay inline. PackageChart uses an inline ref to
		// select the checkout, while Version names the archive and values files it creates.
		if ref != "" && ref != ch.Version {
			ac.URL = gitURL + "@" + ref
		} else {
			ac.URL = gitURL
		}
		ac.GitPath = ch.Git.Path
	case ch.Local != nil && ch.Local.Path != "":
		ac.LocalPath = ch.Local.Path
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

func actionsFromGeneric(a api.ComponentActions) v1alpha1.ZarfComponentActions {
	return v1alpha1.ZarfComponentActions{
		OnCreate: actionSetFromGeneric(a.OnCreate),
		OnDeploy: actionSetFromGeneric(a.OnDeploy),
		OnRemove: actionSetFromGeneric(a.OnRemove),
	}
}

func actionSetFromGeneric(s api.ActionSet) v1alpha1.ZarfComponentActionSet {
	defaults := v1alpha1.ZarfComponentActionDefaults{
		Mute:            s.Defaults.Silent,
		MaxTotalSeconds: s.Defaults.MaxTotalSeconds,
		MaxRetries:      s.Defaults.Retries,
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

func actionSliceFromGeneric(actions []api.Action) []v1alpha1.ZarfComponentAction {
	var out []v1alpha1.ZarfComponentAction
	for _, a := range actions {
		out = append(out, actionFromGeneric(a))
	}
	return out
}

func actionFromGeneric(a api.Action) v1alpha1.ZarfComponentAction {
	aa := v1alpha1.ZarfComponentAction{
		Mute:                  a.Silent,
		Dir:                   a.Dir,
		Env:                   a.Env,
		Cmd:                   a.Cmd,
		Description:           a.Description,
		Wait:                  waitFromGeneric(a.Wait),
		SetVariables:          actionVariablesFromGeneric(a.SetVariables),
		DeprecatedSetVariable: a.DeprecatedSetVariable,
		Template:              boolPointer(a.EnableTemplating),
	}

	if a.MaxTotalSeconds != nil {
		v := int(*a.MaxTotalSeconds)
		aa.MaxTotalSeconds = &v
	}
	if a.Retries != nil {
		v := int(*a.Retries)
		aa.MaxRetries = &v
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

func waitFromGeneric(w *api.ActionWait) *v1alpha1.ZarfComponentActionWait {
	if w == nil {
		return nil
	}
	aw := &v1alpha1.ZarfComponentActionWait{}
	if w.Cluster != nil {
		aw.Cluster = &v1alpha1.ZarfComponentActionWaitCluster{
			Kind:      w.Cluster.Kind,
			Name:      w.Cluster.Name,
			Namespace: w.Cluster.Namespace,
			Condition: w.Cluster.Condition.Expression,
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

func variableToGeneric(v v1alpha1.Variable) api.Variable {
	return api.Variable{
		Name:       v.Name,
		Sensitive:  v.Sensitive,
		AutoIndent: v.AutoIndent,
		Pattern:    v.Pattern,
		Type:       api.VariableType(v.Type),
	}
}

func variableFromGeneric(v api.Variable) v1alpha1.Variable {
	return v1alpha1.Variable{
		Name:       v.Name,
		Sensitive:  v.Sensitive,
		AutoIndent: v.AutoIndent,
		Pattern:    v.Pattern,
		Type:       v1alpha1.VariableType(v.Type),
	}
}

func actionVariablesToGeneric(in []v1alpha1.Variable) []api.ActionVariable {
	if in == nil {
		return nil
	}
	out := make([]api.ActionVariable, 0, len(in))
	for _, variable := range in {
		out = append(out, api.ActionVariable{Name: variable.Name, Sensitive: variable.Sensitive, AutoIndent: variable.AutoIndent, Pattern: variable.Pattern, Type: string(variable.Type)})
	}
	return out
}

func actionVariablesFromGeneric(in []api.ActionVariable) []v1alpha1.Variable {
	if in == nil {
		return nil
	}
	out := make([]v1alpha1.Variable, 0, len(in))
	for _, variable := range in {
		out = append(out, v1alpha1.Variable{Name: variable.Name, Sensitive: variable.Sensitive, AutoIndent: variable.AutoIndent, Pattern: variable.Pattern, Type: v1alpha1.VariableType(variable.Type)})
	}
	return out
}

func interactiveVarsToGeneric(in []v1alpha1.InteractiveVariable) []api.InteractiveVariable {
	var out []api.InteractiveVariable
	for _, v := range in {
		out = append(out, api.InteractiveVariable{
			Variable:    variableToGeneric(v.Variable),
			Description: v.Description,
			Default:     v.Default,
			Prompt:      v.Prompt,
		})
	}
	return out
}

func interactiveVarsFromGeneric(in []api.InteractiveVariable) []v1alpha1.InteractiveVariable {
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

func constantsToGeneric(in []v1alpha1.Constant) []api.Constant {
	var out []api.Constant
	for _, c := range in {
		out = append(out, api.Constant{
			Name:        c.Name,
			Value:       c.Value,
			Description: c.Description,
			AutoIndent:  c.AutoIndent,
			Pattern:     c.Pattern,
		})
	}
	return out
}

func constantsFromGeneric(in []api.Constant) []v1alpha1.Constant {
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

func chartVarsToGeneric(in []v1alpha1.ZarfChartVariable) []api.ZarfChartVariable {
	var out []api.ZarfChartVariable
	for _, v := range in {
		out = append(out, api.ZarfChartVariable{Name: v.Name, Description: v.Description, Path: v.Path})
	}
	return out
}

func chartVarsFromGeneric(in []api.ZarfChartVariable) []v1alpha1.ZarfChartVariable {
	var out []v1alpha1.ZarfChartVariable
	for _, v := range in {
		out = append(out, v1alpha1.ZarfChartVariable{Name: v.Name, Description: v.Description, Path: v.Path})
	}
	return out
}

func dataInjectionsToGeneric(in []v1alpha1.ZarfDataInjection) []api.ZarfDataInjection {
	var out []api.ZarfDataInjection
	for _, d := range in {
		out = append(out, api.ZarfDataInjection{
			Source: d.Source,
			Target: api.ZarfContainerTarget{
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

func dataInjectionsFromGeneric(in []api.ZarfDataInjection) []v1alpha1.ZarfDataInjection {
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

func healthChecksToGeneric(in []v1alpha1.NamespacedObjectKindReference) []api.NamespacedObjectKindReference {
	var out []api.NamespacedObjectKindReference
	for _, h := range in {
		out = append(out, api.NamespacedObjectKindReference{
			APIVersion: h.APIVersion,
			Kind:       h.Kind,
			Namespace:  h.Namespace,
			Name:       h.Name,
		})
	}
	return out
}

func healthChecksFromGeneric(in []api.NamespacedObjectKindReference) []v1alpha1.NamespacedObjectKindReference {
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

func reposToGeneric(repos []string) []api.Repository {
	var out []api.Repository
	for _, url := range repos {
		out = append(out, api.Repository{URL: url})
	}
	return out
}

func reposFromGeneric(repos []api.Repository) []string {
	var out []string
	for _, r := range repos {
		url := r.URL
		if r.Ref != nil {
			if refStr := flattenGitRef(r.Ref); refStr != "" {
				// Strip any existing @ref from the URL before appending, matching the
				// split/join that transform.GitURLSplitRef + git.Clone perform at runtime.
				if urlNoRef, _, err := transform.GitURLSplitRef(url); err == nil {
					url = urlNoRef
				}
				url += "@" + refStr
			}
		}
		out = append(out, url)
	}
	return out
}

// flattenGitRef returns a ref string that, when passed through git.ParseRef at runtime,
// produces the same plumbing.ReferenceName as the structured ref intended
func flattenGitRef(ref *api.GitRef) string {
	if ref == nil {
		return ""
	}
	switch {
	case ref.Tag != "":
		return ref.Tag
	case ref.Commit != "":
		return ref.Commit
	case ref.Branch != "":
		return "refs/heads/" + ref.Branch
	}
	return ""
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
