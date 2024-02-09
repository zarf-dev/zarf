// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helpers provides generic helper functions with no external imports
package helpers

const (
	// ReadWriteUser is used for any internal file not generally consumed by the end user
	// or any file with senstive data
	ReadWriteUser = 0600
	// WriteUserReadAll is used for any non sensitive file intended to be consumed by the end user
	WriteUserReadAll = 0644
)
