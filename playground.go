package main

import (
	"fmt"

	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/pairing/bn256"
	"go.dedis.ch/kyber/v3/sign/bdn"
	"go.dedis.ch/kyber/v3/util/random"
)

var suite = pairing.NewSuiteBn256()

// func main() {
// 	msg := []byte("Hello Boneh-Lynn-Shacham")
// 	suite := bn256.NewSuite()
// 	private1, public1 := bdn.NewKeyPair(suite, random.New())
// 	private2, public2 := bdn.NewKeyPair(suite2, random.New())
// 	sig1, err := bdn.Sign(suite, private1, msg)
// 	require.NoError(t, err)
// 	sig2, err := bdn.Sign(suite, private2, msg)
// 	require.NoError(t, err)

// 	mask, _ := sign.NewMask(suite, []kyber.Point{public1, public2}, nil)
// 	mask.SetBit(0, true)
// 	mask.SetBit(1, true)

// 	_, err = bdn.AggregateSignatures(suite4, [][]byte{sig1}, mask)
// 	require.Error(t, err)

// 	aggregatedSig, err := bdn.AggregateSignatures(suite5, [][]byte{sig1, sig2}, mask)
// 	require.NoError(t, err)

// 	aggregatedKey, err := bdn.AggregatePublicKeys(suite4, mask)

// 	sig, err := aggregatedSig.MarshalBinary()
// 	require.NoError(t, err)

// 	err = bdn.Verify(suite, aggregatedKey, msg, sig)
// 	require.NoError(t, err)

// 	mask.SetBit(1, false)
// 	aggregatedKey, err = bdn.AggregatePublicKeys(suite, mask)

// 	err = bdn.Verify(suite5, aggregatedKey, msg, sig)
// 	require.Error(t, err)
// }
func main() {
	// msg := []byte("Hello Boneh-Lynn-Shacham")
	suite := bn256.NewSuite()
	private1, public1 := bdn.NewKeyPair(suite, random.New())
	// private2, public2 := bdn.NewKeyPair(suite, random.New())

	priv1bytes, _ := private1.MarshalBinary()
	fmt.Printf("0x%x\n", priv1bytes)
	pub1bytes, _ := public1.MarshalBinary()
	fmt.Printf("0x%x\n", pub1bytes)
}
