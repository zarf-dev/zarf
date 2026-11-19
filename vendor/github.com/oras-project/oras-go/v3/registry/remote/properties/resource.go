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

import (
	"fmt"
	"strings"

	"github.com/oras-project/oras-go/v3/errdef"
)

// Resource represents a registry host, optionally narrowed to a namespace or
// repository. It never carries a tag or digest.
type Resource struct {
	// Registry is the normalized name of the registry. The host portion is
	// lowercase because registry hostnames are case-insensitive; a port, if
	// present, is preserved.
	Registry string

	// Path is the namespace or repository path, or empty for the whole registry.
	Path string
}

// ParseResource parses a registry resource. The resource may contain an
// optional oci://, http://, or https:// scheme, and may be narrowed to a
// namespace or repository path. Tags and digests are not accepted.
func ParseResource(resource string) (Resource, error) {
	trimmed := strings.TrimPrefix(resource, "oci://")
	trimmed = strings.TrimPrefix(trimmed, "http://")
	trimmed = strings.TrimPrefix(trimmed, "https://")
	// A single trailing slash is tolerated; more than one is not a valid path.
	trimmed = strings.TrimSuffix(trimmed, "/")
	if strings.HasSuffix(trimmed, "/") {
		return Resource{}, fmt.Errorf("%w: invalid resource path %q", errdef.ErrInvalidReference, resource)
	}

	host, path := splitRegistry(trimmed)
	if host == "" {
		return Resource{}, fmt.Errorf("%w: invalid registry resource %q", errdef.ErrInvalidReference, resource)
	}
	host = strings.ToLower(host)

	// Validate the host independently so a port is not mistaken for a tag.
	if err := (Reference{Registry: host}).ValidateRegistry(); err != nil {
		return Resource{}, err
	}
	if path == "" {
		return Resource{Registry: host}, nil
	}
	if strings.ContainsAny(path, ":@") {
		return Resource{}, fmt.Errorf("%w: resource must not include a tag or digest", errdef.ErrInvalidReference)
	}
	if err := (Reference{Repository: path}).ValidateRepository(); err != nil {
		return Resource{}, err
	}
	return Resource{Registry: host, Path: path}, nil
}

// Host returns the host name of the registry.
//
// For docker.io, it returns registry-1.docker.io.
func (r Resource) Host() string {
	if r.Registry == "docker.io" {
		return "registry-1.docker.io"
	}
	return r.Registry
}

// String returns the resource as a string.
func (r Resource) String() string {
	if r.Path == "" {
		return r.Registry
	}
	return r.Registry + "/" + r.Path
}
