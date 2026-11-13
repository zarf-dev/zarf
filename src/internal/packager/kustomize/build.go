// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package kustomize provides functions for building kustomizations.
package kustomize

import (
	"fmt"
	"os"
	"sync"

	"github.com/zarf-dev/zarf/src/pkg/helpers"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	krustytypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

// krustyMu serializes krusty runs. krusty sets the process-global kyaml OpenAPI schema on every Run, which is
// not safe for concurrent use (kubernetes-sigs/kustomize#4824); Flux guards its kustomize builds the same way.
var krustyMu sync.Mutex

// RunKustomizer builds the kustomization at path in fSys with opts. Every krusty run in Zarf must go through it.
// krusty panics on some invalid inputs (e.g. resources renamed into duplicate IDs by a patch); those panics are
// returned as errors. The recovered panic text may contain templated (possibly secret) content; that is only safe
// because the post-renderers redact Kustomize errors whenever templating may have touched their input.
func RunKustomizer(fSys filesys.FileSystem, path string, opts *krusty.Options) (_ resmap.ResMap, err error) {
	krustyMu.Lock()
	defer krustyMu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered from kustomize build panic: %v", r)
		}
	}()
	return krusty.MakeKustomizer(opts).Run(fSys, path)
}

// Build reads a kustomization and builds it into a single yaml file.
func Build(path string, destination string, kustomizeAllowAnyDirectory bool, enableKustomizePlugins bool) error {
	// Kustomize has to write to the filesystem on-disk
	fSys := filesys.MakeFsOnDisk()

	// flux2 build options for consistency, load restrictions none applies only to local files
	buildOptions := krusty.MakeDefaultOptions()

	if kustomizeAllowAnyDirectory {
		buildOptions.LoadRestrictions = krustytypes.LoadRestrictionsNone
	}

	if enableKustomizePlugins {
		buildOptions.PluginConfig = krustytypes.MakePluginConfig(krustytypes.PluginRestrictionsNone, krustytypes.BploUseStaticallyLinked)
	}

	// Try to build the kustomization
	resources, err := RunKustomizer(fSys, path, buildOptions)
	if err != nil {
		return err
	}

	yaml, err := resources.AsYaml()

	if err != nil {
		return fmt.Errorf("problem converting kustomization to yaml: %w", err)
	}

	return os.WriteFile(destination, yaml, helpers.ReadWriteUser)
}
