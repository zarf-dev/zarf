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

package registry

import (
	"context"
	"encoding/json"
	"fmt"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/oras-project/oras-go/v3/content"
	"github.com/oras-project/oras-go/v3/errdef"
	"github.com/oras-project/oras-go/v3/internal/descriptor"
	"github.com/oras-project/oras-go/v3/internal/spec"
)

// Tags lists the tags available in the repository.
func Tags(ctx context.Context, repo TagLister) ([]string, error) {
	var res []string
	if err := repo.Tags(ctx, "", func(tags []string) error {
		res = append(res, tags...)
		return nil
	}); err != nil {
		return nil, err
	}
	return res, nil
}

// Referrers lists the descriptors of image or artifact manifests directly
// referencing the given manifest descriptor.
//
// Reference: https://github.com/opencontainers/distribution-spec/blob/v1.1.1/spec.md#listing-referrers
func Referrers(ctx context.Context, store content.ReadOnlyGraphStorage, desc ocispec.Descriptor, artifactType string) ([]ocispec.Descriptor, error) {
	if !descriptor.IsManifest(desc) {
		return nil, fmt.Errorf("the descriptor %v is not a manifest: %w", desc, errdef.ErrUnsupported)
	}

	var results []ocispec.Descriptor

	// use the Referrer API if it is available
	if rf, ok := store.(ReferrerLister); ok {
		if err := rf.Referrers(ctx, desc, artifactType, func(referrers []ocispec.Descriptor) error {
			results = append(results, referrers...)
			return nil
		}); err != nil {
			return nil, err
		}
		return results, nil
	}

	predecessors, err := store.Predecessors(ctx, desc)
	if err != nil {
		return nil, err
	}
	for _, node := range predecessors {
		switch node.MediaType {
		case ocispec.MediaTypeImageManifest:
			fetched, err := content.FetchAll(ctx, store, node)
			if err != nil {
				return nil, err
			}
			var manifest ocispec.Manifest
			if err := json.Unmarshal(fetched, &manifest); err != nil {
				return nil, err
			}
			if manifest.Subject == nil || !content.Equal(*manifest.Subject, desc) {
				continue
			}
			node.ArtifactType = manifest.ArtifactType
			if node.ArtifactType == "" {
				node.ArtifactType = manifest.Config.MediaType
			}
			node.Annotations = manifest.Annotations
		case ocispec.MediaTypeImageIndex:
			fetched, err := content.FetchAll(ctx, store, node)
			if err != nil {
				return nil, err
			}
			var index ocispec.Index
			if err := json.Unmarshal(fetched, &index); err != nil {
				return nil, err
			}
			if index.Subject == nil || !content.Equal(*index.Subject, desc) {
				continue
			}
			node.ArtifactType = index.ArtifactType
			node.Annotations = index.Annotations
		case spec.MediaTypeArtifactManifest:
			fetched, err := content.FetchAll(ctx, store, node)
			if err != nil {
				return nil, err
			}
			var artifact spec.Artifact
			if err := json.Unmarshal(fetched, &artifact); err != nil {
				return nil, err
			}
			if artifact.Subject == nil || !content.Equal(*artifact.Subject, desc) {
				continue
			}
			node.ArtifactType = artifact.ArtifactType
			node.Annotations = artifact.Annotations
		default:
			continue
		}
		if artifactType == "" || artifactType == node.ArtifactType {
			// the field artifactType in referrers descriptor is allowed to be empty
			// https://github.com/opencontainers/distribution-spec/issues/458
			results = append(results, node)
		}
	}
	return results, nil
}
