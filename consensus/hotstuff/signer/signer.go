package signer

import (
	"bytes"
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/hotstuff"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	lru "github.com/hashicorp/golang-lru"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/pairing/bn256"
	"go.dedis.ch/kyber/v3/sign"
	"go.dedis.ch/kyber/v3/sign/bdn"
	"go.dedis.ch/kyber/v3/util/random"
	"golang.org/x/crypto/sha3"
)

const (
	inmemorySignatures = 4096 // Number of recent block signatures to keep in memory
)

type SignerImpl struct {
	address       common.Address
	privateKey    *ecdsa.PrivateKey
	signatures    *lru.ARCCache // Signatures of recent blocks to speed up mining
	commitSigSalt byte          //

	logger log.Logger

	// BLS Upgrade - aggregated signature
	suite             *bn256.Suite // From config
	aggregatedPub     kyber.Point
	aggregatedPrv     kyber.Scalar
	aggregatedKeyPair map[common.Address]kyber.Point // map[address] -> pub
	participants      int
	mask              *sign.Mask // update whenever the size of aggregatedKeyPair increases
	// /BLS Upgrade
}

func NewSigner(privateKey *ecdsa.PrivateKey, commitMsgType byte, configSuite *bn256.Suite) hotstuff.Signer {
	signatures, _ := lru.NewARC(inmemorySignatures)
	address := crypto.PubkeyToAddress(privateKey.PublicKey)
	aggregatedPrv, aggregatedPub := bdn.NewKeyPair(configSuite, random.New())
	return &SignerImpl{
		address:           address,
		privateKey:        privateKey,
		signatures:        signatures,
		commitSigSalt:     commitMsgType,
		suite:             configSuite,
		aggregatedPrv:     aggregatedPrv,
		aggregatedPub:     aggregatedPub,
		aggregatedKeyPair: make(map[common.Address]kyber.Point),
		logger:            log.New(),
	}
}

func (s *SignerImpl) Address() common.Address {
	return s.address
}

func (s *SignerImpl) AddAggPub(valSet hotstuff.ValidatorSet, address common.Address, pubByte []byte) (int, error) {
	pub := s.suite.G2().Point()
	if err := pub.UnmarshalBinary(pubByte); err != nil {
		return -1, err
	}
	if _, exist := s.aggregatedKeyPair[address]; !exist {
		s.aggregatedKeyPair[address] = pub
		_, ok := valSet.GetByAddress(address)
		if ok == nil {
			s.logger.Trace("Address not in validators set, backing up", "address", address)
		} else {
			s.logger.Trace("Address in validators set", "address", address)
			s.participants += 1
		}
	}

	return s.participants, nil
}

func (s *SignerImpl) CountAggPub() int {
	return s.participants
}

func (s *SignerImpl) AggregatedSignedFromSingle(msg []byte) ([]byte, []byte, error) {
	if s.aggregatedPub == nil || s.aggregatedPrv == nil {
		return nil, nil, errIncorrectAggInfo
	}
	pubByte, err := s.aggregatedPub.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}
	sig, err := bdn.Sign(s.suite, s.aggregatedPrv, msg)
	if err != nil {
		return nil, nil, err
	}
	return pubByte, sig, nil
}

func (s *SignerImpl) Sign(data []byte) ([]byte, error) {
	hashData := crypto.Keccak256(data)
	return crypto.Sign(hashData, s.privateKey)
}

func (s *SignerImpl) AggregateSignature(valSet hotstuff.ValidatorSet, collectionPub, collectionSig map[common.Address][]byte) ([]byte, []byte, []byte, error) {
	s.logger.Info("AggregateSignature")
	if err := s.collectSignature(valSet, collectionPub); err != nil {
		return nil, nil, nil, err
	}
	if err := s.setBitForMask(collectionPub); err != nil {
		return nil, nil, nil, err
	}
	aggSig, err := s.aggregateSignatures(collectionSig)
	if err != nil {
		return nil, nil, nil, err
	}
	aggKey, err := s.aggregateKeys()
	if err != nil {
		return nil, nil, nil, err
	}
	if len(s.mask.Mask()) != (valSet.Size()+7)/8 {
		// This shouldn't happen because the process stops due to the state not set to StateAcceptRequest yet
		return nil, nil, nil, errInsufficientAggPub
	}
	return s.mask.Mask(), aggSig, aggKey, nil
}

func (s *SignerImpl) collectSignature(valSet hotstuff.ValidatorSet, collection map[common.Address][]byte) error {
	for addr, pubByte := range collection {
		if addr == s.Address() {
			continue
			// return errInvalidProposalMyself
		}
		pub := s.suite.G2().Point()
		if err := pub.UnmarshalBinary(pubByte); err != nil {
			return err
		}
		if _, exist := s.aggregatedKeyPair[addr]; !exist {
			s.aggregatedKeyPair[addr] = pub
			s.participants += 1
		}
	}
	// Update the mask anyway, reset the bit
	if err := s.UpdateMask(valSet); err != nil {
		return err
	}
	return nil
}

func (s *SignerImpl) UpdateMask(valSet hotstuff.ValidatorSet) error {
	s.logger.Info("UpdateMask")
	convert := func(keyPair map[common.Address]kyber.Point) []kyber.Point {
		keyPairSlice := make([]kyber.Point, 0, 100)
		for addr, pub := range keyPair {
			if _, val := valSet.GetByAddress(addr); val != nil {
				s.logger.Trace("Found addr", addr)
				keyPairSlice = append(keyPairSlice, pub)
			}
		}
		return keyPairSlice
	}

	var err error
	filteredList := convert(s.aggregatedKeyPair)
	if len(filteredList) != valSet.F() {
		// This shouldn't happen because the process stops due to the state not set to StateAcceptRequest yet
		return errInsufficientAggPub
	}
	s.mask, err = sign.NewMask(s.suite, filteredList, nil)
	if err != nil {
		return err
	}

	return nil
}

func (s *SignerImpl) setBitForMask(collection map[common.Address][]byte) error {
	for _, pubByte := range collection {
		pub := s.suite.G2().Point()
		if err := pub.UnmarshalBinary(pubByte); err != nil {
			return err
		}
		for i, key := range s.mask.Publics() {
			if key.Equal(pub) {
				s.mask.SetBit(i, true)
			}
		}
	}
	return nil
}

func (s *SignerImpl) aggregateSignatures(collection map[common.Address][]byte) ([]byte, error) {
	sigs := make([][]byte, len(collection))
	i := 0
	for _, sig := range collection {
		sigs[i] = make([]byte, types.HotStuffExtraAggSig)
		copy(sigs[i][:], sig)
		i += 1
	}
	if len(sigs) != len(collection) {
		return nil, errTestIncorrectConversion
	}

	aggregatedSig, err := bdn.AggregateSignatures(s.suite, sigs, s.mask)
	if err != nil {
		return nil, err
	}
	aggregatedSigByte, err := aggregatedSig.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return aggregatedSigByte, nil
}

func (s *SignerImpl) aggregateKeys() ([]byte, error) {
	aggKey, err := bdn.AggregatePublicKeys(s.suite, s.mask)
	if err != nil {
		return nil, err
	}
	aggKeyByte, err := aggKey.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return aggKeyByte, nil
}

// Note, the method requires the extra data to be at least 65 bytes, otherwise it
// panics. This is done to avoid accidentally using both forms (signature present
// or not), which could be abused to produce different hashes for the same header.
func (s *SignerImpl) SealHash(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewLegacyKeccak256()

	// Clean seal is required for calculating proposer seal.
	rlp.Encode(hasher, types.HotstuffFilteredHeader(header, false))
	hasher.Sum(hash[:0])
	return hash
}

// Recover extracts the proposer address from a signed header.
func (s *SignerImpl) Recover(header *types.Header) (common.Address, error) {
	hash := header.Hash()
	if s.signatures != nil {
		if addr, ok := s.signatures.Get(hash); ok {
			return addr.(common.Address), nil
		}
	}

	// Retrieve the signature from the header extra-data
	extra, err := types.ExtractHotstuffExtra(header)
	if err != nil {
		return common.Address{}, errInvalidExtraDataFormat
	}

	payload := s.SealHash(header).Bytes()
	addr, err := getSignatureAddress(payload, extra.Seal)
	if err != nil {
		return addr, err
	}

	if s.signatures != nil {
		s.signatures.Add(hash, addr)
	}
	return addr, nil
}

// SignerSeal proposer sign the header hash and fill extra seal with signature.
func (s *SignerImpl) SealBeforeCommit(h *types.Header) error {
	sealHash := s.SealHash(h)
	seal, err := s.Sign(sealHash.Bytes())
	if err != nil {
		return errInvalidSignature
	}

	if len(seal)%types.HotstuffExtraSeal != 0 {
		return errInvalidSignature
	}

	extra, err := types.ExtractHotstuffExtra(h)
	if err != nil {
		return err
	}
	extra.Seal = seal
	payload, err := rlp.EncodeToBytes(&extra)
	if err != nil {
		return err
	}
	h.Extra = append(h.Extra[:types.HotstuffExtraVanity], payload...)
	return nil
}

func (s *SignerImpl) VerifyQC(
	msg *hotstuff.Message,
	expectedMsg []byte,
	qc *hotstuff.QuorumCert,
	valSet hotstuff.ValidatorSet,
) error {
	if qc.View.Height.Uint64() == 0 {
		return nil
	}
	extra, err := types.ExtractHotstuffExtraPayload(qc.Extra)
	if err != nil {
		return err
	}

	// check proposer signature
	addr, err := getSignatureAddress(qc.Hash.Bytes(), extra.Seal)
	if err != nil {
		return err
	}
	if addr != qc.Proposer {
		return errInvalidSigner
	}
	if idx, _ := valSet.GetByAddress(addr); idx < 0 {
		return errInvalidSigner
	}

	// check aggsigs
	if err := s.verifySig(expectedMsg, msg.AggPub, msg.AggSign); err != nil {
		return err
	}

	if err := s.verifyMask(valSet, extra.Mask); err != nil {
		return err
	}

	return nil
}

func (s *SignerImpl) verifySig(expectedMsg []byte, aggKeyByte, aggSigByte []byte) error {
	// UnmarshalBinary aggKeyByte to kyber.Point
	aggKey := s.suite.G2().Point()
	if err := aggKey.UnmarshalBinary(aggKeyByte); err != nil {
		return err
	}

	// Regenerate the *message
	// [TODO] find way to get origm
	if err := bdn.Verify(s.suite, aggKey, expectedMsg, aggSigByte); err != nil {
		return err
	}
	return nil
}

func (s *SignerImpl) verifyMask(valSet hotstuff.ValidatorSet, mask []byte) error {
	s.logger.Info("verifyMask")

	if len(mask) != (valSet.Size()+7)/8 {
		return errInsufficientAggPub
	}

	count := 0
	for i := range valSet.List() {
		byteIndex := i / 8
		m := byte(1) << uint(i&7)
		if (mask[byteIndex] & m) != 0 {
			count++
		}
	}
	// This excludes the speaker
	if count < valSet.F() {
		return errInvalidAggregatedSig
	}
	return nil
}

// GetSignatureAddress gets the address address from the signature
func (s *SignerImpl) CheckSignature(valSet hotstuff.ValidatorSet, data []byte, sig []byte) (common.Address, error) {
	// 1. Get signature address
	signer, err := getSignatureAddress(data, sig)
	if err != nil {
		return common.Address{}, err
	}

	// 2. Check validator
	if _, val := valSet.GetByAddress(signer); val != nil {
		return val.Address(), nil
	}

	return common.Address{}, errUnauthorizedAddress
}

func (s *SignerImpl) PrepareExtra(header *types.Header, valSet hotstuff.ValidatorSet) ([]byte, error) {
	var (
		buf  bytes.Buffer
		vals = valSet.AddressList()
	)

	// compensate the lack bytes if header.Extra is not enough IstanbulExtraVanity bytes.
	if len(header.Extra) < types.HotstuffExtraVanity {
		header.Extra = append(header.Extra, bytes.Repeat([]byte{0x00}, types.HotstuffExtraVanity-len(header.Extra))...)
	}
	buf.Write(header.Extra[:types.HotstuffExtraVanity])

	ist := &types.HotstuffExtra{
		Validators: vals,
		Seal:       []byte{},
	}

	payload, err := rlp.EncodeToBytes(&ist)
	if err != nil {
		return nil, err
	}

	return append(buf.Bytes(), payload...), nil
}

func getSignatureAddress(data []byte, sig []byte) (common.Address, error) {
	// 1. Keccak data
	hashData := crypto.Keccak256(data)
	// 2. Recover public key
	pubkey, err := crypto.SigToPub(hashData, sig)
	if err != nil {
		return common.Address{}, err
	}
	return crypto.PubkeyToAddress(*pubkey), nil
}
