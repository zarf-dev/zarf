// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import "github.com/zarf-dev/zarf/src/api/v1beta1"

// mergeComponentSpec overlays the head spec onto the base spec. The base is the imported component
// config; the head is the importing package component, which is authoritative on conflicts.
func mergeComponentSpec(base, head v1beta1.ComponentSpec) v1beta1.ComponentSpec {
	merged := base

	if head.Target.OS != "" {
		merged.Target.OS = head.Target.OS
	}
	if head.Target.Architecture != "" {
		merged.Target.Architecture = head.Target.Architecture
	}
	if head.Target.Flavor != "" {
		merged.Target.Flavor = head.Target.Flavor
	}
	if head.Service != "" {
		merged.Service = head.Service
	}

	merged.Files = append(merged.Files, head.Files...)
	merged.ImageArchives = append(merged.ImageArchives, head.ImageArchives...)
	merged.Repositories = append(merged.Repositories, head.Repositories...)
	merged.StateAccess = append(merged.StateAccess, head.StateAccess...)

	merged.Images = mergeImages(merged.Images, head.Images)
	merged.Charts = mergeCharts(merged.Charts, head.Charts)
	merged.Manifests = mergeManifests(merged.Manifests, head.Manifests)
	merged.Actions = mergeActions(merged.Actions, head.Actions)

	return merged
}

// mergeImages merges images by name. The head value of source (and future fields) wins when set.
func mergeImages(base, head []v1beta1.Image) []v1beta1.Image {
	out := append([]v1beta1.Image{}, base...)
	for _, h := range head {
		idx := indexByName(len(out), func(i int) string { return out[i].Name }, h.Name)
		if idx == -1 {
			out = append(out, h)
			continue
		}
		if h.Source != "" {
			out[idx].Source = h.Source
		}
	}
	return out
}

func mergeCharts(base, head []v1beta1.Chart) []v1beta1.Chart {
	out := append([]v1beta1.Chart{}, base...)
	for _, h := range head {
		idx := indexByName(len(out), func(i int) string { return out[i].Name }, h.Name)
		if idx == -1 {
			out = append(out, h)
			continue
		}
		c := out[idx]
		if h.Namespace != "" {
			c.Namespace = h.Namespace
		}
		if h.ReleaseName != "" {
			c.ReleaseName = h.ReleaseName
		}
		if h.HelmRepository != nil {
			c.HelmRepository = h.HelmRepository
		}
		if h.Git != nil {
			c.Git = h.Git
		}
		if h.Local != nil {
			c.Local = h.Local
		}
		if h.OCI != nil {
			c.OCI = h.OCI
		}
		if h.ServerSideApply != "" {
			c.ServerSideApply = h.ServerSideApply
		}
		if h.SkipWait {
			c.SkipWait = true
		}
		if h.SkipSchemaValidation {
			c.SkipSchemaValidation = true
		}
		// FIXME: check if the ordering is correct here
		c.ValuesFiles = append(c.ValuesFiles, h.ValuesFiles...)
		c.Values = append(c.Values, h.Values...)
		out[idx] = c
	}
	return out
}

func mergeManifests(base, head []v1beta1.Manifest) []v1beta1.Manifest {
	out := append([]v1beta1.Manifest{}, base...)
	for _, h := range head {
		idx := indexByName(len(out), func(i int) string { return out[i].Name }, h.Name)
		if idx == -1 {
			out = append(out, h)
			continue
		}
		m := out[idx]
		if h.Namespace != "" {
			m.Namespace = h.Namespace
		}
		m.Files = append(m.Files, h.Files...)
		if h.Kustomize != nil {
			if m.Kustomize == nil {
				m.Kustomize = h.Kustomize
			} else {
				m.Kustomize.Files = append(m.Kustomize.Files, h.Kustomize.Files...)
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
	base.Defaults = head.Defaults
	base.Before = append(base.Before, head.Before...)
	base.OnSuccess = append(base.OnSuccess, head.OnSuccess...)
	base.OnFailure = append(base.OnFailure, head.OnFailure...)
	return base
}

func indexByName(n int, nameAt func(int) string, name string) int {
	for i := 0; i < n; i++ {
		if nameAt(i) == name {
			return i
		}
	}
	return -1
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
		if spec.Charts[i].Local != nil {
			spec.Charts[i].Local.Path = makePathRelativeTo(spec.Charts[i].Local.Path, relativeToHead)
		}
		for j := range spec.Charts[i].ValuesFiles {
			spec.Charts[i].ValuesFiles[j] = makePathRelativeTo(spec.Charts[i].ValuesFiles[j], relativeToHead)
		}
	}
	for i := range spec.Manifests {
		for j := range spec.Manifests[i].Files {
			spec.Manifests[i].Files[j] = makePathRelativeTo(spec.Manifests[i].Files[j], relativeToHead)
		}
		if spec.Manifests[i].Kustomize != nil {
			for j := range spec.Manifests[i].Kustomize.Files {
				spec.Manifests[i].Kustomize.Files[j] = makePathRelativeTo(spec.Manifests[i].Kustomize.Files[j], relativeToHead)
			}
		}
	}

	defaultDir := spec.Actions.OnCreate.Defaults.Dir
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
