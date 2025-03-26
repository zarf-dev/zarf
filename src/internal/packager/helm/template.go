// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/internal/packager/template"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/variables"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/releaseutil"

	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/message"
)

// TemplateChart generates a helm template from a given chart.
func TemplateChart(ctx context.Context, zarfChart v1alpha1.ZarfChart, chart *chart.Chart, values chartutil.Values, chartPath string,
	kubeVersion string, variableConfig *variables.VariableConfig) (string, error) {
	if variableConfig == nil {
		variableConfig = template.GetZarfVariableConfig(ctx)
	}
	l := logger.From(ctx)
	spinner := message.NewProgressSpinner("Templating helm chart %s", zarfChart.Name)
	defer spinner.Stop()
	l.Debug("templating helm chart", "name", zarfChart.Name)

	actionCfg, err := createActionConfig(ctx, zarfChart.Namespace)
	if err != nil {
		return "", err
	}

	// Bind the helm action.
	client := action.NewInstall(actionCfg)

	client.DryRun = true
	client.Replace = true // Skip the name check.
	client.ClientOnly = true
	client.IncludeCRDs = true
	// TODO: Further research this with regular/OCI charts
	client.Verify = false
	client.InsecureSkipTLSverify = config.CommonOptions.InsecureSkipTLSVerify
	if kubeVersion != "" {
		parsedKubeVersion, err := chartutil.ParseKubeVersion(kubeVersion)
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
	templatedChart, err := client.RunWithContext(ctx, chart, values)
	if err != nil {
		return "", fmt.Errorf("error generating helm chart template: %w", err)
	}

	manifest := templatedChart.Manifest

	for _, hook := range templatedChart.Hooks {
		manifest += fmt.Sprintf("\n---\n%s", hook.Manifest)
	}

	spinner.Success()

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

func (tr *templateRenderer) Run(renderedManifests *bytes.Buffer) (*bytes.Buffer, error) {
	// This is very low cost and consistent for how we replace elsewhere, also good for debugging
	resources, err := getTemplatedManifests(renderedManifests, tr.variableConfig, tr.actionConfig)
	if err != nil {
		return nil, err
	}

	finalManifestsOutput := bytes.NewBuffer(nil)

	for _, resource := range resources {
		fmt.Fprintf(finalManifestsOutput, "---\n# Source: %s\n%s\n", resource.Name, resource.Content)
	}

	return finalManifestsOutput, nil
}

func getTemplatedManifests(renderedManifests *bytes.Buffer, variableConfig *variables.VariableConfig, actionConfig *action.Configuration) ([]releaseutil.Manifest, error) {
	tempDir, err := utils.MakeTempDir("")
	if err != nil {
		return nil, fmt.Errorf("unable to create tmpdir:  %w", err)
	}
	path := filepath.Join(tempDir, "chart.yaml")

	if err := os.WriteFile(path, renderedManifests.Bytes(), helpers.ReadWriteUser); err != nil {
		return nil, fmt.Errorf("unable to write the post-render file for the helm chart")
	}

	// Run the template engine against the chart output
	if err := variableConfig.ReplaceTextTemplate(path); err != nil {
		return nil, fmt.Errorf("error templating the helm chart: %w", err)
	}

	// Read back the templated file contents
	buff, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading temporary post-rendered helm chart: %w", err)
	}

	// Use helm to re-split the manifest byte (same call used by helm to pass this data to postRender)
	_, resources, err := releaseutil.SortManifests(map[string]string{path: string(buff)},
		actionConfig.Capabilities.APIVersions,
		releaseutil.InstallOrder,
	)
	if err != nil {
		return nil, fmt.Errorf("error re-rendering helm output: %w", err)
	}
	return resources, nil
}
