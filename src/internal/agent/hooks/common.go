// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hooks contains the mutation hooks for the Zarf agent.
package hooks

import (
	"github.com/defenseunicorns/zarf/src/internal/agent/operations"
)

func addAgentLabel(labels map[string]string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["zarf-agent"] = "patched"
	return labels
}

func getAnnotationPatch(currAnnotations map[string]string) operations.PatchOperation {
	return operations.ReplacePatchOperation("/metadata/annotations", addAgentLabel(currAnnotations))
}
