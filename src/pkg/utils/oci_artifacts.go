// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic utility functions.
package utils

import (
	"github.com/google/go-containerregistry/pkg/name"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
)

// GetCosignArtifacts returns signatures and attestations for the given image.
func GetCosignArtifacts(image string) ([]string, error) {
	var nameOpts []name.Option

	ref, err := name.ParseReference(image, nameOpts...)
	if err != nil {
		return nil, err
	}

	var remoteOpts []ociremote.Option
	simg, _ := ociremote.SignedEntity(ref, remoteOpts...) //nolint:errcheck
	if simg == nil {
		return nil, nil
	}

	sigRef, _ := ociremote.SignatureTag(ref, remoteOpts...)   //nolint:errcheck
	attRef, _ := ociremote.AttestationTag(ref, remoteOpts...) //nolint:errcheck

	ss, err := simg.Signatures()
	if err != nil {
		return nil, err
	}
	ssLayers, err := ss.Layers()
	if err != nil {
		return nil, err
	}

	var cosignArtifactList = make([]string, 0)
	if 0 < len(ssLayers) {
		cosignArtifactList = append(cosignArtifactList, sigRef.String())
	}

	atts, err := simg.Attestations()
	if err != nil {
		return nil, err
	}
	aLayers, err := atts.Layers()
	if err != nil {
		return nil, err
	}
	if 0 < len(aLayers) {
		cosignArtifactList = append(cosignArtifactList, attRef.String())
	}
	return cosignArtifactList, nil
}
