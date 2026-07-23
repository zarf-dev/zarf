// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rsa

import (
	"crypto"
	"crypto/rsa"
	"errors"
	"hash"
	"io"
)

var invalidSaltLenErr = errors.New("crypto/rsa: PSSOptions.SaltLength cannot be negative")

// Per RFC 8017, Section 9.1
//
//     EM = MGF1 xor DB || H( 8*0x00 || mHash || salt ) || 0xbc
//
// where
//
//     DB = PS || 0x01 || salt
//
// and PS can be empty so
//
//     emLen = dbLen + hLen + 1 = psLen + sLen + hLen + 2
//

// EMSAPSSEncode is extracted from SignPSS, and is used to generate a EM value
// for a PSS signature operation.
func EMSAPSSEncode(mHash []byte, pub *rsa.PublicKey, salt []byte, hash hash.Hash) ([]byte, error) {
	emBits := pub.N.BitLen() - 1

	// See RFC 8017, Section 9.1.1.

	hLen := hash.Size()
	sLen := len(salt)
	emLen := (emBits + 7) / 8

	// 1.  If the length of M is greater than the input limitation for the
	//     hash function (2^61 - 1 octets for SHA-1), output "message too
	//     long" and stop.
	//
	// 2.  Let mHash = Hash(M), an octet string of length hLen.

	if len(mHash) != hLen {
		return nil, errors.New("crypto/rsa: input must be hashed with given hash")
	}

	// 3.  If emLen < hLen + sLen + 2, output "encoding error" and stop.

	if emLen < hLen+sLen+2 {
		return nil, rsa.ErrMessageTooLong
	}

	em := make([]byte, emLen)
	psLen := emLen - sLen - hLen - 2
	db := em[:psLen+1+sLen]
	h := em[psLen+1+sLen : emLen-1]

	// 4.  Generate a random octet string salt of length sLen; if sLen = 0,
	//     then salt is the empty string.
	//
	// 5.  Let
	//       M' = (0x)00 00 00 00 00 00 00 00 || mHash || salt;
	//
	//     M' is an octet string of length 8 + hLen + sLen with eight
	//     initial zero octets.
	//
	// 6.  Let H = Hash(M'), an octet string of length hLen.

	var prefix [8]byte

	hash.Write(prefix[:])
	hash.Write(mHash)
	hash.Write(salt)

	h = hash.Sum(h[:0])
	hash.Reset()

	// 7.  Generate an octet string PS consisting of emLen - sLen - hLen - 2
	//     zero octets. The length of PS may be 0.
	//
	// 8.  Let DB = PS || 0x01 || salt; DB is an octet string of length
	//     emLen - hLen - 1.

	db[psLen] = 0x01
	copy(db[psLen+1:], salt)

	// 9.  Let dbMask = MGF(H, emLen - hLen - 1).
	//
	// 10. Let maskedDB = DB \xor dbMask.

	mgf1XOR(db, hash, h)

	// 11. Set the leftmost 8 * emLen - emBits bits of the leftmost octet in
	//     maskedDB to zero.

	db[0] &= 0xff >> (8*emLen - emBits)

	// 12. Let EM = maskedDB || H || 0xbc.
	em[emLen-1] = 0xbc

	// 13. Output EM.
	return em, nil
}

// mgf1XOR XORs the bytes in out with a mask generated using the MGF1 function
// specified in PKCS #1 v2.1.
func mgf1XOR(out []byte, hash hash.Hash, seed []byte) {
	var counter [4]byte
	var digest []byte

	done := 0
	for done < len(out) {
		hash.Write(seed)
		hash.Write(counter[0:4])
		digest = hash.Sum(digest[:0])
		hash.Reset()

		for i := 0; i < len(digest) && done < len(out); i++ {
			out[done] ^= digest[i]
			done++
		}
		incCounter(&counter)
	}
}

// incCounter increments a four byte, big-endian counter.
func incCounter(c *[4]byte) {
	if c[3]++; c[3] != 0 {
		return
	}
	if c[2]++; c[2] != 0 {
		return
	}
	if c[1]++; c[1] != 0 {
		return
	}
	c[0]++
}

// NewSalt is extracted from SignPSS and is used to generate a salt value for a
// PSS signature.
func NewSalt(rand io.Reader, pub *rsa.PublicKey, hash crypto.Hash, opts *rsa.PSSOptions) ([]byte, error) {
	saltLength := opts.SaltLength
	switch saltLength {
	case rsa.PSSSaltLengthAuto:
		saltLength = (pub.N.BitLen()-1+7)/8 - 2 - hash.Size()
		if saltLength < 0 {
			return nil, rsa.ErrMessageTooLong
		}
	case rsa.PSSSaltLengthEqualsHash:
		saltLength = hash.Size()
	default:
		// If we get here saltLength is either > 0 or < -1, in the
		// latter case we fail out.
		if saltLength <= 0 {
			return nil, invalidSaltLenErr
		}
	}
	salt := make([]byte, saltLength)
	if _, err := io.ReadFull(rand, salt); err != nil {
		return nil, err
	}
	return salt, nil
}
