// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package utils provides generic utility functions.
package utils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/defenseunicorns/pkg/helpers/v2"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/fulcio"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/options"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/sign"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/verify"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	ociremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
	sigs "github.com/sigstore/cosign/v2/pkg/signature"

	// Register the provider-specific plugins
	_ "github.com/sigstore/sigstore/pkg/signature/kms/aws"
	_ "github.com/sigstore/sigstore/pkg/signature/kms/azure"
	_ "github.com/sigstore/sigstore/pkg/signature/kms/gcp"
	_ "github.com/sigstore/sigstore/pkg/signature/kms/hashivault"

	"github.com/zarf-dev/zarf/src/config/lang"
	"github.com/zarf-dev/zarf/src/pkg/message"
)

const (
	cosignB64Enabled        = true
	cosignOutputCertificate = ""
	cosignTLogUpload        = false
)

// Sget performs a cosign signature verification on a given image using the specified public key.
//
// Forked from https://github.com/sigstore/cosign/blob/v1.7.1/pkg/sget/sget.go
func Sget(ctx context.Context, image, key string, out io.Writer) error {
	message.Warnf(lang.WarnSGetDeprecation)

	// Remove the custom protocol header from the url
	image = strings.TrimPrefix(image, helpers.SGETURLPrefix)

	ref, err := name.ParseReference(image)
	if err != nil {
		return err
	}

	opts := []remote.Option{
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
		remote.WithContext(ctx),
	}

	co := &cosign.CheckOpts{
		ClaimVerifier:      cosign.SimpleClaimVerifier,
		RegistryClientOpts: []ociremote.Option{ociremote.WithRemoteOptions(opts...)},
	}
	if _, ok := ref.(name.Tag); ok {
		if key == "" && !options.EnableExperimental() {
			return errors.New("public key must be specified when fetching by tag, you must fetch by digest or supply a public key")
		}
	}
	// Overwrite "ref" with a digest to avoid a race where we verify the tag,
	// and then access the file through the tag.  This has a race where we
	// might download content that isn't what we verified.
	ref, err = ociremote.ResolveDigest(ref, co.RegistryClientOpts...)
	if err != nil {
		return err
	}

	if key != "" {
		pub, err := sigs.LoadPublicKey(ctx, key)
		if err != nil {
			return err
		}
		co.SigVerifier = pub
	}

	// NB: There are only 2 kinds of verification right now:
	// 1. You gave us the public key explicitly to verify against so co.SigVerifier is non-nil or,
	// 2. We're going to find an x509 certificate on the signature and verify against Fulcio root trust
	// TODO(nsmith5): Refactor this verification logic to pass back _how_ verification
	// was performed so we don't need to use this fragile logic here.
	co.RootCerts, err = fulcio.GetRoots()
	if err != nil {
		return fmt.Errorf("getting Fulcio roots: %w", err)
	}

	co.IntermediateCerts, err = fulcio.GetIntermediates()
	if err != nil {
		return fmt.Errorf("getting Fulcio intermediates: %w", err)
	}

	co.IgnoreTlog = true
	co.IgnoreSCT = true
	co.Offline = true

	verifyMsg := fmt.Sprintf("%s cosign verified: ", image)

	sp, bundleVerified, err := cosign.VerifyImageSignatures(ctx, ref, co)
	if err != nil {
		return err
	}

	if co.ClaimVerifier != nil {
		if co.Annotations != nil {
			verifyMsg += "ANNOTATIONS. "
		}
		verifyMsg += "CLAIMS. "
	}

	if bundleVerified {
		verifyMsg += "TRANSPARENCY LOG (BUNDLED). "
	} else if co.RekorClient != nil {
		verifyMsg += "TRANSPARENCY LOG. "
	}

	if co.SigVerifier != nil {
		verifyMsg += "PUBLIC KEY. "
	}

	for _, sig := range sp {
		if cert, err := sig.Cert(); err == nil && cert != nil {
			message.Debugf("Certificate subject: %s", cert.Subject)

			ce := cosign.CertExtensions{Cert: cert}
			if issuerURL := ce.GetIssuer(); issuerURL != "" {
				message.Debugf("Certificate issuer URL: %s", issuerURL)
			}
		}

		p, err := sig.Payload()
		if err != nil {
			return fmt.Errorf("error getting payload: %w", err)
		}
		message.Debug(string(p))
	}

	// TODO(mattmoor): Depending on what this is, use the higher-level stuff.
	img, err := remote.Image(ref, opts...)
	if err != nil {
		return err
	}
	layers, err := img.Layers()
	if err != nil {
		return err
	}
	if len(layers) != 1 {
		return errors.New("invalid artifact")
	}
	rc, err := layers[0].Compressed()
	if err != nil {
		return err
	}

	_, err = io.Copy(out, rc)
	return err
}

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

	message.Successf("Package signature validated!")
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
	simg, _ := ociremote.SignedEntity(ref, remoteOpts...) // TODO(mkcp): //nolint:errcheck
	if simg == nil {
		return nil, nil
	}

	// Errors are dogsled because these functions always return a name.Tag which we can check for layers
	sigRef, _ := ociremote.SignatureTag(ref, remoteOpts...)   // TODO(mkcp): //nolint:errcheck
	attRef, _ := ociremote.AttestationTag(ref, remoteOpts...) // TODO(mkcp): //nolint:errcheck

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
