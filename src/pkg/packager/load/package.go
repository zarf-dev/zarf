// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/zarf-dev/zarf/src/pkg/helpers"

	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/value"
)

// PackageOptions configures resource-ready package loading.
type PackageOptions struct {
	DefinitionOptions
	// SkipValuesSchemaValidation skips validating merged values against the
	// resolved schema. Schema parsing and merging still occur.
	SkipValuesSchemaValidation bool
}

// ResolvedPackage is a package definition together with its source
// resources and package values. Call Close when resource access is no
// longer needed.
type ResolvedPackage struct {
	Definition   api.Package
	Resources    *ResourceSet
	Values       value.Values
	ValuesSchema value.SchemaDocument
}

// Close removes temporary resources materialized while loading the package.
func (p *ResolvedPackage) Close() error {
	if p == nil || p.Resources == nil {
		return nil
	}
	return p.Resources.Close()
}

// ResourceSet maps logical paths in a package definition to filesystem paths.
// It keeps temporary imported-component resources private to the package load.
type ResourceSet struct {
	packageRoot string
	workspace   string
	remoteRoots map[string]struct{}

	closed bool
}

// NewResourceSet creates a resource set rooted at a local component or package directory.
func NewResourceSet(root string) *ResourceSet {
	return &ResourceSet{
		packageRoot: root,
		remoteRoots: map[string]struct{}{},
	}
}

// Root returns the original package source directory.
func (r *ResourceSet) Root() (string, error) {
	if r.closed {
		return "", errors.New("package resources are closed")
	}
	return r.packageRoot, nil
}

// Path returns the physical path for a local or materialized remote resource.
// URLs are intentionally not resource-set paths and must be handled by their
// owning resource type.
func (r *ResourceSet) Path(logicalPath string) (string, error) {
	if r.closed {
		return "", errors.New("package resources are closed")
	}
	if filepath.IsAbs(logicalPath) {
		return logicalPath, nil
	}
	logicalPath = filepath.ToSlash(logicalPath)
	for root := range r.remoteRoots {
		if logicalPath == root || strings.HasPrefix(logicalPath, root+"/") {
			physical := filepath.Join(r.workspace, filepath.FromSlash(logicalPath))
			if _, err := os.Stat(physical); err != nil {
				return "", fmt.Errorf("unable to access remote resource %q: %w", logicalPath, err)
			}
			return physical, nil
		}
	}
	physical := filepath.Join(r.packageRoot, filepath.FromSlash(logicalPath))
	if _, err := os.Stat(physical); err != nil {
		return "", fmt.Errorf("unable to access local resource %q: %w", logicalPath, err)
	}
	return physical, nil
}

// ReadFile reads an exact resource file.
func (r *ResourceSet) ReadFile(logicalPath string) ([]byte, error) {
	physical, err := r.Path(logicalPath)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(physical)
}

// Close removes temporary remote-component resources.
func (r *ResourceSet) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	if r.workspace == "" {
		return nil
	}
	return os.RemoveAll(r.workspace)
}

// Package resolves a package definition and makes imported component resources
// available without assembling a Zarf package.
func Package(ctx context.Context, packagePath string, opts PackageOptions) (_ *ResolvedPackage, err error) {
	resolved, err := resolve(ctx, packagePath, opts.DefinitionOptions)
	if err != nil {
		return nil, err
	}

	resources, err := materializeResources(ctx, resolved.packageRoot, resolved.remoteResources)
	if err != nil {
		return nil, err
	}
	loaded := &ResolvedPackage{
		Definition: resolved.definition,
		Resources:  resources,
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, loaded.Close())
		}
	}()

	loaded.Values, loaded.ValuesSchema, err = loadValues(ctx, resources, resolved.values, opts.SkipValuesSchemaValidation)
	if err != nil {
		return nil, err
	}

	return loaded, nil
}

func materializeResources(ctx context.Context, packageRoot string, remoteResources []remoteResource) (_ *ResourceSet, err error) {
	resourceSet := NewResourceSet(packageRoot)
	if len(remoteResources) == 0 {
		return resourceSet, nil
	}
	workspace, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, err
	}
	resourceSet.workspace = workspace
	defer func() {
		if err != nil {
			err = errors.Join(err, resourceSet.Close())
		}
	}()

	for _, resource := range remoteResources {
		if !validResourcePath(resource.importRoot) || !validResourcePath(resource.mountPath) {
			return nil, fmt.Errorf("remote component has an invalid resource path")
		}
		resourceSet.remoteRoots[resource.importRoot] = struct{}{}
		destination := filepath.Join(workspace, filepath.FromSlash(resource.importRoot), filepath.FromSlash(resource.mountPath))
		if err := os.MkdirAll(filepath.Dir(destination), helpers.ReadWriteExecuteUser); err != nil {
			return nil, err
		}
		contents, err := resource.remote.FetchLayer(ctx, resource.descriptor)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(destination, contents, helpers.ReadWriteUser); err != nil {
			return nil, err
		}
	}
	return resourceSet, nil
}

func validResourcePath(value string) bool {
	return value != "." && fs.ValidPath(value) && !strings.ContainsAny(value, `\:`)
}
