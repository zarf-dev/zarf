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

package properties

// Mirror represents a registry mirror endpoint.
type Mirror struct {
	// Location is the mirror registry host (e.g., "mirror.gcr.io").
	Location string

	// Transport contains transport settings for the mirror.
	Transport Transport

	// PullFromMirror controls when to use this mirror:
	// "all", "digest-only", "tag-only", or "" (defaults to "all").
	PullFromMirror string
}
