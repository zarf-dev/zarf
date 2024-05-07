// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hooks contains the mutation hooks for the Zarf agent.
package hooks

import "github.com/defenseunicorns/zarf/src/internal/agent/operations"

func addPatchedAnnotation(patches []operations.PatchOperation, currAnnotations map[string]string) []operations.PatchOperation {
	if currAnnotations == nil {
		annotations := map[string]string{
			"zarf-agent": "patched",
		}
		patches = append(patches, operations.ReplacePatchOperation("/metadata/annotations", annotations))
	} else {
		patches = append(patches, operations.ReplacePatchOperation("/metadata/annotations/zarf-agent", "patched"))
	}
	return patches
}
