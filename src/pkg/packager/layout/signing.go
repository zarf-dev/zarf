// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package layout

import (
	"github.com/zarf-dev/zarf/src/pkg/signing"
	"github.com/zarf-dev/zarf/src/types"
)

func signBlobOptions(opts types.SignOptions) signing.SignBlobOptions {
	backendOpts := signing.DefaultSignBlobOptions()
	backendOpts.Key = opts.Key
	backendOpts.Password = opts.Password
	backendOpts.Fulcio.IdentityToken = opts.IdentityToken
	backendOpts.Fulcio.URL = opts.FulcioURL
	backendOpts.Fulcio.AuthFlow = opts.FulcioAuthFlow
	backendOpts.OIDC.Issuer = opts.OIDCIssuer
	backendOpts.OIDC.ClientID = opts.OIDCClientID
	backendOpts.Rekor.URL = opts.RekorURL
	backendOpts.TlogUpload = opts.TlogUpload
	backendOpts.TSAServerURL = opts.TSAServerURL
	backendOpts.SkipConfirmation = opts.SkipConfirmation
	return backendOpts
}

func verifyBlobOptions(opts types.VerifyOptions) signing.VerifyBlobOptions {
	backendOpts := signing.DefaultVerifyBlobOptions()
	backendOpts.Key = opts.Key
	backendOpts.CertVerify.CertIdentity = opts.CertificateIdentity
	backendOpts.CertVerify.CertIdentityRegexp = opts.CertificateIdentityRegexp
	backendOpts.CertVerify.CertOidcIssuer = opts.CertificateOIDCIssuer
	backendOpts.CertVerify.CertOidcIssuerRegexp = opts.CertificateOIDCIssuerRegexp
	backendOpts.CommonVerifyOptions.TrustedRootPath = opts.TrustedRoot
	backendOpts.CommonVerifyOptions.IgnoreTlog = opts.InsecureIgnoreTlog
	backendOpts.CommonVerifyOptions.UseSignedTimestamps = opts.UseSignedTimestamps
	backendOpts.TempDir = opts.TempDir
	return backendOpts
}
