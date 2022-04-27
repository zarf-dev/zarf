package kustomize

import (
	"fmt"

	"github.com/defenseunicorns/zarf/src/internal/utils"
	"sigs.k8s.io/kustomize/api/krusty"
	kustypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

// BuildKustomization reads a kustomization and builds it into a single yaml file.
func BuildKustomization(path string, destination string, kustomizeAllowAnyDirectory bool) error {
	// Kustomize has to write to the filesystem on-disk
	fSys := filesys.MakeFsOnDisk()

	// flux2 build options for consistency, load restrictions none applies only to local files
	buildOptions := krusty.MakeDefaultOptions()

	if kustomizeAllowAnyDirectory {
		buildOptions.LoadRestrictions = kustypes.LoadRestrictionsNone
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
