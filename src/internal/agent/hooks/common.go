// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package hooks contains the mutation hooks for the Zarf agent.
package hooks

import (
	"context"
	"encoding/json"
	"fmt"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/zarf-dev/zarf/src/internal/agent/operations"
	"github.com/zarf-dev/zarf/src/internal/packager/images"
	"github.com/zarf-dev/zarf/src/pkg/state"
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

func getManifestConfigMediaType(ctx context.Context, zarfState *state.State, imageAddress string) (string, error) {
	ref, err := registry.ParseReference(imageAddress)
	if err != nil {
		return "", err
	}
	client := &auth.Client{
		Client: orasRetry.DefaultClient,
		Cache:  auth.NewCache(),
		Credential: auth.StaticCredential(ref.Registry, auth.Credential{
			Username: zarfState.RegistryInfo.PullUsername,
			Password: zarfState.RegistryInfo.PullPassword,
		}),
	}

	http, err := images.ShouldUsePlainHTTP(ctx, ref.Registry, client)
	if err != nil {
		return "", err
	}

	registry := &orasRemote.Repository{
		PlainHTTP: http,
		Reference: ref,
		Client:    client,
	}

	_, b, err := oras.FetchBytes(ctx, registry, imageAddress, oras.DefaultFetchBytesOptions)

	if err != nil {
		return "", fmt.Errorf("got an error when trying to access the manifest for %s, error %w", imageAddress, err)
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(b, &manifest); err != nil {
		return "", fmt.Errorf("unable to unmarshal the manifest json for %s", imageAddress)
	}

	return manifest.Config.MediaType, nil
}
