// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hooks contains the mutation hooks for the Zarf agent.
package hooks

import "github.com/defenseunicorns/zarf/src/internal/agent/operations"

func initialPatchMap() map[string]string {
	return map[string]string{
		"zarf-agent": "patched",
	}
}

func addPatchedAnnotation(patches []operations.PatchOperation, currAnnotations map[string]string) []operations.PatchOperation {
	if currAnnotations == nil {
		return append(patches, operations.ReplacePatchOperation("/metadata/annotations", initialPatchMap()))
	}
	return append(patches, operations.ReplacePatchOperation("/metadata/annotations/zarf-agent", "patched"))
}

func addPatchedLabel(patches []operations.PatchOperation, currLabels map[string]string) []operations.PatchOperation {
	if currLabels == nil {
		return append(patches, operations.ReplacePatchOperation("/metadata/labels", initialPatchMap()))
	}
	return append(patches, operations.ReplacePatchOperation("/metadata/labels/zarf-agent", "patched"))
}
