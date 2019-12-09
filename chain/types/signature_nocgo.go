//+build !cgo

package types

import (
	"fmt"

	"github.com/filecoin-project/lotus/chain/address"
)

func (s *Signature) Verify(addr address.Address, msg []byte) error {
	if addr.Protocol() == address.ID {
		return fmt.Errorf("must resolve ID addresses before using them to verify a signature")
	}
	log.Warnf("signature verification not supported without cgo")

	switch s.Type {
	case KTSecp256k1:
		return fmt.Errorf("signature did not match")
	case KTBLS:
		return fmt.Errorf("bls signature failed to verify")
	default:
		return fmt.Errorf("cannot verify signature of unsupported type: %s", s.Type)
	}
}
