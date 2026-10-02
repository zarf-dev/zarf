// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package load

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/zarf-dev/zarf/src/api/v1beta1"
	internalv1beta1 "github.com/zarf-dev/zarf/src/internal/api/v1beta1"
	"github.com/zarf-dev/zarf/src/pkg/value"
	"github.com/zarf-dev/zarf/src/types"
)

// ResolvedComponent is a component config with its imports, resources, and values loaded.
// Call Close when resource access is no longer needed.
type ResolvedComponent struct {
	Definition   v1beta1.ComponentConfig
	Resources    *ResourceSet
	Values       value.Values
	ValuesSchema value.SchemaDocument
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
	if validationErrs := internalv1beta1.ValidateComponent(v1beta1.Component{
		Name:          resolved.Component.Metadata.Name,
		ComponentSpec: resolved.Component.Component,
	}); len(validationErrs) > 0 {
		return nil, fmt.Errorf("component validation failed:\n%w", validationErrs)
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

	plan := valuePlan{
		files:   loaded.Definition.Values.Files,
		schemas: schemaSources(loaded.Definition.Values.Schema, resolved.ImportedSchemas),
	}
	loaded.Values, loaded.ValuesSchema, err = loadValues(ctx, resources, plan, false)
	if err != nil {
		return nil, err
	}
	return loaded, nil
}
