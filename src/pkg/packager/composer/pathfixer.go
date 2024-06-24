// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package composer contains functions for composing components within Zarf packages.
package composer

import (
	"path/filepath"

	"github.com/defenseunicorns/pkg/helpers/v2"
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
			invalid := helpers.InvalidPath(abs)
			if !invalid {
				child.Manifests[manifestIdx].Kustomizations[kustomizeIdx] = composed
			}
		}
	}

	for dataInjectionsIdx, dataInjection := range child.DataInjections {
		composed := makePathRelativeTo(dataInjection.Source, relativeToHead)
		child.DataInjections[dataInjectionsIdx].Source = composed
	}

	defaultDir := child.Actions.OnCreate.Defaults.Dir
	child.Actions.OnCreate.Before = fixActionPaths(child.Actions.OnCreate.Before, defaultDir, relativeToHead)
	child.Actions.OnCreate.After = fixActionPaths(child.Actions.OnCreate.After, defaultDir, relativeToHead)
	child.Actions.OnCreate.OnFailure = fixActionPaths(child.Actions.OnCreate.OnFailure, defaultDir, relativeToHead)
	child.Actions.OnCreate.OnSuccess = fixActionPaths(child.Actions.OnCreate.OnSuccess, defaultDir, relativeToHead)

	// deprecated
	if child.DeprecatedCosignKeyPath != "" {
		composed := makePathRelativeTo(child.DeprecatedCosignKeyPath, relativeToHead)
		child.DeprecatedCosignKeyPath = composed
	}
}

// fixActionPaths takes a slice of actions and mutates the Dir to be relative to the head node
func fixActionPaths(actions []types.ZarfComponentAction, defaultDir, relativeToHead string) []types.ZarfComponentAction {
	for actionIdx, action := range actions {
		var composed string
		if action.Dir != nil {
			composed = makePathRelativeTo(*action.Dir, relativeToHead)
		} else {
			composed = makePathRelativeTo(defaultDir, relativeToHead)
		}
		actions[actionIdx].Dir = &composed
	}
	return actions
}
