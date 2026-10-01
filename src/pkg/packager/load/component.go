// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/zarf-dev/zarf/src/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/value"
	"github.com/zarf-dev/zarf/src/types"
)

// ResolvedComponent is a component config with its imports, resources, and values loaded.
// Call Close when resource access is no longer needed.
type ResolvedComponent struct {
	Definition v1beta1.ComponentConfig
	Resources  *ResourceSet
	Values     value.Values
}

// ComponentOptions configures resource-ready component loading.
type ComponentOptions struct {
	// CachePath stores remote component layers locally when non-empty.
	CachePath string
	types.RemoteOptions
}

// Close removes temporary resources materialized while loading the component.
func (c *ResolvedComponent) Close() error {
	if c == nil || c.Resources == nil {
		return nil
	}
	return c.Resources.Close()
}

// Component loads a v1beta1 component config and makes imported resources available.
func Component(ctx context.Context, componentPath string, opts ComponentOptions) (_ *ResolvedComponent, err error) {
	componentPath = filepath.Clean(componentPath)
	component, err := ComponentConfig(componentPath)
	if err != nil {
		return nil, err
	}
	resolved, err := ResolveComponentConfigImports(ctx, component, componentPath, opts.RemoteOptions, opts.CachePath)
	if err != nil {
		return nil, err
	}
	resources, err := resolved.MaterializeResources(ctx, componentPath)
	if err != nil {
		return nil, err
	}
	loaded := &ResolvedComponent{Definition: resolved.Component, Resources: resources}
	defer func() {
		if err != nil {
			err = errors.Join(err, loaded.Close())
		}
	}()

	valuesPaths := make([]string, 0, len(loaded.Definition.Values.Files))
	for _, valuePath := range loaded.Definition.Values.Files {
		physical, pathErr := resources.Path(valuePath)
		if pathErr != nil {
			return nil, pathErr
		}
		valuesPaths = append(valuesPaths, physical)
	}
	loaded.Values, err = value.ParseFiles(ctx, valuesPaths, value.ParseFilesOptions{})
	if err != nil {
		return nil, err
	}
	return loaded, nil
}
