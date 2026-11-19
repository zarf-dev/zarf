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

package auth

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/oras-project/oras-go/v3/registry/remote/properties"
)

// apiPrefix is the path prefix of the distribution API.
const apiPrefix = "/v2/"

// endpointSeparators separate a repository name from the endpoint acting on it.
//
// Reference: https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#endpoints
var endpointSeparators = []string{
	"/manifests/",
	"/blobs/",
	"/tags/list",
	"/referrers/",
}

// requestResource derives the registry resource addressed by req.
// A URL that does not address a repository yields the whole registry.
//
// Registry is the canonical registry name, not the host on the wire: a request
// dialed at registry-1.docker.io derives docker.io, so that a derived Resource
// compares equal to ParseResource("docker.io/..."). Callers that dial, or that
// resolve credentials, must map it back with Host().
func requestResource(req *http.Request) properties.Resource {
	host := req.Host
	if host == "" && req.URL != nil {
		host = req.URL.Host
	}
	// Store the canonical name; Resource.Host() maps it back for dialing.
	resource := properties.Resource{Registry: canonicalRegistry(host)}
	if req.URL != nil {
		resource.Path = repositoryFromPath(req.URL.Path)
	}
	return resource
}

// canonicalRegistry returns the canonical name of a registry host: lower-cased,
// as registry host names are case-insensitive, with the Docker Hub endpoint
// mapped back to the name it is addressed by. Resource.Host() maps it back for
// dialing.
func canonicalRegistry(host string) string {
	host = strings.ToLower(host)
	if host == "registry-1.docker.io" {
		return "docker.io"
	}
	return host
}

// repositoryFromPath returns the repository named by a distribution API path,
// or an empty string if there is none.
func repositoryFromPath(path string) string {
	path, ok := strings.CutPrefix(path, apiPrefix)
	if !ok {
		return ""
	}

	// Split at the rightmost separator, as the repository name may itself
	// contain a segment like "blobs" or "manifests".
	end := -1
	for _, separator := range endpointSeparators {
		if index := strings.LastIndex(path, separator); index > end {
			end = index
		}
	}
	if end <= 0 {
		return ""
	}

	// Degrade to the whole registry rather than name a bogus repository.
	repository := path[:end]
	if err := (properties.Reference{Repository: repository}).ValidateRepository(); err != nil {
		return ""
	}
	return repository
}

// crossesRepository reports whether a and b address different repositories
// under the distribution API. A URL that does not address a repository is not
// a repository boundary.
func crossesRepository(a, b *url.URL) bool {
	from := repositoryFromPath(a.Path)
	to := repositoryFromPath(b.Path)
	return from != "" && to != "" && from != to
}
