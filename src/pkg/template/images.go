// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package template

import (
	"fmt"

	"github.com/zarf-dev/zarf/src/pkg/transform"
)

func imageRepository(image string) (string, error) {
	ref, err := transform.ParseImageRef(image)
	if err != nil {
		return "", err
	}
	return ref.Path, nil
}

func imageTag(image string) (string, error) {
	ref, err := transform.ParseImageRef(image)
	if err != nil {
		return "", err
	}
	if ref.Digest != "" {
		return "", fmt.Errorf("image reference %q requires a digest suffix", image)
	}
	return ref.Tag, nil
}

func imageTagOrDigest(image string) (string, error) {
	ref, err := transform.ParseImageRef(image)
	if err != nil {
		return "", err
	}
	return ref.TagOrDigest, nil
}
