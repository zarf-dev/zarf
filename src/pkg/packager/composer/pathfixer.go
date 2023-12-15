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

func fixPaths(child *types.ZarfComponent, relativeToHeadOrURL string) {
	for fileIdx, file := range child.Files {
		composed := makePathRelativeTo(file.Source, relativeToHeadOrURL)
		child.Files[fileIdx].Source = composed
	}

	for chartIdx, chart := range child.Charts {
		for valuesIdx, valuesFile := range chart.ValuesFiles {
			composed := makePathRelativeTo(valuesFile, relativeToHeadOrURL)
			child.Charts[chartIdx].ValuesFiles[valuesIdx] = composed
		}
		if child.Charts[chartIdx].LocalPath != "" {
			composed := makePathRelativeTo(chart.LocalPath, relativeToHeadOrURL)
			child.Charts[chartIdx].LocalPath = composed
		}
	}

	for manifestIdx, manifest := range child.Manifests {
		for fileIdx, file := range manifest.Files {
			composed := makePathRelativeTo(file, relativeToHeadOrURL)
			child.Manifests[manifestIdx].Files[fileIdx] = composed
		}
		for kustomizeIdx, kustomization := range manifest.Kustomizations {
			composed := makePathRelativeTo(kustomization, relativeToHeadOrURL)
			// kustomizations can use non-standard urls, so we need to check if the composed path exists on the local filesystem
			abs, _ := filepath.Abs(composed)
			invalid := utils.InvalidPath(abs)
			if !invalid {
				child.Manifests[manifestIdx].Kustomizations[kustomizeIdx] = composed
			}
		}
	}

	for dataInjectionsIdx, dataInjection := range child.DataInjections {
		composed := makePathRelativeTo(dataInjection.Source, relativeToHeadOrURL)
		child.DataInjections[dataInjectionsIdx].Source = composed
	}

	defaultDir := child.Actions.OnCreate.Defaults.Dir
	child.Actions.OnCreate.Before = fixActionPaths(child.Actions.OnCreate.Before, defaultDir, relativeToHeadOrURL)
	child.Actions.OnCreate.After = fixActionPaths(child.Actions.OnCreate.After, defaultDir, relativeToHeadOrURL)
	child.Actions.OnCreate.OnFailure = fixActionPaths(child.Actions.OnCreate.OnFailure, defaultDir, relativeToHeadOrURL)
	child.Actions.OnCreate.OnSuccess = fixActionPaths(child.Actions.OnCreate.OnSuccess, defaultDir, relativeToHeadOrURL)

	// deprecated
	if child.DeprecatedCosignKeyPath != "" {
		composed := makePathRelativeTo(child.DeprecatedCosignKeyPath, relativeToHeadOrURL)
		child.DeprecatedCosignKeyPath = composed
	}
}

// fixActionPaths takes a slice of actions and mutates the Dir to be relative to the head node
func fixActionPaths(actions []types.ZarfComponentAction, defaultDir, relativeToHeadOrURL string) []types.ZarfComponentAction {
	for actionIdx, action := range actions {
		var composed string
		if action.Dir != nil {
			composed = makePathRelativeTo(*action.Dir, relativeToHeadOrURL)
		} else {
			composed = makePathRelativeTo(defaultDir, relativeToHeadOrURL)
		}
		actions[actionIdx].Dir = &composed
	}
	return actions
}
