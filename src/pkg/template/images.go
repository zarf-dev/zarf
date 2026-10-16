// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package template

import "github.com/zarf-dev/zarf/src/pkg/transform"

func imageRepository(image string) (string, error) {
	ref, err := transform.ParseImageRef(image)
	if err != nil {
		return "", err
	}
	return ref.Path, nil
}
