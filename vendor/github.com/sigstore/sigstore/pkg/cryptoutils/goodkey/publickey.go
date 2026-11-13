//
// Copyright 2021 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package goodkey implements public key validation
//
// Deprecated: Validation will no longer check keys using letsencrypt/boulder.
// Validation is now minimal and unnecessary as verification will only
// accept current key sizes. This package will be removed in a future release.
package goodkey

import (
	"crypto"

	"github.com/sigstore/sigstore/pkg/cryptoutils"
)

// ValidatePubKey validates the parameters of an RSA, ECDSA, or ED25519 public key.
func ValidatePubKey(pub crypto.PublicKey) error {
	return cryptoutils.ValidatePubKey(pub) // //nolint:staticcheck
}
