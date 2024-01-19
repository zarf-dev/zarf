// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helm contains operations for working with helm charts.
package helm

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/internal/packager/template"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"helm.sh/helm/v3/pkg/releaseutil"
)

type devRenderer struct {
	*Helm
	values template.Values
}

func (h *Helm) newDevRenderer() (*devRenderer, error) {
	values, err := template.Generate(h.cfg)
	if err != nil {
		return nil, err
	}
	values.SetState(&types.ZarfState{})
	values.SetRegistry("ZARF_REGISTRY")

	return &devRenderer{
		Helm:   h,
		values: *values,
	}, nil
}

func (r *devRenderer) Run(renderedManifests *bytes.Buffer) (*bytes.Buffer, error) {
	// This is very low cost and consistent for how we replace elsewhere, also good for debugging
	tempDir, err := utils.MakeTempDir(r.chartPath)
	if err != nil {
		return nil, fmt.Errorf("unable to create tmpdir:  %w", err)
	}
	path := filepath.Join(tempDir, "chart.yaml")

	// Write the context to a file for processing
	if err := utils.WriteFile(path, renderedManifests.Bytes()); err != nil {
		return nil, fmt.Errorf("unable to write the post-render file for the helm chart")
	}

	// Run the template engine against the chart output
	if _, err := template.ProcessYamlFilesInPath(tempDir, r.component, r.values); err != nil {
		return nil, fmt.Errorf("error templating the helm chart: %w", err)
	}

	// Read back the templated file contents
	buff, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading temporary post-rendered helm chart: %w", err)
	}

	// Use helm to re-split the manifest byte (same call used by helm to pass this data to postRender)
	_, resources, err := releaseutil.SortManifests(map[string]string{path: string(buff)},
		r.actionConfig.Capabilities.APIVersions,
		releaseutil.InstallOrder,
	)

	if err != nil {
		return nil, fmt.Errorf("error re-rendering helm output: %w", err)
	}

	finalManifestsOutput := bytes.NewBuffer(nil)

	// Otherwise, loop over the resources,
	for _, resource := range resources {
		fmt.Fprintf(finalManifestsOutput, "---\n# Source: %s\n%s\n", resource.Name, resource.Content)
	}

	// Send the bytes back to helm
	return finalManifestsOutput, nil
}
