// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"fmt"
	"slices"

	"github.com/zarf-dev/zarf/src/api/v1beta1"
)

// mergeComponentConfigSpec overlays override onto imported.
func mergeComponentConfigSpec(imported, override v1beta1.ComponentSpec) (v1beta1.ComponentSpec, error) {
	merged := imported

	if override.Target.OS != "" {
		merged.Target.OS = override.Target.OS
	}
	if override.Service.Name != "" {
		merged.Service = override.Service
	}

	merged.Files = append(merged.Files, override.Files...)
	merged.ImageArchives = append(merged.ImageArchives, override.ImageArchives...)
	merged.Repositories = append(merged.Repositories, override.Repositories...)
	merged.StateAccess = append(merged.StateAccess, override.StateAccess...)

	merged.Images = mergeImages(merged.Images, override.Images)
	var err error
	merged.Charts, err = mergeCharts(merged.Charts, override.Charts)
	if err != nil {
		return v1beta1.ComponentSpec{}, err
	}
	merged.Manifests = mergeManifests(merged.Manifests, override.Manifests)
	merged.Actions = mergeActions(merged.Actions, override.Actions)

	return merged, nil
}

// mergeImages matches non-empty IDs first, then names. Non-empty head fields take precedence.
func mergeImages(base, head []v1beta1.Image) []v1beta1.Image {
	out := slices.Clone(base)
	for _, h := range head {
		idx := -1
		if h.ID != "" {
			idx = slices.IndexFunc(out, func(img v1beta1.Image) bool { return img.ID == h.ID })
		}
		if idx == -1 {
			idx = slices.IndexFunc(out, func(img v1beta1.Image) bool { return img.Name == h.Name })
		}
		if idx == -1 {
			out = append(out, h)
			continue
		}
		if h.ID != "" {
			out[idx].ID = h.ID
		}
		if h.Name != "" {
			out[idx].Name = h.Name
		}
		if h.Source != "" {
			out[idx].Source = h.Source
		}
	}
	return out
}

func mergeCharts(base, headCharts []v1beta1.Chart) ([]v1beta1.Chart, error) {
	out := slices.Clone(base)
	for _, headChart := range headCharts {
		idx := slices.IndexFunc(out, func(chart v1beta1.Chart) bool { return chart.Name == headChart.Name })
		if idx == -1 {
			out = append(out, headChart)
			continue
		}
		importedChart := out[idx]
		if err := validateChartSourceOverride(importedChart, headChart); err != nil {
			return nil, err
		}
		if headChart.Namespace != "" {
			importedChart.Namespace = headChart.Namespace
		}
		if headChart.ReleaseName != "" {
			importedChart.ReleaseName = headChart.ReleaseName
		}
		// Preserve different source types so resolved validation rejects conflicts.
		if headChart.HelmRepository != nil {
			source := *headChart.HelmRepository
			if importedChart.HelmRepository != nil {
				source = *importedChart.HelmRepository
				if headChart.HelmRepository.Version != "" {
					source.Version = headChart.HelmRepository.Version
				}
			}
			importedChart.HelmRepository = &source
		}
		if headChart.Git != nil {
			source := *headChart.Git
			if importedChart.Git != nil {
				source = *importedChart.Git
				if headChart.Git.Ref != (v1beta1.GitRef{}) {
					source.Ref = headChart.Git.Ref
				}
			}
			importedChart.Git = &source
		}
		if headChart.Local != nil && importedChart.Local == nil {
			importedChart.Local = headChart.Local
		}
		if headChart.OCI != nil {
			source := *headChart.OCI
			if importedChart.OCI != nil {
				source = *importedChart.OCI
				if headChart.OCI.Ref != (v1beta1.OCIRef{}) {
					source.Ref = headChart.OCI.Ref
				}
			}
			importedChart.OCI = &source
		}
		if headChart.ServerSideApply != "" {
			importedChart.ServerSideApply = headChart.ServerSideApply
		}
		if headChart.SkipWait {
			importedChart.SkipWait = true
		}
		if headChart.SkipSchemaValidation {
			importedChart.SkipSchemaValidation = true
		}
		importedChart.ValuesFiles = append(importedChart.ValuesFiles, headChart.ValuesFiles...)
		importedChart.Values = append(importedChart.Values, headChart.Values...)
		out[idx] = importedChart
	}
	return out, nil
}

func validateChartSourceOverride(base, head v1beta1.Chart) error {
	type fieldOverride struct {
		name, base, head string
	}
	var fields []fieldOverride
	if base.HelmRepository != nil && head.HelmRepository != nil {
		name := base.HelmRepository.Name
		if name == "" {
			name = base.Name
		}
		fields = append(fields,
			fieldOverride{"helmRepository.url", base.HelmRepository.URL, head.HelmRepository.URL},
			fieldOverride{"helmRepository.name", name, head.HelmRepository.Name},
		)
	}
	if base.Git != nil && head.Git != nil {
		fields = append(fields,
			fieldOverride{"git.url", base.Git.URL, head.Git.URL},
			fieldOverride{"git.path", base.Git.Path, head.Git.Path},
		)
	}
	if base.OCI != nil && head.OCI != nil {
		fields = append(fields, fieldOverride{"oci.url", base.OCI.URL, head.OCI.URL})
	}
	if base.Local != nil && head.Local != nil {
		fields = append(fields, fieldOverride{"local.path", base.Local.Path, head.Local.Path})
	}
	for _, field := range fields {
		if field.head != "" && field.head != field.base {
			return fmt.Errorf("chart %q cannot override %s from %q to %q; only source version or ref can be overridden", head.Name, field.name, field.base, field.head)
		}
	}
	return nil
}

func mergeManifests(base, head []v1beta1.Manifest) []v1beta1.Manifest {
	out := slices.Clone(base)
	for _, h := range head {
		idx := slices.IndexFunc(out, func(manifest v1beta1.Manifest) bool { return manifest.Name == h.Name })
		if idx == -1 {
			out = append(out, h)
			continue
		}
		m := out[idx]
		if h.Namespace != "" {
			m.Namespace = h.Namespace
		}
		m.Files = append(m.Files, h.Files...)
		if len(h.Kustomize.Files) > 0 || h.Kustomize.AllowAnyDirectory || h.Kustomize.EnablePlugins {
			m.Kustomize.Files = append(m.Kustomize.Files, h.Kustomize.Files...)
			if h.Kustomize.AllowAnyDirectory {
				m.Kustomize.AllowAnyDirectory = true
			}
			if h.Kustomize.EnablePlugins {
				m.Kustomize.EnablePlugins = true
			}
		}
		if h.ServerSideApply != "" {
			m.ServerSideApply = h.ServerSideApply
		}
		if h.SkipWait {
			m.SkipWait = true
		}
		if h.EnableTemplating {
			m.EnableTemplating = true
		}
		out[idx] = m
	}
	return out
}

func mergeActions(base, head v1beta1.ComponentActions) v1beta1.ComponentActions {
	return v1beta1.ComponentActions{
		OnCreate: mergeActionSet(base.OnCreate, head.OnCreate),
		OnDeploy: mergeActionSet(base.OnDeploy, head.OnDeploy),
		OnRemove: mergeActionSet(base.OnRemove, head.OnRemove),
	}
}

func mergeActionSet(base, head v1beta1.ComponentActionSet) v1beta1.ComponentActionSet {
	if head.Defaults != nil {
		base.Defaults = head.Defaults
	}
	base.Before = append(base.Before, head.Before...)
	base.OnSuccess = append(base.OnSuccess, head.OnSuccess...)
	base.OnFailure = append(base.OnFailure, head.OnFailure...)
	return base
}

// fixPathsV1Beta1 rebases a component spec's relative resource paths to be relative to the head node,
// where relativeToHead is the imported config's directory relative to the importing component.
func fixPathsV1Beta1(spec v1beta1.ComponentSpec, relativeToHead string) v1beta1.ComponentSpec {
	for i := range spec.Files {
		spec.Files[i].Source = makePathRelativeTo(spec.Files[i].Source, relativeToHead)
	}
	for i := range spec.ImageArchives {
		spec.ImageArchives[i].Path = makePathRelativeTo(spec.ImageArchives[i].Path, relativeToHead)
	}
	for i := range spec.Charts {
		if spec.Charts[i].Local != nil && spec.Charts[i].Local.Path != "" {
			spec.Charts[i].Local.Path = makePathRelativeTo(spec.Charts[i].Local.Path, relativeToHead)
		}
		for j := range spec.Charts[i].ValuesFiles {
			spec.Charts[i].ValuesFiles[j].Path = makePathRelativeTo(spec.Charts[i].ValuesFiles[j].Path, relativeToHead)
		}
	}
	for i := range spec.Manifests {
		for j := range spec.Manifests[i].Files {
			spec.Manifests[i].Files[j] = makePathRelativeTo(spec.Manifests[i].Files[j], relativeToHead)
		}
		for j := range spec.Manifests[i].Kustomize.Files {
			spec.Manifests[i].Kustomize.Files[j] = makePathRelativeTo(spec.Manifests[i].Kustomize.Files[j], relativeToHead)
		}
	}

	var defaultDir string
	if spec.Actions.OnCreate.Defaults != nil {
		defaultDir = spec.Actions.OnCreate.Defaults.Dir
	}
	spec.Actions.OnCreate.Before = fixActionPathsV1Beta1(spec.Actions.OnCreate.Before, defaultDir, relativeToHead)
	spec.Actions.OnCreate.OnSuccess = fixActionPathsV1Beta1(spec.Actions.OnCreate.OnSuccess, defaultDir, relativeToHead)
	spec.Actions.OnCreate.OnFailure = fixActionPathsV1Beta1(spec.Actions.OnCreate.OnFailure, defaultDir, relativeToHead)

	return spec
}

func fixActionPathsV1Beta1(actions []v1beta1.ComponentAction, defaultDir, relativeToHead string) []v1beta1.ComponentAction {
	for i := range actions {
		var composed string
		if actions[i].Dir != nil {
			composed = makePathRelativeTo(*actions[i].Dir, relativeToHead)
		} else {
			composed = makePathRelativeTo(defaultDir, relativeToHead)
		}
		actions[i].Dir = &composed
	}
	return actions
}
