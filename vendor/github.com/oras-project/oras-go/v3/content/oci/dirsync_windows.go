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

// syncDir does nothing on Windows, where flushing a directory handle is not
// supported. The rename that its callers perform still replaces the target in
// a single operation there, although the Go API does not promise the same
// atomicity that it does on Unix; only the additional durability against a
// crash is unavailable.
func syncDir(dir string) {}
