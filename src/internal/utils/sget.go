// Forked from https://github.com/sigstore/cosign/blob/v1.7.1/pkg/sget/sget.go
package utils

import (
	"context"
	"fmt"
	"io"

	"github.com/defenseunicorns/zarf/src/internal/message"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/pkg/errors"

	"github.com/sigstore/cosign/cmd/cosign/cli/fulcio"
	"github.com/sigstore/cosign/cmd/cosign/cli/options"
	"github.com/sigstore/cosign/pkg/cosign"
	ociremote "github.com/sigstore/cosign/pkg/oci/remote"
	sigs "github.com/sigstore/cosign/pkg/signature"
)

// Sget performs a cosign signature verification on a given image using the specified public key. 
func Sget(image, key string, out io.Writer, ctx context.Context) error {
	message.Debugf("utils.Sget: image=%s, key=%s", image, key)

	spinner := message.NewProgressSpinner("Loading signed file %s", image)
	defer spinner.Stop()

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
	fulcioVerified := (co.SigVerifier == nil)

	co.RootCerts = fulcio.GetRoots()

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

	if fulcioVerified {
		spinner.Updatef("KEYLESS (OIDC). ")
	}

	for _, sig := range sp {
		if cert, err := sig.Cert(); err == nil && cert != nil {
			spinner.Debugf("Certificate subject: ", sigs.CertSubject(cert))
			if issuerURL := sigs.CertIssuerExtension(cert); issuerURL != "" {
				spinner.Debugf("Certificate issuer URL: ", issuerURL)
			}
		}

		p, err := sig.Payload()
		if err != nil {
			spinner.Errorf(err, "Error getting payload")
			return err
		}
		spinner.Debugf(string(p))
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
	spinner.Successf(verifyMsg)

	return err
}
