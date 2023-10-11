// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package composer contains functions for composing components within Zarf packages.
package composer

import (
	"path/filepath"

	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/pkg/utils/helpers"
	"github.com/defenseunicorns/zarf/src/types"
)

func makePathRelativeTo(path, relativeTo string) string {
	if helpers.IsURL(path) {
		return path
	}

	return filepath.Join(relativeTo, path)
}

func fixPaths(child *types.ZarfComponent, relativeToHead string) {
	for fileIdx, file := range child.Files {
		composed := makePathRelativeTo(file.Source, relativeToHead)
		child.Files[fileIdx].Source = composed
	}

	for chartIdx, chart := range child.Charts {
		for valuesIdx, valuesFile := range chart.ValuesFiles {
			composed := makePathRelativeTo(valuesFile, relativeToHead)
			child.Charts[chartIdx].ValuesFiles[valuesIdx] = composed
		}
		if child.Charts[chartIdx].LocalPath != "" {
			composed := makePathRelativeTo(chart.LocalPath, relativeToHead)
			child.Charts[chartIdx].LocalPath = composed
		}
	}

	for manifestIdx, manifest := range child.Manifests {
		for fileIdx, file := range manifest.Files {
			composed := makePathRelativeTo(file, relativeToHead)
			child.Manifests[manifestIdx].Files[fileIdx] = composed
		}
		for kustomizeIdx, kustomization := range manifest.Kustomizations {
			composed := makePathRelativeTo(kustomization, relativeToHead)
			// kustomizations can use non-standard urls, so we need to check if the composed path exists on the local filesystem
			abs, _ := filepath.Abs(composed)
			invalid := utils.InvalidPath(abs)
			if !invalid {
				child.Manifests[manifestIdx].Kustomizations[kustomizeIdx] = composed
			}
		}
	}

	for dataInjectionsIdx, dataInjection := range child.DataInjections {
		composed := makePathRelativeTo(dataInjection.Source, relativeToHead)
		child.DataInjections[dataInjectionsIdx].Source = composed
	}

	for actionIdx, action := range child.Actions.OnCreate.Before {
		if action.Dir != nil {
			composed := makePathRelativeTo(*action.Dir, relativeToHead)
			child.Actions.OnCreate.Before[actionIdx].Dir = &composed
		}
	}

	for actionIdx, action := range child.Actions.OnCreate.After {
		if action.Dir != nil {
			composed := makePathRelativeTo(*action.Dir, relativeToHead)
			child.Actions.OnCreate.After[actionIdx].Dir = &composed
		}
	}

	for actionIdx, action := range child.Actions.OnCreate.OnFailure {
		if action.Dir != nil {
			composed := makePathRelativeTo(*action.Dir, relativeToHead)
			child.Actions.OnCreate.OnFailure[actionIdx].Dir = &composed
		}
	}

	for actionIdx, action := range child.Actions.OnCreate.OnSuccess {
		if action.Dir != nil {
			composed := makePathRelativeTo(*action.Dir, relativeToHead)
			child.Actions.OnCreate.OnSuccess[actionIdx].Dir = &composed
		}
	}

	totalActions := len(child.Actions.OnCreate.Before) + len(child.Actions.OnCreate.After) + len(child.Actions.OnCreate.OnFailure) + len(child.Actions.OnCreate.OnSuccess)

	if totalActions > 0 {
		composedDefaultDir := makePathRelativeTo(child.Actions.OnCreate.Defaults.Dir, relativeToHead)
		child.Actions.OnCreate.Defaults.Dir = composedDefaultDir
	}

	// deprecated
	if child.DeprecatedCosignKeyPath != "" {
		composed := makePathRelativeTo(child.DeprecatedCosignKeyPath, relativeToHead)
		child.DeprecatedCosignKeyPath = composed
	}
}
