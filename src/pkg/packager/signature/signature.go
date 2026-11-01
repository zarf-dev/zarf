// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package signature defines how packages are handed to a signing implementation such as pkg/signing,
// so loading and deploying packages does not have to link one.
package signature

import (
	"context"
	"errors"
)

// ErrNoVerificationMaterial is returned when there is nothing to verify against.
var ErrNoVerificationMaterial = errors.New("no verification material available")

// Files are the package files a Verifier checks.
type Files struct {
	// ZarfYAML is the signed package definition.
	ZarfYAML string
	// Bundle is the Sigstore bundle, which may not exist.
	Bundle string
	// LegacySignature is the signature used before bundles, which may not exist.
	LegacySignature string
	// Signed is whether the package definition says the package is signed.
	Signed bool
}

// Verifier verifies a package signature.
type Verifier interface {
	// VerifyPackage returns an error wrapping ErrNoVerificationMaterial when there is nothing to verify against.
	VerifyPackage(ctx context.Context, files Files) error
}

// SignRequest describes the files involved in signing a package.
type SignRequest struct {
	// ZarfYAML is the package definition to sign.
	ZarfYAML string
	// Bundle is where the Sigstore bundle is written.
	Bundle string
	// Destination is where the bundle ends up once signing succeeds, so a Signer can refuse to overwrite it.
	Destination string
}

// Signer signs a package definition and writes a Sigstore bundle.
type Signer interface {
	SignPackage(ctx context.Context, req SignRequest) error
}
