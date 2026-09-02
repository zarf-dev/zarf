//go:build !pgp3

package rpmutils

import (
	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

func parseSignatureMaybeV3(genpkt packet.Packet, knownKeys openpgp.EntityList) (*Signature, error) {
	return parseSignatureV4(genpkt, knownKeys)
}
