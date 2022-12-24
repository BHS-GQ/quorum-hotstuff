package hotstuff

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type Signer interface {
	Address() common.Address

	// AddAggPub adds new aggPub to local recording everytime the valset gets updated
	AddAggPub(valSet ValidatorSet, address common.Address, pubByte []byte) (int, error)

	// CountAggPub retrieves the size of current aggregated public key collection
	CountAggPub() int

	// AggregatedSignedFromSingle assigns value to msg.AggPub and msg.AggSign
	AggregatedSignedFromSingle(msg []byte) ([]byte, []byte, error)

	// AggregateSignature aggregates the signatures
	AggregateSignature(valSet ValidatorSet, collectionPub, collectionSig map[common.Address][]byte) ([]byte, []byte, []byte, error)

	// UpdateMask updates the state of the current mask
	UpdateMask(valSet ValidatorSet) error

	// Sign generate signature
	Sign(data []byte) ([]byte, error)

	// SigHash generate header hash without signature
	SealHash(header *types.Header) (hash common.Hash)

	// VerifyQC verify quorum cert
	VerifyQC(msg *Message, expectedMsg []byte, qc *QuorumCert, valSet ValidatorSet) error

	// CheckSignature extract address from signature and check if the address exist in validator set
	CheckSignature(valSet ValidatorSet, data []byte, signature []byte) (common.Address, error)

	// PrepareExtra returns a extra-data of the given header and validators, without `Seal` and `CommittedSeal`
	PrepareExtra(header *types.Header, valSet ValidatorSet) ([]byte, error)

	// Recover extracts the proposer address from a signed header.
	Recover(h *types.Header) (common.Address, error)

	// SealBeforeCommit writes the extra-data field of a block header with given seal.
	SealBeforeCommit(h *types.Header) error

	// // SignVote returns an signature of wrapped proposal hash which used as an vote
	// SignVote(proposal Proposal) ([]byte, error)

	// PrepareExtra(header *types.Header, valSet ValidatorSet) ([]byte, error)

	// // SealAfterCommit writes the extra-data field of a block header with given committed seals.
	// SealAfterCommit(h *types.Header, committedSeals [][]byte) error

	// // VerifyHeader verify proposer signature and committed seals
	// VerifyHeader(header *types.Header, valSet ValidatorSet, seal bool) error

	// // CheckQCParticipant return nil if `signer` is qc proposer or committer
	// CheckQCParticipant(qc *QuorumCert, signer common.Address) error
}
