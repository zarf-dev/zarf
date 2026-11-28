// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package operations provides json patch operations.
package operations

const (
	addOperation     = "add"
	replaceOperation = "replace"
)

// PatchOperation is an operation of a JSON patch https://tools.ietf.org/html/rfc6902.
type PatchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

// AddPatchOperation returns an add JSON patch operation.
func AddPatchOperation(path string, value interface{}) PatchOperation {
	return PatchOperation{
		Op:    addOperation,
		Path:  path,
		Value: value,
	}
}

// ReplacePatchOperation returns a replace JSON patch operation.
func ReplacePatchOperation(path string, value interface{}) PatchOperation {
	return PatchOperation{
		Op:    replaceOperation,
		Path:  path,
		Value: value,
	}
}
