// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package assemble

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"

	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/packager/load"
	"github.com/zarf-dev/zarf/src/pkg/utils"
)

// hydrateRemoteResources fetches imported component resource layers into an assembly-only workspace.
// The content-addressed copy is retained in the Zarf cache so repeated assemblies do not re-fetch blobs.
func hydrateRemoteResources(ctx context.Context, resources []load.RemoteResource, cachePath string) (string, error) {
	if len(resources) == 0 {
		return "", nil
	}
	// FIXME: does this still work if there is no cache?
	cachePath, err := utils.ResolveCachePath(cachePath)
	if err != nil {
		return "", err
	}
	workspace, err := utils.MakeTempDir("")
	if err != nil {
		return "", err
	}
	for _, resource := range resources {
		if !validHydrationPath(resource.ImportRoot) || !validHydrationPath(resource.MountPath) {
			return "", fmt.Errorf("remote component has an invalid resource path")
		}
		blobPath := filepath.Join(cachePath, "component-blobs", resource.Descriptor.Digest.Algorithm().String(), resource.Descriptor.Digest.Encoded())
		contents, err := os.ReadFile(blobPath)
		if os.IsNotExist(err) {
			contents, err = resource.Remote.FetchLayer(ctx, resource.Descriptor)
			if err != nil {
				return "", err
			}
			if err := os.MkdirAll(filepath.Dir(blobPath), helpers.ReadWriteExecuteUser); err != nil {
				return "", err
			}
			if err := os.WriteFile(blobPath, contents, helpers.ReadWriteUser); err != nil {
				return "", err
			}
		} else if err != nil {
			return "", err
		}
		destination := filepath.Join(workspace, filepath.FromSlash(resource.ImportRoot), filepath.FromSlash(resource.MountPath))
		if err := os.MkdirAll(filepath.Dir(destination), helpers.ReadWriteExecuteUser); err != nil {
			return "", err
		}
		if err := os.WriteFile(destination, contents, helpers.ReadWriteUser); err != nil {
			return "", err
		}
	}
	return workspace, nil
}

func validHydrationPath(value string) bool {
	clean := path.Clean(value)
	return value != "" && !path.IsAbs(value) && clean == value && value != "." && !strings.HasPrefix(value, "../") && !strings.Contains(value, "/../")
}

// FIXME: should take a v1beta1 package, or rather a package definition, and only proceed if it's v1beta1
func rewriteRemoteComponentPaths(pkg *v1alpha1.ZarfPackage, importedSchemas []string, workspace string, resources []load.RemoteResource) []string {
	roots := make(map[string]struct{}, len(resources))
	for _, resource := range resources {
		roots[resource.ImportRoot] = struct{}{}
	}
	rewrite := func(value string) string {
		if value == "" || filepath.IsAbs(value) || helpers.IsURL(value) {
			return value
		}
		value = filepath.ToSlash(value)
		for root := range roots {
			if value == root || strings.HasPrefix(value, root+"/") {
				return filepath.Join(workspace, filepath.FromSlash(value))
			}
		}
		return value
	}

	for i := range pkg.Values.Files {
		pkg.Values.Files[i] = rewrite(pkg.Values.Files[i])
	}
	pkg.Values.Schema = rewrite(pkg.Values.Schema)
	for i := range importedSchemas {
		importedSchemas[i] = rewrite(importedSchemas[i])
	}
	for i := range pkg.Components {
		component := &pkg.Components[i]
		for j := range component.Files {
			component.Files[j].Source = rewrite(component.Files[j].Source)
		}
		for j := range component.Charts {
			chart := &component.Charts[j]
			chart.LocalPath = rewrite(chart.LocalPath)
			for k := range chart.ValuesFiles {
				chart.ValuesFiles[k] = rewrite(chart.ValuesFiles[k])
			}
			for k := range chart.TemplatedValuesFiles {
				chart.TemplatedValuesFiles[k] = rewrite(chart.TemplatedValuesFiles[k])
			}
		}
		for j := range component.Manifests {
			manifest := &component.Manifests[j]
			for k := range manifest.Files {
				manifest.Files[k] = rewrite(manifest.Files[k])
			}
			for k := range manifest.Kustomizations {
				manifest.Kustomizations[k] = rewrite(manifest.Kustomizations[k])
			}
		}
	}
	return importedSchemas
}
