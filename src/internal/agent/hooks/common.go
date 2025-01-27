// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hooks contains the mutation hooks for the Zarf agent.
package hooks

import "github.com/zarf-dev/zarf/src/internal/agent/operations"

const annotationDisableCRC32 = "zarf.dev/remove-checksum"

func getLabelPatch(currLabels map[string]string) operations.PatchOperation {
	if currLabels == nil {
		currLabels = make(map[string]string)
	}
	currLabels["zarf-agent"] = "patched"
	return operations.ReplacePatchOperation("/metadata/labels", currLabels)
}

func hasRemoveChecksumAnnotation(annotations map[string]string) bool {
	if val, ok := annotations[annotationDisableCRC32]; ok {
		return val == "enable"
	}
	return false
}
