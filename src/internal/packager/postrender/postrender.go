// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package postrender applies user-defined post-renderers (currently Kustomize) to Helm-rendered chart manifests.
package postrender

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/internal/packager/kustomize"
	"github.com/zarf-dev/zarf/src/pkg/template"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	"sigs.k8s.io/kustomize/api/krusty"
	ktypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/resid"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
	"sigs.k8s.io/yaml"
)

const (
	// filenameAnnotation is set by Helm v4 on every document passed to a post-renderer so it can re-associate the
	// output with source templates. Helm does not export it.
	filenameAnnotation = "postrenderer.helm.sh/postrender-filename"

	kustomizeDir       = "/postrender"
	kustomizeResources = "resources.yaml"
)

// Renderer is a chart's post-renderers with templating applied. A nil *Renderer passes manifests through unchanged.
type Renderer struct {
	chart    string
	strategy api.PostRenderStrategy
	steps    []kustomization
}

// kustomization is one post-renderer resolved into a Kustomization.
type kustomization struct {
	spec ktypes.Kustomization
	// templated reports whether this post-renderer enables templating.
	templated bool
	// redact hides Kustomize error details, which may echo templated (possibly secret) content from this
	// post-renderer or, through its input, from an earlier one.
	redact bool
}

// pass is one run of the post-renderer chain over a subset of the documents.
type pass struct {
	// group labels the subset in errors when the chain runs more than once.
	group string
	nodes []*kyaml.RNode
}

// NeedsTemplating reports whether any of chart's post-renderers enables templating, so callers only build template
// objects (which can be costly) when New uses them.
func NeedsTemplating(chart api.Chart) bool {
	for _, pr := range chart.PostRenderers {
		if pr.Kustomize != nil && pr.Kustomize.EnableTemplating {
			return true
		}
	}
	return false
}

// New resolves chart's post-renderers, rendering templated fields with objs once. objs is only used when
// NeedsTemplating(chart). It returns nil when chart has no post-renderers. Errors identify the chart.
func New(ctx context.Context, chart api.Chart, objs template.Objects) (*Renderer, error) {
	if len(chart.PostRenderers) == 0 {
		return nil, nil
	}
	if !chart.PostRenderStrategy.IsValid() {
		return nil, fmt.Errorf("chart %q: unknown post-render strategy %q, must be one of %q, %q or %q", chart.Name,
			chart.PostRenderStrategy, api.PostRenderStrategyNoHooks, api.PostRenderStrategyCombined, api.PostRenderStrategySeparate)
	}
	r := &Renderer{chart: chart.Name, strategy: chart.PostRenderStrategy}
	templatedBefore := false
	for i, pr := range chart.PostRenderers {
		spec, err := buildKustomization(ctx, pr, objs)
		if err != nil {
			return nil, fmt.Errorf("chart %q: post-renderer at index %d: %w", chart.Name, i, err)
		}
		templated := pr.Kustomize.EnableTemplating
		r.steps = append(r.steps, kustomization{spec: spec, templated: templated, redact: templated || templatedBefore})
		templatedBefore = templatedBefore || templated
	}
	return r, nil
}

// Run applies the post-renderers to a Helm-rendered YAML stream according to the chart's strategy. It satisfies
// Helm's post-renderer interface. Errors identify the chart.
func (r *Renderer) Run(in *bytes.Buffer) (*bytes.Buffer, error) {
	if r == nil {
		return in, nil
	}
	out, err := r.run(in.Bytes())
	if err != nil {
		return nil, fmt.Errorf("chart %q: %w", r.chart, err)
	}
	return bytes.NewBufferString(out), nil
}

func (r *Renderer) run(manifests []byte) (string, error) {
	nodes, err := parse(manifests)
	if err != nil {
		return "", fmt.Errorf("parsing rendered manifests for post-rendering: %w", err)
	}
	passes, untouched := plan(r.strategy, nodes)
	var out []*kyaml.RNode
	for _, p := range passes {
		rendered, err := r.runPass(p)
		if err != nil {
			return "", err
		}
		out = append(out, rendered...)
	}
	out = append(out, untouched...)
	s, err := kio.StringAll(out)
	if err != nil {
		return "", fmt.Errorf("serializing post-rendered manifests: %w", err)
	}
	return s, nil
}

// parse splits a YAML stream into documents, dropping empty and comment-only documents. Lists are kept intact here
// so hook partitioning sees each document the way Helm does.
func parse(manifests []byte) ([]*kyaml.RNode, error) {
	r := kio.ByteReader{
		Reader:                bytes.NewReader(manifests),
		OmitReaderAnnotations: true,
		DisableUnwrapping:     true,
	}
	return r.Read()
}

// plan splits nodes into the passes strategy runs the chain over and the documents it leaves untouched, preserving
// relative order. Hooks are the documents with Helm's hook annotation.
func plan(s api.PostRenderStrategy, nodes []*kyaml.RNode) (passes []pass, untouched []*kyaml.RNode) {
	if s == api.PostRenderStrategyCombined {
		return []pass{{nodes: nodes}}, nil
	}
	var hooks, resources []*kyaml.RNode
	for _, n := range nodes {
		if _, ok := n.GetAnnotations()[releasev1.HookAnnotation]; ok {
			hooks = append(hooks, n)
		} else {
			resources = append(resources, n)
		}
	}
	if s == api.PostRenderStrategySeparate {
		return []pass{{group: "resources", nodes: resources}, {group: "hooks", nodes: hooks}}, nil
	}
	return []pass{{nodes: resources}}, hooks
}

// runPass feeds the pass's documents through each post-renderer in order. An empty input skips Kustomize entirely.
func (r *Renderer) runPass(p pass) ([]*kyaml.RNode, error) {
	nodes, err := expandLists(p.nodes)
	if err != nil {
		return nil, err
	}
	label := ""
	if p.group != "" {
		label = " (" + p.group + ")"
	}
	for i, k := range r.steps {
		if len(nodes) == 0 {
			return nil, nil
		}
		out, err := runKustomize(nodes, k.spec)
		if err != nil {
			h := hint(err, r.strategy, p.group)
			if !k.redact {
				return nil, fmt.Errorf("post-renderer at index %d%s: kustomize build failed: %w%s", i, label, err, h)
			}
			reason := "templating is enabled"
			if !k.templated {
				reason = "an earlier post-renderer enables templating"
			}
			return nil, fmt.Errorf("post-renderer at index %d%s: kustomize build failed (details redacted because %s)%s",
				i, label, reason, h)
		}
		nodes = out
	}
	return nodes, nil
}

// runKustomize runs a kustomization over nodes using an in-memory filesystem and no plugins.
func runKustomize(nodes []*kyaml.RNode, spec ktypes.Kustomization) ([]*kyaml.RNode, error) {
	resources, err := kio.StringAll(nodes)
	if err != nil {
		return nil, fmt.Errorf("serializing kustomize input: %w", err)
	}
	k, err := yaml.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("marshalling kustomization: %w", err)
	}

	fs := filesys.MakeFsInMemory()
	if err := fs.MkdirAll(kustomizeDir); err != nil {
		return nil, err
	}
	if err := fs.WriteFile(path.Join(kustomizeDir, kustomizeResources), []byte(resources)); err != nil {
		return nil, err
	}
	if err := fs.WriteFile(path.Join(kustomizeDir, "kustomization.yaml"), k); err != nil {
		return nil, err
	}

	// Default options keep input order (ReorderOptionNone), restrict loading to the root, and disable plugins.
	resMap, err := kustomize.RunKustomizer(fs, kustomizeDir, krusty.MakeDefaultOptions())
	if err != nil {
		return nil, err
	}
	return resMap.ToRNodeSlice(), nil
}

// hint returns remediation advice for common Kustomize failures in a pass. It never includes content from err, so it
// is safe to use with redacted errors. Matching relies on Kustomize's error strings, which tests pin.
func hint(err error, strategy api.PostRenderStrategy, group string) string {
	combined := strategy == api.PostRenderStrategyCombined
	mayContainHooks := combined || group == "hooks"
	msg := err.Error()
	switch {
	case strings.Contains(msg, "already registered id"):
		if combined {
			return fmt.Sprintf(" (hint: duplicate resource id; a hook may duplicate a regular resource; use the "+
				"%q or %q post-render strategy)", api.PostRenderStrategyNoHooks, api.PostRenderStrategySeparate)
		}
		return " (hint: duplicate resource id; Kustomize requires each resource's kind, name and namespace to be unique)"
	case strings.Contains(msg, "missing metadata.name"):
		if mayContainHooks {
			return fmt.Sprintf(" (hint: missing metadata.name; Kustomize cannot process resources that only set "+
				"metadata.generateName, which is common for hooks; use the %q post-render strategy)", api.PostRenderStrategyNoHooks)
		}
		return " (hint: missing metadata.name; a regular (non-hook) resource that only sets metadata.generateName " +
			"cannot be post-rendered by Kustomize)"
	case strings.Contains(msg, "no resource matches strategic merge patch"):
		return fmt.Sprintf(" (hint: a patch without a target must match a resource in every Kustomize pass; hooks "+
			"are not post-rendered under %q and are post-rendered separately from regular resources under %q; "+
			"add a target to the patch)", api.PostRenderStrategyNoHooks, api.PostRenderStrategySeparate)
	}
	return ""
}

// expandLists inlines the items of FooList documents, as Kustomize would, but in place and carrying the list's
// Helm filename annotation onto items that lack one so Helm can still attribute them to their source template.
func expandLists(nodes []*kyaml.RNode) ([]*kyaml.RNode, error) {
	out := make([]*kyaml.RNode, 0, len(nodes))
	for _, n := range nodes {
		items := n.Field("items")
		if !strings.HasSuffix(n.GetKind(), "List") || items == nil {
			out = append(out, n)
			continue
		}
		if kyaml.IsMissingOrNull(items.Value) {
			continue
		}
		elements, err := items.Value.Elements()
		if err != nil {
			return nil, fmt.Errorf("expanding %s items: %w", n.GetKind(), err)
		}
		filename := n.GetAnnotations()[filenameAnnotation]
		for _, item := range elements {
			if _, ok := item.GetAnnotations()[filenameAnnotation]; !ok && filename != "" {
				if err := item.PipeE(kyaml.SetAnnotation(filenameAnnotation, filename)); err != nil {
					return nil, fmt.Errorf("annotating %s item: %w", n.GetKind(), err)
				}
			}
			out = append(out, item)
		}
	}
	return out, nil
}

// buildKustomization resolves a post-renderer into a Kustomization, applying Zarf templating when enabled. The
// caller's structs are not modified.
func buildKustomization(ctx context.Context, r api.PostRenderer, objs template.Objects) (ktypes.Kustomization, error) {
	k := r.Kustomize
	if k == nil {
		return ktypes.Kustomization{}, fmt.Errorf("kustomize is not set")
	}
	kust := ktypes.Kustomization{
		TypeMeta:  ktypes.TypeMeta{APIVersion: ktypes.KustomizationVersion, Kind: ktypes.KustomizationKind},
		Resources: []string{kustomizeResources},
	}
	for j, p := range k.Patches {
		patch := p.Patch
		if k.EnableTemplating {
			var err error
			if patch, err = template.Apply(ctx, patch, objs); err != nil {
				return ktypes.Kustomization{}, fmt.Errorf("kustomize patch at index %d: templating failed: %w", j, err)
			}
			if strings.TrimSpace(patch) == "" {
				return ktypes.Kustomization{}, fmt.Errorf("kustomize patch at index %d is empty after templating", j)
			}
		}
		kp := ktypes.Patch{Patch: patch}
		if t := p.Target; t != nil {
			kp.Target = &ktypes.Selector{
				ResId: resid.ResId{
					Gvk:       resid.Gvk{Group: t.Group, Version: t.Version, Kind: t.Kind},
					Name:      t.Name,
					Namespace: t.Namespace,
				},
				LabelSelector:      t.LabelSelector,
				AnnotationSelector: t.AnnotationSelector,
			}
		}
		kust.Patches = append(kust.Patches, kp)
	}
	for j, img := range k.Images {
		ki := ktypes.Image{Name: img.Name, NewName: img.NewName, NewTag: img.NewTag, Digest: img.Digest}
		if k.EnableTemplating {
			for _, f := range []*string{&ki.Name, &ki.NewName, &ki.NewTag, &ki.Digest} {
				var err error
				if *f, err = template.Apply(ctx, *f, objs); err != nil {
					return ktypes.Kustomization{}, fmt.Errorf("image override at index %d: templating failed: %w", j, err)
				}
			}
			if strings.TrimSpace(ki.Name) == "" {
				return ktypes.Kustomization{}, fmt.Errorf("image override at index %d: name is empty after templating", j)
			}
		}
		kust.Images = append(kust.Images, ki)
	}
	return kust, nil
}
