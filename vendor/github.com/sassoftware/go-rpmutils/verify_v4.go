package rpmutils

import (
	"bytes"
	"hash"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

func parseSignature(blob []byte, knownKeys openpgp.EntityList) (*Signature, error) {
	reader := bytes.NewReader(blob)
	genpkt, err := packet.Read(reader)
	if err != nil {
		return nil, err
	} else if reader.Len() > 0 {
		return nil, ErrTrailingGarbage
	}
	return parseSignatureMaybeV3(genpkt, knownKeys)
}

func parseSignatureV4(genpkt packet.Packet, knownKeys openpgp.EntityList) (*Signature, error) {
	pkt, ok := genpkt.(*packet.Signature)
	if !ok || (pkt.IssuerKeyId == nil && len(pkt.IssuerFingerprint) == 0) {
		return nil, ErrNoPGPSignature
	}
	sig := &Signature{
		Hash:           pkt.Hash,
		CreationTime:   pkt.CreationTime,
		KeyFingerprint: pkt.IssuerFingerprint,
	}
	if pkt.IssuerKeyId != nil {
		sig.KeyId = *pkt.IssuerKeyId
	}
	sig.validate = func(h hash.Hash) error {
		if knownKeys == nil {
			return nil
		}
		if entity, key := findKey(pkt, knownKeys); key != nil {
			setEntity(sig, entity)
			return key.VerifySignature(h, pkt)
		}
		return KeyNotFoundError{
			KeyID:       sig.KeyId,
			Fingerprint: sig.KeyFingerprint,
		}
	}
	return sig, nil
}

// set identity attributes on signature
func setEntity(sig *Signature, entity *openpgp.Entity) {
	sig.Signer = entity
	if sig.KeyId == 0 {
		sig.KeyId = entity.PrimaryKey.KeyId
	}
	if sig.KeyFingerprint == nil {
		sig.KeyFingerprint = entity.PrimaryKey.Fingerprint
	}
	if identity := entity.PrimaryIdentity(); identity != nil {
		sig.PrimaryName = identity.Name
	}
}

// find a key by its fingerprint (if available) or key ID
func findKey(sig *packet.Signature, knownKeys openpgp.EntityList) (*openpgp.Entity, *packet.PublicKey) {
	for _, entity := range knownKeys {
		if sig.CheckKeyIdOrFingerprint(entity.PrimaryKey) {
			return entity, entity.PrimaryKey
		}
		for _, sub := range entity.Subkeys {
			if sig.CheckKeyIdOrFingerprint(sub.PublicKey) {
				return entity, sub.PublicKey
			}
		}
	}
	return nil, nil
}
