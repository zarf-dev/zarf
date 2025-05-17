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
	"oras.land/oras-go/v2"
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

func getManifestMediaType(ctx context.Context, zarfState *state.State, registryAddress string) (string, error) {
	l := logger.From(ctx)

	client := &auth.Client{
		Client: orasRetry.DefaultClient,
		Cache:  auth.NewCache(),
		Credential: auth.StaticCredential(registryAddress, auth.Credential{
			Username: zarfState.RegistryInfo.PullUsername,
			Password: zarfState.RegistryInfo.PullPassword,
		}),
	}

	registry := &orasRemote.Repository{
		PlainHTTP: true,
		Client:    client,
	}

	_, b, err := oras.FetchBytes(ctx, registry, registryAddress, oras.DefaultFetchBytesOptions)

	if err != nil {
		l.Debug("Got the following error when trying to fetch manifest", "manifest", registryAddress, "error", err)
		return "", err
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(b, &manifest); err != nil {
		l.Debug("Unable to unmarshal the manifest json", "manifest", registryAddress, "error", err)
		return "", err
	}

	return manifest.Config.MediaType, nil
}
