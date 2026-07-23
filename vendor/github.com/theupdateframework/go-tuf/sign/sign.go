package sign

import (
	"encoding/json"
	"errors"

	"github.com/secure-systems-lab/go-securesystemslib/cjson"
	"github.com/theupdateframework/go-tuf/data"
	"github.com/theupdateframework/go-tuf/pkg/keys"
)

const maxSignatures = 1024

// MakeSignatures creates data.Signatures for canonical using signer k.
//
// There will be one data.Signature for each of k's IDs, each wih the same
// signature data.
func MakeSignatures(canonical []byte, k keys.Signer) ([]data.Signature, error) {
	sigData, err := k.SignMessage(canonical)
	if err != nil {
		return nil, err
	}

	ids := k.PublicData().IDs()
	signatures := make([]data.Signature, 0, len(ids))
	for _, id := range ids {
		signatures = append(signatures, data.Signature{
			KeyID:     id,
			Signature: sigData,
		})
	}

	return signatures, nil
}

// Sign signs the to-be-signed part of s using the signer k.
//
// The new signature(s) (one for each of k's key IDs) are appended to
// s.Signatures. Existing signatures for the Key IDs are replaced.
func Sign(s *data.Signed, k keys.Signer) error {
	canonical, err := cjson.EncodeCanonical(s.Signed)
	if err != nil {
		return err
	}

	size := len(s.Signatures)
	if size > maxSignatures-1 {
		return errors.New("value too large")
	}
	signatures := make([]data.Signature, 0, size+1)
	for _, oldSig := range s.Signatures {
		if !k.PublicData().ContainsID(oldSig.KeyID) {
			signatures = append(signatures, oldSig)
		}
	}

	newSigs, err := MakeSignatures(canonical, k)
	if err != nil {
		return err
	}
	signatures = append(signatures, newSigs...)

	s.Signatures = signatures
	return nil
}

func Marshal(v interface{}, keys ...keys.Signer) (*data.Signed, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	s := &data.Signed{Signed: b, Signatures: make([]data.Signature, 0)}
	for _, k := range keys {
		if err := Sign(s, k); err != nil {
			return nil, err
		}

	}
	return s, nil
}
