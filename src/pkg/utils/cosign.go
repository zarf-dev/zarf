// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic utility functions.
package utils

import (
	"context"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/options"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/sign"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/verify"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	ociremote "github.com/sigstore/cosign/v2/pkg/oci/remote"

	// Register the provider-specific plugins
	_ "github.com/sigstore/sigstore/pkg/signature/kms/aws"
	_ "github.com/sigstore/sigstore/pkg/signature/kms/azure"
	_ "github.com/sigstore/sigstore/pkg/signature/kms/gcp"
	_ "github.com/sigstore/sigstore/pkg/signature/kms/hashivault"

	"github.com/zarf-dev/zarf/src/pkg/logger"
)

const (
	cosignB64Enabled        = true
	cosignOutputCertificate = ""
	cosignTLogUpload        = false
)

// CosignVerifyBlob verifies the zarf.yaml.sig was signed with the key provided by the flag
func CosignVerifyBlob(ctx context.Context, blobRef, sigRef, keyPath string) error {
	keyOptions := options.KeyOpts{KeyRef: keyPath}
	cmd := &verify.VerifyBlobCmd{
		KeyOpts:    keyOptions,
		SigRef:     sigRef,
		IgnoreSCT:  true,
		Offline:    true,
		IgnoreTlog: true,
	}
	err := cmd.Exec(ctx, blobRef)
	if err != nil {
		return err
	}

	logger.From(ctx).Debug("package signature validated", "key", keyPath)
	return nil
}

// CosignSignBlob signs the provide binary and returns the signature
func CosignSignBlob(blobPath, outputSigPath, keyPath string, passFn cosign.PassFunc) ([]byte, error) {
	rootOptions := &options.RootOptions{
		Verbose: false,
		Timeout: options.DefaultTimeout,
	}

	keyOptions := options.KeyOpts{
		KeyRef:   keyPath,
		PassFunc: passFn,
	}

	sig, err := sign.SignBlobCmd(
		rootOptions,
		keyOptions,
		blobPath,
		cosignB64Enabled,
		outputSigPath,
		cosignOutputCertificate,
		cosignTLogUpload)
	if err != nil {
		return []byte{}, err
	}

	return sig, nil
}

// GetCosignArtifacts returns signatures and attestations for the given image
func GetCosignArtifacts(image string) ([]string, error) {
	var nameOpts []name.Option

	ref, err := name.ParseReference(image, nameOpts...)
	if err != nil {
		return nil, err
	}

	// Return empty if we don't have a signature on the image
	var remoteOpts []ociremote.Option
	simg, _ := ociremote.SignedEntity(ref, remoteOpts...) //nolint:errcheck
	if simg == nil {
		return nil, nil
	}

	// Errors are dogsled because these functions always return a name.Tag which we can check for layers
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
