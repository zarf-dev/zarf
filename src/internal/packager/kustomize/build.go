// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package kustomize provides functions for building kustomizations.
package kustomize

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"sigs.k8s.io/kustomize/api/krusty"
	krustytypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

// Fetch creates a kustomization file for a git repository.
func Fetch(gitRef string, directory string) error {
	k := krustytypes.Kustomization{
		Resources: []string{
			gitRef,
		},
	}
	err := os.MkdirAll(directory, 0600)
	if err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(directory, "kustomization.yaml"))
	if err != nil {
		return err
	}
	defer file.Close()
	b := []byte{}
	err = k.Unmarshal(b)
	if err != nil {
		return err
	}
	_, err = file.Write(b)
	if err != nil {
		return err
	}
	return nil
}

// Build reads a kustomization and builds it into a single yaml file.
func Build(path string, destination string, kustomizeAllowAnyDirectory bool) error {
	// Kustomize has to write to the filesystem on-disk
	fSys := filesys.MakeFsOnDisk()

	// flux2 build options for consistency, load restrictions none applies only to local files
	buildOptions := krusty.MakeDefaultOptions()

	if kustomizeAllowAnyDirectory {
		buildOptions.LoadRestrictions = krustytypes.LoadRestrictionsNone
	}

	kustomizer := krusty.MakeKustomizer(buildOptions)

	// Try to build the kustomization
	resources, err := kustomizer.Run(fSys, path)
	if err != nil {
		return err
	}

	yaml, err := resources.AsYaml()

	if err != nil {
		return fmt.Errorf("problem converting kustomization to yaml: %w", err)
	}

	return utils.WriteFile(destination, yaml)
}
