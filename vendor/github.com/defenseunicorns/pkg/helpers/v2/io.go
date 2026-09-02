// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2024-Present Defense Unicorns

package helpers

const (
	// ReadUser is used for any internal file to be read only
	ReadUser = 0400
	// ReadWriteUser is used for any internal file not normally used by the end user or containing sensitive data
	ReadWriteUser = 0600
	// ReadAllWriteUser is used for any non sensitive file intended to be consumed by the end user
	ReadAllWriteUser = 0644
	// ReadWriteExecuteUser is used for any directory or executable not normally used by the end user or containing sensitive data
	ReadWriteExecuteUser = 0700
	// ReadExecuteAllWriteUser is used for any non sensitive directory or executable intended to be consumed by the end user
	ReadExecuteAllWriteUser = 0755
)
