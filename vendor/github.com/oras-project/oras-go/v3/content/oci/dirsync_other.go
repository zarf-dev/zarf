//go:build !windows

/*
Copyright The ORAS Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package oci

import "os"

// syncDir flushes the directory entries of dir to stable storage, so that a
// file that has been renamed within dir is not lost on a crash.
//
// The operation is best-effort and reports nothing: its callers reach it only
// once a rename has already taken effect, so a failure here describes an
// operation that did happen and that cannot be undone. Directory
// synchronization is also unsupported by some file systems, which report it in
// ways that vary between them.
func syncDir(dir string) {
	dirFile, err := os.Open(dir)
	if err != nil {
		return
	}
	defer dirFile.Close()
	_ = dirFile.Sync()
}
