// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package operations provides json patch operations.
package operations

const (
	addOperation     = "add"
	removeOperation  = "remove"
	replaceOperation = "replace"
	copyOperation    = "copy"
	moveOperation    = "move"
)

// PatchOperation is an operation of a JSON patch https://tools.ietf.org/html/rfc6902.
type PatchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	From  string      `json:"from,omitempty"`
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

// RemovePatchOperation returns a remove JSON patch operation.
func RemovePatchOperation(path string) PatchOperation {
	return PatchOperation{
		Op:   removeOperation,
		Path: path,
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

// CopyPatchOperation returns a copy JSON patch operation.
func CopyPatchOperation(from, path string) PatchOperation {
	return PatchOperation{
		Op:   copyOperation,
		Path: path,
		From: from,
	}
}

// MovePatchOperation returns a move JSON patch operation.
func MovePatchOperation(from, path string) PatchOperation {
	return PatchOperation{
		Op:   moveOperation,
		Path: path,
		From: from,
	}
}
