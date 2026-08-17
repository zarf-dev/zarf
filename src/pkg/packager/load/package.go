// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"

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
	Definition   api.PackageDefinition
	Resources    *ResourceSet
	Values       value.Values
	ValuesSchema value.SchemaDocument
}

// Close removes temporary resources materialized while loading the package.
func (p *ResolvedPackage) Close() error {
	if p == nil || p.Resources == nil {
		return nil
	}
	return p.Resources.close()
}

// ResourceSet maps logical paths in a package definition to filesystem paths.
// It keeps temporary imported-component resources private to the package load.
type ResourceSet struct {
	packageRoot string
	workspace   string
	remoteRoots map[string]struct{}

	closed bool
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

func (r *ResourceSet) close() error {
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

	valuesPaths := make([]string, 0, len(resolved.values.files))
	for _, source := range resolved.values.files {
		physical, err := resources.Path(source)
		if err != nil {
			return nil, err
		}
		valuesPaths = append(valuesPaths, physical)
	}
	if len(valuesPaths) > 0 {
		loaded.Values, err = value.ParseFiles(ctx, valuesPaths, value.ParseFilesOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to parse package values files: %w", err)
		}
	} else {
		loaded.Values = value.Values{}
	}

	schemas := make([]value.SchemaDocument, 0, len(resolved.values.schemas))
	for _, source := range resolved.values.schemas {
		contents, err := resources.ReadFile(source)
		if err != nil {
			return nil, fmt.Errorf("reading values schema %q: %w", source, err)
		}
		schema, err := value.ParseSchemaDocument(source, contents)
		if err != nil {
			return nil, err
		}
		schemas = append(schemas, schema)
	}
	loaded.ValuesSchema, err = value.MergeSchemaDocuments(schemas)
	if err != nil {
		return nil, err
	}
	if loaded.ValuesSchema != nil && !opts.SkipValuesSchemaValidation {
		if err := loaded.Values.ValidateAgainstSchema(ctx, loaded.ValuesSchema, "resolved package values schema", value.ValidateOptions{SkipRequired: true}); err != nil {
			return nil, fmt.Errorf("values validation failed: %w", err)
		}
	}

	return loaded, nil
}

func materializeResources(ctx context.Context, packageRoot string, remoteResources []remoteResource) (*ResourceSet, error) {
	resourceSet := &ResourceSet{
		packageRoot: packageRoot,
		remoteRoots: map[string]struct{}{},
	}
	if len(remoteResources) == 0 {
		return resourceSet, nil
	}
	workspace, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
	if err != nil {
		return nil, err
	}
	resourceSet.workspace = workspace
	fail := func(cause error) (*ResourceSet, error) {
		return nil, errors.Join(cause, resourceSet.close())
	}
	for _, resource := range remoteResources {
		if !validResourcePath(resource.importRoot) || !validResourcePath(resource.mountPath) {
			return fail(fmt.Errorf("remote component has an invalid resource path"))
		}
		resourceSet.remoteRoots[resource.importRoot] = struct{}{}
		destination := filepath.Join(workspace, filepath.FromSlash(resource.importRoot), filepath.FromSlash(resource.mountPath))
		if err := os.MkdirAll(filepath.Dir(destination), helpers.ReadWriteExecuteUser); err != nil {
			return fail(err)
		}
		contents, err := resource.remote.FetchLayer(ctx, resource.descriptor)
		if err != nil {
			return fail(err)
		}
		if err := os.WriteFile(destination, contents, helpers.ReadWriteUser); err != nil {
			return fail(err)
		}
	}
	return resourceSet, nil
}

func validResourcePath(value string) bool {
	clean := path.Clean(value)
	return value != "" && !path.IsAbs(value) && clean == value && value != "." && !strings.HasPrefix(value, "../") && !strings.Contains(value, "/../")
}
