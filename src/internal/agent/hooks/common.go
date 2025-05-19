// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hooks contains the mutation hooks for the Zarf agent.
package hooks

import (
	"context"
	"encoding/json"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/internal/agent/operations"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/state"
	"github.com/zarf-dev/zarf/src/pkg/transform"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry"
	orasRemote "oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	orasRetry "oras.land/oras-go/v2/registry/remote/retry"
)

func getLabelPatch(currLabels map[string]string) operations.PatchOperation {
	if currLabels == nil {
		currLabels = make(map[string]string)
	}
	currLabels["zarf-agent"] = "patched"
	return operations.ReplacePatchOperation("/metadata/labels", currLabels)
}

func getManifestMediaType(ctx context.Context, zarfState *state.State, imageAddress string) (string, error) {
	l := logger.From(ctx)

	image, err := transform.ParseImageRef(imageAddress)
	if err != nil {
		return "", err
	}

	registry := &orasRemote.Repository{
		PlainHTTP: true,
		Reference: registry.Reference{
			Registry:   image.Host,
			Repository: image.Path,
			Reference:  image.Reference,
		},
		Client: &auth.Client{
			Client: orasRetry.DefaultClient,
			Cache:  auth.NewCache(),
			Credential: auth.StaticCredential(imageAddress, auth.Credential{
				Username: zarfState.RegistryInfo.PullUsername,
				Password: zarfState.RegistryInfo.PullPassword,
			}),
		},
	}

	_, b, err := oras.FetchBytes(ctx, registry, imageAddress, oras.DefaultFetchBytesOptions)

	if err != nil {
		l.Debug("Got the following error when trying to fetch manifest", "imageAddress", imageAddress, "error", err)
		return "", err
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(b, &manifest); err != nil {
		l.Debug("Unable to unmarshal the manifest json", "manifest", imageAddress, "error", err)
		return "", err
	}

	return manifest.Config.MediaType, nil
}
