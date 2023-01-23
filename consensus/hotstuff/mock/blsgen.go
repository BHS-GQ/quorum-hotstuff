package mock

import (
	"go.dedis.ch/kyber/v3/pairing/bn256"
	"go.dedis.ch/kyber/v3/share"
)

func GenerateBLSKeys(n, f int) (*bn256.Suite, *share.PubPoly, *share.PriPoly) {
	suite := bn256.NewSuite()
	secret := suite.G1().Scalar().Pick(suite.RandomStream())
	priPoly := share.NewPriPoly(suite.G2(), f+1, secret, suite.RandomStream()) // Private key.
	pubPoly := priPoly.Commit(suite.G2().Point().Base())                       // Common public key.

	return suite, pubPoly, priPoly
}
