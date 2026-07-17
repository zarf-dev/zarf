/*
 * Copyright (c) SAS Institute Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package rpmutils

import (
	"crypto"
	"errors"
	"fmt"
	"hash"
	"io"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// Signature describes a PGP signature found within a RPM while verifying it.
type Signature struct {
	// Signer is the PGP identity that created the signature. It may be nil if
	// the public key is not available at verification time, but KeyId will
	// always be set.
	Signer *openpgp.Entity
	// Hash is the algorithm used to digest the signature contents
	Hash crypto.Hash
	// CreationTime is when the signature was created
	CreationTime time.Time
	// HeaderOnly is true for signatures that only cover the general RPM header,
	// and false for signatures that cover the general header plus the payload
	HeaderOnly bool
	// KeyId is the PGP key that created the signature.
	KeyId uint64
	// KeyFingerprint is the fingerprint of the public key that created the
	// signature, if available.
	KeyFingerprint []byte
	// PrimaryName is the primary identity of the signing key, if available.
	PrimaryName string

	validate func(hash.Hash) error
}

// Verify the PGP signature over a RPM file. knownKeys should enumerate public
// keys to check against, otherwise the signature validity cannot be verified.
// If knownKeys is nil then digests will be checked but only the raw key ID will
// be available.
func Verify(stream io.Reader, knownKeys openpgp.EntityList) (header *RpmHeader, sigs []*Signature, err error) {
	lead, sigHeader, err := readSignatureHeader(stream)
	if err != nil {
		return nil, nil, err
	}
	// parse the general header
	headerDigestValue, headerDigestType := getHashAndType(sigHeader)
	genHeader, err := readHeader(stream, headerDigestValue, headerDigestType, sigHeader.isSource, false)
	if err != nil {
		return nil, nil, err
	}
	// setup digesters for PGP and payload digest
	sigs, hashes, err := digestAndVerify(sigHeader, genHeader, stream, knownKeys)
	if err != nil {
		return nil, nil, err
	}
	// verify PGP signatures
	for i, sig := range sigs {
		h := hashes[i]
		if err := sig.validate(h); err != nil {
			return nil, nil, err
		}
	}
	hdr := &RpmHeader{
		lead:      lead,
		sigHeader: sigHeader,
		genHeader: genHeader,
		isSource:  sigHeader.isSource,
	}
	return hdr, sigs, nil
}

var (
	ErrNoPGPSignature  = errors.New("no supported PGP signature packet found")
	ErrTrailingGarbage = errors.New("trailing garbage after PGP signature packet")
)

type KeyNotFoundError struct {
	KeyID       uint64
	Fingerprint []byte
}

func (e KeyNotFoundError) Error() string {
	if e.KeyID == 0 && len(e.Fingerprint) > 0 {
		return fmt.Sprintf("key with fingerprint %x not found", e.Fingerprint)
	}
	return fmt.Sprintf("keyid %08x not found", e.KeyID)
}
