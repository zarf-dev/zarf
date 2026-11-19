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

package cache

import (
	"os"

	"github.com/oras-project/oras-go/v3"
	"github.com/oras-project/oras-go/v3/content/oci"
)

// EnvVarName is the environment variable name for specifying the cache root.
const EnvVarName = "ORAS_CACHE"

// Cache provides options for configuring a cache layer.
type Cache struct {
	// Root specifies the cache root directory.
	Root string
}

// NewFromEnv creates a Cache with the root set from the ORAS_CACHE environment variable.
// Returns nil if the environment variable is not set.
func NewFromEnv() *Cache {
	root := os.Getenv(EnvVarName)
	if root == "" {
		return nil
	}
	return &Cache{
		Root: root,
	}
}

// ReadOnlyTarget wraps the source ReadOnlyTarget with a caching layer.
// The returned target will first check the cache for content before
// fetching from the source. Fetched content is cached while being read.
//
// Note: The returned target implements only oras.ReadOnlyTarget. If the source
// implements additional interfaces (e.g., registry.ReferenceFetcher), those
// methods are also cached, but type assertions for other interfaces may fail.
func (c *Cache) ReadOnlyTarget(source oras.ReadOnlyTarget) (oras.ReadOnlyTarget, error) {
	// Use oci.NewStorage for process-safe cache operations.
	// Unlike oci.New, Storage doesn't maintain index.json and is safe
	// for concurrent access from multiple processes.
	ociStore, err := oci.NewStorage(c.Root)
	if err != nil {
		return nil, err
	}

	return CacheReadOnlyTarget(source, ociStore), nil
}
