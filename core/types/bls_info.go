package types

import (
	"go.dedis.ch/kyber/v3/pairing/bn256"
	"go.dedis.ch/kyber/v3/share"
)

type BLSInfo struct {
	T int
	N int

	Suite      *bn256.Suite
	BLSPubPoly *share.PubPoly
	BLSPrivKey *share.PriShare
}
