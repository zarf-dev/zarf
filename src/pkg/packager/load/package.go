// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mholt/archives"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/api"
	"github.com/zarf-dev/zarf/src/config"
	"github.com/zarf-dev/zarf/src/pkg/archive"
	"github.com/zarf-dev/zarf/src/pkg/packager/layout"
	"github.com/zarf-dev/zarf/src/pkg/utils"
	"github.com/zarf-dev/zarf/src/pkg/value"
	"oras.land/oras-go/v2/content"
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
		if !validResourcePath(resource.importRoot) {
			return nil, fmt.Errorf("remote component has an invalid resource path")
		}
		if _, exists := resourceSet.remoteRoots[resource.importRoot]; exists {
			continue
		}
		resourceSet.remoteRoots[resource.importRoot] = struct{}{}
		destination := filepath.Join(workspace, filepath.FromSlash(resource.importRoot))
		if err := materializeComponentResources(ctx, resource, destination); err != nil {
			return nil, err
		}
	}
	return resourceSet, nil
}

func materializeComponentResources(ctx context.Context, resource remoteResource, destination string) (err error) {
	reader, err := resource.remote.Fetch(ctx, resource.descriptor)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, reader.Close()) }()
	verified := content.NewVerifyReader(reader, resource.descriptor)
	if err := archive.DecompressStream(ctx, verified, destination, archive.DecompressOpts{Extractor: archives.Tar{}}); err != nil {
		return fmt.Errorf("extracting remote component archive: %w", err)
	}
	// Tar readers stop at the end markers; drain the remaining bytes to verify the OCI digest.
	if _, err := io.Copy(io.Discard, verified); err != nil {
		return err
	}
	if err := verified.Verify(); err != nil {
		return err
	}
	root, err := os.OpenRoot(destination)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	for _, descriptor := range resource.imageLayers {
		if err := materializeComponentImageLayer(ctx, root, resource, descriptor); err != nil {
			return err
		}
	}
	mountPaths := map[string]struct{}{}
	err = filepath.WalkDir(destination, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(destination, filePath)
		if err != nil {
			return err
		}
		mountPaths[filepath.ToSlash(rel)] = struct{}{}
		return nil
	})
	if err != nil {
		return err
	}
	return validateRemoteComponentResources(resource.requiredPaths, mountPaths)
}

func materializeComponentImageLayer(ctx context.Context, root *os.Root, resource remoteResource, descriptor ocispec.Descriptor) (err error) {
	mountPath := descriptor.Annotations[layout.ComponentResourceMountPathAnnotation]
	if err := root.MkdirAll(filepath.Dir(filepath.FromSlash(mountPath)), 0o700); err != nil {
		return err
	}
	file, err := root.OpenFile(filepath.FromSlash(mountPath), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("creating image layout file %q: %w", mountPath, err)
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	reader, err := resource.remote.Fetch(ctx, descriptor)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, reader.Close()) }()
	verified := content.NewVerifyReader(reader, descriptor)
	if _, err := io.Copy(file, verified); err != nil {
		return err
	}
	return verified.Verify()
}

func validResourcePath(value string) bool {
	return value != "." && fs.ValidPath(value) && !strings.ContainsAny(value, `\:`)
}
