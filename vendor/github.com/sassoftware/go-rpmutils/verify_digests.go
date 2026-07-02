package rpmutils

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"

	"github.com/ProtonMail/go-crypto/openpgp"
)

var headerSigTags = []int{SIG_RSA, SIG_DSA}
var payloadSigTags = []int{
	SIG_PGP - _SIGHEADER_TAG_BASE,
	SIG_GPG - _SIGHEADER_TAG_BASE,
}

// Try to parse a PGP signature with the given tag and return its metadata and
// hash function.
func setupDigester(sigHeader *rpmHeader, tag int, knownKeys openpgp.EntityList) (*Signature, error) {
	blob, err := sigHeader.GetBytes(tag)
	if _, ok := err.(NoSuchTagError); ok {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return parseSignature(blob, knownKeys)
}

// Parse signatures from the header and determine which hash functions are
// needed to digest the RPM. The caller must write the payload to the returned
// WriteCloser, then call Close to check if the payload digest matches.
func digestAndVerify(sigHeader, genHeader *rpmHeader, payloadReader io.Reader, knownKeys openpgp.EntityList) ([]*Signature, []hash.Hash, error) {
	var sigs []*Signature
	var hashes []hash.Hash
	// signatures over the general header alone
	for _, tag := range headerSigTags {
		sig, err := setupDigester(sigHeader, tag, knownKeys)
		if err != nil {
			return nil, nil, err
		} else if sig == nil {
			continue
		}
		sig.HeaderOnly = true
		h := sig.Hash.New()
		h.Write(genHeader.orig)
		sigs = append(sigs, sig)
		hashes = append(hashes, h)
	}
	// signatures over the general header + payload
	var payloadWriters []io.Writer
	for _, tag := range payloadSigTags {
		sig, err := setupDigester(sigHeader, tag, knownKeys)
		if err != nil {
			return nil, nil, err
		} else if sig == nil {
			continue
		}
		sig.HeaderOnly = false
		h := sig.Hash.New()
		h.Write(genHeader.orig)
		payloadWriters = append(payloadWriters, h)
		sigs = append(sigs, sig)
		hashes = append(hashes, h)
	}
	err := digestPayload(sigHeader, genHeader, payloadReader, payloadWriters)
	return sigs, hashes, err
}

func digestPayload(sigHeader, genHeader *rpmHeader, payloadReader io.Reader, payloadWriters []io.Writer) error {
	// Also compute a digest over the payload for integrity checking purposes
	if payloadValue, payloadType := getPayloadDigest(genHeader); payloadType != 0 {
		if !payloadType.Available() {
			return fmt.Errorf("unknown payload digest %s", payloadType)
		}
		payloadHasher := payloadType.New()
		// hash payload only
		payloadWriters = append(payloadWriters, payloadHasher)
		if _, err := io.Copy(io.MultiWriter(payloadWriters...), payloadReader); err != nil {
			return err
		}
		calculated := hex.EncodeToString(payloadHasher.Sum(nil))
		if calculated != payloadValue {
			return fmt.Errorf("payload %s digest mismatch", payloadType)
		}
		return nil
	}
	// Check legacy MD5 digest in sig header as a last resort. This is the only
	// digest found in the signature header that covers the payload, so for some
	// old RPMs that don't have a payload digest in the general header this is
	// the only integrity check we can use unless we're verifying the PGP
	// signatures.
	if sigmd5, _ := sigHeader.GetBytes(SIG_MD5 - _SIGHEADER_TAG_BASE); len(sigmd5) != 0 {
		payloadHasher := md5.New()
		// hash header + payload
		payloadHasher.Write(genHeader.orig)
		payloadWriters = append(payloadWriters, payloadHasher)
		if _, err := io.Copy(io.MultiWriter(payloadWriters...), payloadReader); err != nil {
			return err
		}
		calculated := payloadHasher.Sum(nil)
		if !bytes.Equal(calculated, sigmd5) {
			return errors.New("md5 digest mismatch")
		}
		return nil
	}
	return errors.New("no usable payload digest found")
}
