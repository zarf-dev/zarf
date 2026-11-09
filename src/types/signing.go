// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package types

// SignOptions configures signing for packages and component manifests.
// A nonnil signing options pointer requests signing; an empty Key selects keyless signing.
// TlogUpload is a resolved policy, not an unset-aware CLI flag. Callers choose
// policy explicitly; the CLI resolves flag and configuration presence first.
type SignOptions struct {
	Key              string
	Password         string
	IdentityToken    string
	FulcioURL        string
	FulcioAuthFlow   string
	OIDCIssuer       string
	OIDCClientID     string
	RekorURL         string
	TlogUpload       bool
	TSAServerURL     string
	SkipConfirmation bool
}

// VerifyOptions configures trust for packages and component manifests.
// InsecureIgnoreTlog is a resolved policy, not an unset-aware CLI flag.
// Callers choose policy explicitly; the CLI resolves configuration presence first.
type VerifyOptions struct {
	Key                         string
	CertificateIdentity         string
	CertificateIdentityRegexp   string
	CertificateOIDCIssuer       string
	CertificateOIDCIssuerRegexp string
	TrustedRoot                 string
	InsecureIgnoreTlog          bool
	UseSignedTimestamps         bool
	// TempDir is the parent for staging an embedded trusted root during verification.
	// It does not control package extraction, signing, caches, or backend-managed files.
	// Empty uses Zarf's configured temp directory for package verification and the
	// system temp directory for manifest verification. An explicit TrustedRoot or
	// public Key avoids embedded-root staging.
	TempDir string
}

// DefaultSignOptions returns library defaults, without CLI endpoint defaults.
// Supplied options are never merged with defaults: empty and false are meaningful.
func DefaultSignOptions() SignOptions {
	return SignOptions{OIDCClientID: "sigstore"}
}

// DefaultVerifyOptions returns library defaults for air-gapped verification.
func DefaultVerifyOptions() VerifyOptions {
	return VerifyOptions{InsecureIgnoreTlog: true}
}
