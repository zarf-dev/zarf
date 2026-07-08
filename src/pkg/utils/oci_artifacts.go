// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic utility functions.
package utils

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
	"github.com/zarf-dev/zarf/src/pkg/logger"
	"github.com/zarf-dev/zarf/src/pkg/ocitransport"
	"github.com/zarf-dev/zarf/src/types"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/registry"
	orasRemote "oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// GetCosignArtifacts returns signatures and attestations for the given image.
func GetCosignArtifacts(ctx context.Context, image string, client *auth.Client, remoteOptions types.RemoteOptions) ([]string, error) {
	l := logger.From(ctx)

	var nameOpts []name.Option
	if client == nil {
		return nil, fmt.Errorf("auth client is required")
	}

	ref, err := name.ParseReference(image, nameOpts...)
	if err != nil {
		return nil, err
	}

	// This image reference was discovered by scanning a package's resources, not
	// named explicitly on the command line, so remoteOptions.PlainHTTP is not applied
	// to it directly.
	var plainHTTP bool
	if remoteOptions.PlainHTTP {
		plainHTTP, err = ocitransport.From(ctx).UsePlainHTTP(ctx, ref.Context().RegistryStr(), ocitransport.ProbeOptions{InsecureSkipTLSVerify: remoteOptions.InsecureSkipTLSVerify})
		if err != nil {
			// If we can't reach the registry, we can't get the cosign artifacts so log the error and skip it
			l.Debug("could not reach registry for cosign artifact lookup", "image", image, "error", err)
			return nil, nil
		}
	}

	// We get the digest reference for the image specifically so that we can short circuit the
	// `crane` lookup that would otherwise happen in ociremote.SignatureTag and ociremote.AttestationTag
	digestRef, err := imageDigestRef(ctx, image, ref, client, plainHTTP)
	if err != nil {
		// If we can't get the digest reference, we can't get the cosign artifacts so log the error and skip it
		l.Debug("could not get digest reference for image", "image", image, "error", err)
		return nil, nil
	}

	sigTag, err := ociremote.SignatureTag(digestRef)
	if err != nil {
		return nil, err
	}
	attTag, err := ociremote.AttestationTag(digestRef)
	if err != nil {
		return nil, err
	}

	var cosignArtifactList = make([]string, 0, 2)

	sigExists, err := existsInRemote(ctx, sigTag.String(), client, plainHTTP)
	if err != nil {
		return nil, err
	}
	if sigExists {
		cosignArtifactList = append(cosignArtifactList, sigTag.String())
	}

	attExists, err := existsInRemote(ctx, attTag.String(), client, plainHTTP)
	if err != nil {
		return nil, err
	}
	if attExists {
		cosignArtifactList = append(cosignArtifactList, attTag.String())
	}

	return cosignArtifactList, nil
}

func imageDigestRef(ctx context.Context, reference string, parsedRef name.Reference, client *auth.Client, plainHTTP bool) (name.Digest, error) {
	if digestRef, ok := parsedRef.(name.Digest); ok {
		return digestRef, nil
	}

	repo := &orasRemote.Repository{PlainHTTP: plainHTTP}
	orasRef, err := registry.ParseReference(reference)
	if err != nil {
		return name.Digest{}, err
	}
	repo.Reference = orasRef
	repo.Client = client

	desc, err := oras.Resolve(ctx, repo, reference, oras.DefaultResolveOptions)
	if err != nil {
		return name.Digest{}, err
	}

	digestRef, err := name.NewDigest(fmt.Sprintf("%s@%s", parsedRef.Context().Name(), desc.Digest.String()))
	if err != nil {
		return name.Digest{}, err
	}

	return digestRef, nil
}

func existsInRemote(ctx context.Context, reference string, client *auth.Client, plainHTTP bool) (bool, error) {
	repo := &orasRemote.Repository{PlainHTTP: plainHTTP}

	ref, err := registry.ParseReference(reference)
	if err != nil {
		return false, err
	}
	repo.Reference = ref
	repo.Client = client

	_, err = oras.Resolve(ctx, repo, reference, oras.DefaultResolveOptions)
	if err != nil {
		if errors.Is(err, errdef.ErrNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}
