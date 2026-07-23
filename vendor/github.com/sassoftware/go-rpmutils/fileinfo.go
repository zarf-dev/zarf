/*
 * Copyright (c) SAS Institute, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package rpmutils

// FileInfo describes a file in the RPM payload
type FileInfo interface {
	Name() string
	Size() int64
	UserName() string
	GroupName() string
	Flags() int
	Mtime() int
	Digest() string
	Mode() int
	Linkname() string
	Device() int
	Inode() int
}

type fileInfo struct {
	name      string
	size      uint64
	userName  string
	groupName string
	flags     uint32
	mtime     uint32
	digest    string
	mode      uint32
	linkName  string
	device    uint32
	inode     uint32
}

// Name returns the full path of the file
func (fi *fileInfo) Name() string {
	return fi.name
}

// Size of the file in bytes
func (fi *fileInfo) Size() int64 {
	return int64(fi.size)
}

// UserName returns the file's owner user name
func (fi *fileInfo) UserName() string {
	return fi.userName
}

// GroupName returns the file's owner group name
func (fi *fileInfo) GroupName() string {
	return fi.groupName
}

// Flags returns RPM-specific file flags (config, ghost etc.)
func (fi *fileInfo) Flags() int {
	return int(fi.flags)
}

// Mtime returns the modification time of the file as a UNIX epoch time
func (fi *fileInfo) Mtime() int {
	return int(fi.mtime)
}

// Digest of the file, according to FILEDIGESTALGO
func (fi *fileInfo) Digest() string {
	return fi.digest
}

// Mode of the file, holding both permissions and file type
func (fi *fileInfo) Mode() int {
	return int(fi.mode)
}

// Linkname returns the target of a symlink
func (fi *fileInfo) Linkname() string {
	return fi.linkName
}

// Device returns the major and minor device number of a character or block device
func (fi *fileInfo) Device() int {
	return int(fi.device)
}

// Inode returns a inode number used to tie hardlinked files together
func (fi *fileInfo) Inode() int {
	return int(fi.inode)
}

func (fi *fileInfo) fileType() uint32 {
	return fi.mode &^ 07777
}

func (fi *fileInfo) inode64() uint64 {
	return (uint64(fi.device) << 32) | uint64(fi.inode)
}
