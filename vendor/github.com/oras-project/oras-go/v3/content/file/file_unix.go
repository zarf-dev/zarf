//go:build unix

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

package file

import (
	"os"
	"syscall"
)

// createNoFollow creates or truncates the named file for writing with the
// given permission, failing instead of following the path if its final
// component is a symlink. This closes the window in which a symlink planted
// after a prior Lstat check would redirect the write.
// (GHSA-2hhj-vj64-hx3j)
func createNoFollow(path string, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|syscall.O_NOFOLLOW, perm)
}
