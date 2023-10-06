// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package test provides e2e tests for Zarf.
package test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/defenseunicorns/zarf/src/pkg/layout"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/mholt/archiver/v3"
	"github.com/stretchr/testify/require"
)

func TestCosignLookup(t *testing.T) {
	t.Log("E2E: Cosign lookup")
	tmpdir := t.TempDir()

	var (
		createPath  = "src/test/packages/10-cosign-lookup"
		packageName = fmt.Sprintf("zarf-package-cosign-lookup-%s.tar.zst", e2e.Arch)
	)

	e2e.CleanFiles(packageName)

	// Create the package
	stdOut, stdErr, err := e2e.Zarf("package", "create", createPath, "--confirm")
	require.NoError(t, err, stdOut, stdErr)

	// Extract the yaml of the differential package
	err = archiver.Extract(packageName, layout.ZarfYAML, tmpdir)
	require.NoError(t, err, "unable to extract zarf.yaml from the package")

	// Load the extracted zarf.yaml specification
	var zarfConfig types.ZarfPackage
	err = utils.ReadYaml(filepath.Join(tmpdir, layout.ZarfYAML), &zarfConfig)
	require.NoError(t, err, "unable to read zarf.yaml from the package")

	// Get a list of all images and repos that are inside of the differential package
	actualImages := []string{}
	for _, component := range zarfConfig.Components {
		actualImages = append(actualImages, component.Images...)
	}

	expectedImages := []string{
		"ghcr.io/defenseunicorns/zarf/agent:v0.30.0",
		"ghcr.io/defenseunicorns/zarf/agent:sha256-90863c246da361499e8f59dfae728c34b50ee2057e61e06dacdcfad983425c32.sig",
	}

	require.Len(t, actualImages, 2, "zarf.yaml from the package does not contain the expected number of images")
	for _, expectedImage := range expectedImages {
		require.Contains(t, actualImages, expectedImage, fmt.Sprintf("unable to find expected image %s", expectedImage))
	}

	e2e.CleanFiles(packageName)
}
