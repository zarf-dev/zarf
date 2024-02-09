// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package helpers provides generic helper functions with no external imports
package helpers

const (
	// ReadWriteUser is used for any internal file not normallly used by the end user or containing senstive data
	ReadWriteUser = 0600
	// WriteUserReadAll is used for any non sensitive file intended to be consumed by the end user
	WriteUserReadAll = 0644

	// ReadWriteXUser is used for any directory or executable not normally used by the end user or containing sensitive data
	ReadWriteXUser = 0700

	// WriteUserReadXAll is used for any non sensitive directory or executable intended to be consumed by the end user
	WriteUserReadXAll = 0755
)
