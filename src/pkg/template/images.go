// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package template

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
)

func imageRepository(image string) (string, error) {
	ref, err := name.ParseReference(image)
	if err != nil {
		return "", fmt.Errorf("parsing image reference %q: %w", image, err)
	}
	return ref.Context().RepositoryStr(), nil
}

func imageTag(image string) (string, error) {
	ref, err := name.ParseReference(image)
	if err != nil {
		return "", fmt.Errorf("parsing image reference %q: %w", image, err)
	}
	tag, ok := ref.(name.Tag)
	if !ok {
		return "", fmt.Errorf("image reference %q has no tag", image)
	}
	return tag.TagStr(), nil
}
