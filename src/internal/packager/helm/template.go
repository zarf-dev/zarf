// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/variables"
	"github.com/zarf-dev/zarf/src/types"

	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/common"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/release"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"
)

// TemplateChart generates a helm template from a given chart.
func TemplateChart(ctx context.Context, zarfChart v1alpha1.ZarfChart, chart *chartv2.Chart, values common.Values,
	kubeVersion string, variableConfig *variables.VariableConfig, isInteractive bool, remoteOptions types.RemoteOptions) (string, error) {
	if variableConfig == nil {
		variableConfig = template.GetZarfVariableConfig(ctx, isInteractive)
	}
	l := logger.From(ctx)
	l.Debug("templating helm chart", "name", zarfChart.Name)

	actionCfg, err := createActionConfig(ctx, zarfChart.Namespace)
	if err != nil {
		return "", err
	}

	// Bind the helm action.
	client := action.NewInstall(actionCfg)

	client.DryRunStrategy = action.DryRunClient
	client.Replace = true // Skip the name check.
	client.IncludeCRDs = true
	// TODO: Further research this with regular/OCI charts
	client.Verify = false
	// client.PlainHTTP is intentionally left unset: RunWithContext (below) never
	// reads it for an already-loaded chart like this one — Helm only consults it in
	// LocateChart, which resolves a chart by name/reference before loading, a step
	// TemplateChart doesn't perform.
	client.InsecureSkipTLSVerify = remoteOptions.InsecureSkipTLSVerify
	if kubeVersion != "" {
		parsedKubeVersion, err := common.ParseKubeVersion(kubeVersion)
		if err != nil {
			return "", fmt.Errorf("invalid kube version %s: %w", kubeVersion, err)
		}
		client.KubeVersion = parsedKubeVersion
	}
	client.ReleaseName = zarfChart.ReleaseName

	// If no release name is specified, use the chart name.
	if client.ReleaseName == "" {
		client.ReleaseName = zarfChart.Name
	}

	// Namespace must be specified.
	client.Namespace = zarfChart.Namespace

	client.PostRenderer, err = newTemplateRenderer(actionCfg, variableConfig)
	if err != nil {
		return "", fmt.Errorf("unable to create helm renderer: %w", err)
	}

	// Perform the loadedChart installation.
	templatedReleaser, err := client.RunWithContext(ctx, chart, values)
	if err != nil {
		return "", fmt.Errorf("error generating helm chart template: %w", err)
	}

	templatedRelease, err := release.NewAccessor(templatedReleaser)
	if err != nil {
		return "", err
	}

	manifest := templatedRelease.Manifest()

	for _, hook := range templatedRelease.Hooks() {
		hook, err := release.NewHookAccessor(hook)
		if err != nil {
			return "", err
		}
		manifest += fmt.Sprintf("\n---\n%s", hook.Manifest())
	}

	return manifest, nil
}

type templateRenderer struct {
	actionConfig   *action.Configuration
	variableConfig *variables.VariableConfig
}

func newTemplateRenderer(actionConfig *action.Configuration, vc *variables.VariableConfig) (*templateRenderer, error) {
	rend := &templateRenderer{
		actionConfig:   actionConfig,
		variableConfig: vc,
	}
	return rend, nil
}

// Run satisfies the Helm post-renderer interface and templates the Zarf vars in the rendered manifests.
func (tr *templateRenderer) Run(renderedManifests *bytes.Buffer) (*bytes.Buffer, error) {
	// This is very low cost and consistent for how we replace elsewhere, also good for debugging
	hooks, resources, err := getTemplatedManifests(renderedManifests, tr.variableConfig, tr.actionConfig)
	if err != nil {
		return nil, err
	}

	finalManifestsOutput := bytes.NewBuffer(nil)

	for _, hook := range hooks {
		fmt.Fprintf(finalManifestsOutput, "---\n# Source: %s\n%s\n", hook.Path, hook.Manifest)
	}

	for _, resource := range resources {
		fmt.Fprintf(finalManifestsOutput, "---\n# Source: %s\n%s\n", resource.Name, resource.Content)
	}

	return finalManifestsOutput, nil
}

func getTemplatedManifests(renderedManifests *bytes.Buffer, variableConfig *variables.VariableConfig, actionConfig *action.Configuration) (_ []*releasev1.Hook, _ []releaseutil.Manifest, err error) {
	tmpdir, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create tmpdir:  %w", err)
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(tmpdir))
	}()
	path := filepath.Join(tmpdir, "chart.yaml")

	if err := os.WriteFile(path, renderedManifests.Bytes(), helpers.ReadWriteUser); err != nil {
		return nil, nil, fmt.Errorf("unable to write the post-render file for the helm chart")
	}

	// Run the template engine against the chart output
	if err := variableConfig.ReplaceTextTemplate(path); err != nil {
		return nil, nil, fmt.Errorf("error templating the helm chart: %w", err)
	}

	// Read back the templated file contents
	buff, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("error reading temporary post-rendered helm chart: %w", err)
	}

	// Use helm to re-split the manifest byte (same call used by helm to pass this data to postRender)
	hooks, resources, err := releaseutil.SortManifests(map[string]string{path: string(buff)},
		actionConfig.Capabilities.APIVersions,
		releaseutil.InstallOrder,
	)
	if err != nil {
		// STOPGAP (zarf #4977, fixed upstream by helm/helm#32204): Helm v4 splits
		// kind:List items into separate documents but keeps YAML anchors, dangling
		// any alias that crosses items. Repair the anchor scope and retry once.
		if repaired, rerr := resolveCrossDocumentAnchors(buff); rerr == nil && repaired != nil {
			hooks, resources, err = releaseutil.SortManifests(map[string]string{path: string(repaired)},
				actionConfig.Capabilities.APIVersions,
				releaseutil.InstallOrder,
			)
		}
		if err != nil {
			return nil, nil, fmt.Errorf("error re-rendering helm output: %w", err)
		}
	}
	return hooks, resources, nil
}

// resolveCrossDocumentAnchors materializes YAML aliases against anchors collected
// across the whole stream, returning (nil, nil) when there is nothing to repair.
//
// STOPGAP for zarf #4977 (fixed upstream by helm/helm#32204); remove once a fixed
// Helm is vendored.
func resolveCrossDocumentAnchors(content []byte) ([]byte, error) {
	// goccy's parser tolerates the dangling alias that yaml.v3 rejects at decode time.
	file, err := parser.ParseBytes(content, 0)
	if err != nil {
		return nil, err
	}

	collector := &anchorCollector{anchors: map[string]ast.Node{}}
	for _, doc := range file.Docs {
		if doc.Body != nil {
			ast.Walk(collector, doc.Body)
		}
	}
	if len(collector.anchors) == 0 {
		return nil, nil
	}

	resolver := &aliasResolver{anchors: collector.anchors}
	for _, doc := range file.Docs {
		if doc.Body != nil {
			ast.Walk(resolver, doc.Body)
		}
	}
	if resolver.resolved == 0 {
		return nil, nil
	}

	return []byte(file.String()), nil
}

// anchorCollector walks a YAML AST collecting anchor definitions keyed by name.
type anchorCollector struct {
	anchors map[string]ast.Node
}

func (c *anchorCollector) Visit(node ast.Node) ast.Visitor {
	if anchor, ok := node.(*ast.AnchorNode); ok && anchor.Name != nil && anchor.Value != nil {
		c.anchors[anchor.Name.String()] = anchor.Value
	}
	return c
}

// aliasResolver walks a YAML AST replacing alias references with the matching
// anchor's value node, in place.
type aliasResolver struct {
	anchors  map[string]ast.Node
	resolved int
}

// resolve returns the anchor value node for an alias, or nil if node is not a
// resolvable alias.
func (r *aliasResolver) resolve(node ast.Node) ast.Node {
	alias, ok := node.(*ast.AliasNode)
	if !ok || alias.Value == nil {
		return nil
	}
	value, found := r.anchors[alias.Value.String()]
	if !found {
		return nil
	}
	r.resolved++
	return value
}

func (r *aliasResolver) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.MappingValueNode:
		if value := r.resolve(n.Value); value != nil {
			n.Value = value
		}
	case *ast.SequenceNode:
		for i, entry := range n.Values {
			if value := r.resolve(entry); value != nil {
				n.Values[i] = value
			}
		}
	}
	return r
}
