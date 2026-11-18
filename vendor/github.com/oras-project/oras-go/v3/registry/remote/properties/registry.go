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

// Registry contains configuration for connecting to a remote registry.
type Registry struct {
	// Reference contains the parsed registry and repository reference.
	Reference Reference

	// Source holds the registry and repository as they were before the
	// registries.conf location rewrite (after alias resolution). It is empty
	// when no rewrite changed the base, and never carries a tag or digest.
	Source Reference

	// Transport is properties describing the communication layer.
	Transport Transport

	// Credential used for authentication.
	Credential Credential

	// Attributes contains registry-specific attributes.
	Attributes Attributes

	// Mirrors is an ordered list of mirror endpoints for this registry.
	Mirrors []Mirror
}

// NewRegistry creates a new Registry property from a reference string.
// The reference string should be in the format: registry/repository[:tag][@digest]
func NewRegistry(reference string) (*Registry, error) {
	ref, err := NewReference(reference)
	if err != nil {
		return nil, err
	}
	return &Registry{
		Reference: ref,
		Transport: Transport{
			HeaderFlags: make(map[string]string),
		},
	}, nil
}

// NewRegistryFromReference creates a new Registry property from a Reference.
func NewRegistryFromReference(ref Reference) *Registry {
	return &Registry{
		Reference: ref,
		Transport: Transport{
			HeaderFlags: make(map[string]string),
		},
	}
}
