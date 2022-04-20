package kustomize

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/internal/utils"
	"sigs.k8s.io/kustomize/api/krusty"	
	kustypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

// BuildKustomization reads a kustomization and builds it into a single yaml file
func BuildKustomization(path string, destination string) error {
	// Kustomize has to write to the filesystem on-disk
	fSys := filesys.MakeFsOnDisk()

	// flux2 options for consistency
	buildOptions := &krusty.Options{
		DoLegacyResourceSort: true,
		LoadRestrictions:     kustypes.LoadRestrictionsNone,
		AddManagedbyLabel:    false,
		DoPrune:              false,
		PluginConfig:         kustypes.DisabledPluginConfig(),
	}

	kustomizer := krusty.MakeKustomizer(buildOptions)

	// Try to build the kustomization
	resources, err := kustomizer.Run(fSys, path)
	if err != nil {
		return err
	}

	if yaml, err := resources.AsYaml(); err != nil {
		return fmt.Errorf("problem converting kustomization to yaml: %w", err)
	} else {
		return utils.WriteFile(destination, yaml)
	}
}
