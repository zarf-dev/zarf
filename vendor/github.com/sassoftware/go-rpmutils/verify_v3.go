//go:build pgp3

package rpmutils

import (
	"hash"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// Using a fork of PM that does parse V3 signatures, so check if that's what it is
func parseSignatureMaybeV3(genpkt packet.Packet, knownKeys openpgp.EntityList) (*Signature, error) {
	pkt, ok := genpkt.(*packet.SignatureV3)
	if !ok {
		return parseSignatureV4(genpkt, knownKeys)
	}
	sig := &Signature{
		Hash:         pkt.Hash,
		CreationTime: pkt.CreationTime,
		KeyId:        pkt.IssuerKeyId,
	}
	sig.validate = func(h hash.Hash) error {
		if knownKeys == nil {
			return nil
		}
		keys := knownKeys.KeysById(pkt.IssuerKeyId)
		if len(keys) == 0 || keys[0].PublicKey == nil {
			return KeyNotFoundError{KeyID: pkt.IssuerKeyId}
		}
		setEntity(sig, keys[0].Entity)
		return keys[0].PublicKey.VerifySignatureV3(h, pkt)
	}
	return sig, nil
}
