package core

import (
	"bytes"
	"crypto/ecdsa"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	lru "github.com/hashicorp/golang-lru"
	"go.dedis.ch/kyber/v3/pairing/bn256"
	"go.dedis.ch/kyber/v3/share"
	"go.dedis.ch/kyber/v3/sign/bls"
	"go.dedis.ch/kyber/v3/sign/tbls"
	"golang.org/x/crypto/sha3"
)

const (
	inmemorySignatures = 4096 // Number of recent block signatures to keep in memory
)

type HotstuffSigner struct {
	address       common.Address
	privateKey    *ecdsa.PrivateKey
	signatures    *lru.ARCCache // Signatures of recent blocks to speed up mining
	commitSigSalt byte          //

	logger log.Logger

	// BLS Upgrade - aggregated signature
	suite      *bn256.Suite // From config
	blsPubPoly *share.PubPoly
	blsPrivKey *share.PriShare
	t          int
	n          int
	// /BLS Upgrade
}

func NewSigner(
	privateKey *ecdsa.PrivateKey,
	commitMsgType byte,
	blsInfo *types.BLSInfo,
) hs.Signer {
	signatures, _ := lru.NewARC(inmemorySignatures)
	address := crypto.PubkeyToAddress(privateKey.PublicKey)
	return &HotstuffSigner{
		address:       address,
		privateKey:    privateKey,
		signatures:    signatures,
		commitSigSalt: commitMsgType,
		suite:         blsInfo.Suite,
		logger:        log.New(),
		blsPubPoly:    blsInfo.BLSPubPoly,
		blsPrivKey:    blsInfo.BLSPrivKey,
		t:             blsInfo.T,
		n:             blsInfo.N,
	}
}

func (s *HotstuffSigner) Address() common.Address {
	return s.address
}

// BLS section
func (s *HotstuffSigner) BLSSign(data []byte) ([]byte, error) {
	signed_data, err := tbls.Sign(s.suite, s.blsPrivKey, data)
	if err != nil {
		return nil, err
	}
	return signed_data, nil
}

func (s *HotstuffSigner) BLSRecoverAggSig(data []byte, sigShares [][]byte) ([]byte, error) {
	aggSig, err := tbls.Recover(s.suite, s.blsPubPoly, data, sigShares, s.t, s.n)
	if err != nil {
		return nil, err
	}
	return aggSig, nil
}

func (s *HotstuffSigner) BLSVerifyAggSig(data []byte, aggSig []byte) error {
	err := bls.Verify(s.suite, s.blsPubPoly.Commit(), data, aggSig)
	if err != nil {
		return err
	}
	return nil
}

func (s *HotstuffSigner) VerifyQC(qc *hs.QuorumCert) error {
	// skip genesis block
	if qc.View.Height.Uint64() == 0 {
		return nil
	}

	// check proposer signature
	data, _ := hs.Encode(&hs.Vote{
		Code:    qc.Code,
		View:    qc.View,
		CmdNode: qc.CmdNode,
	})
	if err := s.BLSVerifyAggSig(data, qc.BLSSignature); err != nil {
		return err
	}

	return nil
}

func (s *HotstuffSigner) VerifyHeader(header *types.Header, valSet hs.ValidatorSet, seal bool) error {
	// Verifying the genesis block is not supported
	number := header.Number.Uint64()
	if number == 0 {
		return nil
	}

	// resolve the authorization key and check against signers
	signer, err := s.RecoverSigner(header)
	if err != nil {
		return err
	}
	if signer != header.Coinbase {
		return hs.ErrInvalidSigner
	}

	// Signer should be in the validator set of previous block's extraData.
	if _, v := valSet.GetByAddress(signer); v == nil {
		return hs.ErrUnauthorized
	}

	if seal {
		extra, err := types.ExtractHotstuffExtra(header)
		if err != nil {
			return hs.ErrInvalidExtraDataFormat
		}
		encodedQC := extra.EncodedQC

		var commitQC *hs.QuorumCert
		if err := rlp.DecodeBytes(encodedQC, &commitQC); err != nil {
			s.logger.Trace("Failed to decode", "err", err)
			return err
		}

		// Check CommitQC delivered via header
		if err := s.VerifyQC(commitQC); err != nil {
			s.logger.Trace("Failed to verify QC in header", "err", err)
			return err
		}
	}

	return nil
}

// VVV Not BLS related section VVV

func (s *HotstuffSigner) Sign(hash common.Hash) ([]byte, error) {
	if hash == hs.EmptyHash {
		return nil, hs.ErrInvalidRawHash
	}
	if s.privateKey == nil {
		return nil, hs.ErrInvalidSigner
	}

	return crypto.Sign(hash.Bytes(), s.privateKey)
}

// Note, the method requires the extra data to be at least 65 bytes, otherwise it
// panics. This is done to avoid accidentally using both forms (signature present
// or not), which could be abused to produce different hashes for the same header.
func (s *HotstuffSigner) HeaderHash(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewLegacyKeccak256()

	// Clean seal is required for calculating proposer seal.
	rlp.Encode(hasher, types.HotstuffFilteredHeader(header, false))
	hasher.Sum(hash[:0])
	return hash
}

// RecoverSigner extracts the proposer address from a signed header.
func (s *HotstuffSigner) RecoverSigner(header *types.Header) (common.Address, error) {
	hash := header.Hash()
	if s.signatures != nil {
		if addr, ok := s.signatures.Get(hash); ok {
			return addr.(common.Address), nil
		}
	}

	// Retrieve the signature from the header extra-data
	extra, err := types.ExtractHotstuffExtra(header)
	if err != nil {
		return common.Address{}, hs.ErrInvalidExtraDataFormat
	}

	headerHash := s.HeaderHash(header)
	addr, err := getSignatureAddress(headerHash, extra.Seal)
	if err != nil {
		return addr, err
	}

	if s.signatures != nil {
		s.signatures.Add(hash, addr)
	}
	return addr, nil
}

// SignerSeal proposer sign the header hash and fill extra seal with signature.
func (s *HotstuffSigner) SignerSeal(h *types.Header) error {
	HeaderHash := s.HeaderHash(h)
	seal, err := s.Sign(HeaderHash)
	if err != nil {
		return hs.ErrInvalidSignature
	}

	if len(seal)%types.HotstuffExtraSeal != 0 {
		return hs.ErrInvalidSignature
	}

	if err := h.SetSeal(seal); err != nil {
		return err
	}

	return nil
}

// GetSignatureAddress gets the address address from the signature
func (s *HotstuffSigner) CheckSignature(valSet hs.ValidatorSet, hash common.Hash, sig []byte) (common.Address, error) {
	if valSet == nil {
		return hs.EmptyAddress, fmt.Errorf("invalid ValidatorSet")
	}
	if hash == hs.EmptyHash {
		return hs.EmptyAddress, hs.ErrInvalidRawHash
	}
	if sig == nil {
		return hs.EmptyAddress, hs.ErrInvalidSignature
	}

	// 1. Get signature address
	signer, err := getSignatureAddress(hash, sig)
	if err != nil {
		return common.Address{}, err
	}

	// 2. Check validator
	if _, val := valSet.GetByAddress(signer); val != nil {
		return val.Address(), nil
	}

	return common.Address{}, hs.ErrUnauthorizedAddress
}

func (s *HotstuffSigner) BuildPrepareExtra(header *types.Header, valSet hs.ValidatorSet) ([]byte, error) {
	var (
		buf  bytes.Buffer
		vals = valSet.AddressList()
	)

	// compensate the lack bytes if header.Extra is not enough IstanbulExtraVanity bytes.
	if len(header.Extra) < types.HotstuffExtraVanity {
		header.Extra = append(header.Extra, bytes.Repeat([]byte{0x00}, types.HotstuffExtraVanity-len(header.Extra))...)
	}
	buf.Write(header.Extra[:types.HotstuffExtraVanity])

	// [TODO] Consider explicitly adding other fields
	ist := &types.HotstuffExtra{
		Validators: vals,
		Seal:       []byte{},
	}

	payload, err := hs.Encode(&ist)
	if err != nil {
		return nil, err
	}

	return append(buf.Bytes(), payload...), nil
}

func getSignatureAddress(hash common.Hash, sig []byte) (common.Address, error) {
	if hash == hs.EmptyHash {
		return hs.EmptyAddress, hs.ErrInvalidRawHash
	}
	if sig == nil {
		return hs.EmptyAddress, hs.ErrInvalidSignature
	}

	// 2. Recover public key
	pubkey, err := crypto.SigToPub(hash.Bytes(), sig)
	if err != nil {
		return common.Address{}, err
	}
	return crypto.PubkeyToAddress(*pubkey), nil
}
