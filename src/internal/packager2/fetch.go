// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package packager2

import (
	"context"
	"fmt"

	"github.com/defenseunicorns/pkg/oci"
	"github.com/zarf-dev/zarf/src/api/v1alpha1"
	"github.com/zarf-dev/zarf/src/pkg/zoci"
)

type FetchOptions struct {
	Source                  string
	Shasum                  string
	Architecture            string
	PublicKeyPath           string
	SkipSignatureValidation bool
}

// FetchZarfYAML fetches the zarf.yaml file from the remote repository.
func FetchZarfYAML(ctx context.Context, opts FetchOptions, mods ...oci.Modifier) (v1alpha1.ZarfPackage, error) {
	if opts.Shasum != "" {
		opts.Source = fmt.Sprintf("%s@sha256:%s", opts.Source, opts.Shasum)
	}
	platform := oci.PlatformForArch(opts.Architecture)
	remote, err := zoci.NewRemote(ctx, opts.Source, platform, mods...)
	if err != nil {
		return v1alpha1.ZarfPackage{}, err
	}
	return remote.FetchZarfYAML(ctx)
}
