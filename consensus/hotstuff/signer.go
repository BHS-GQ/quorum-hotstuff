package hotstuff

import (
	"github.com/ethereum/go-ethereum/common"
	"go.dedis.ch/kyber/v3/pairing/bn256"
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

	// SetAggInfo assigns new keypair for unit testing
	SetAggInfo(unitTest bool, suite *bn256.Suite)

	// Sign generate signature
	Sign(data []byte) ([]byte, error)

	// // SigHash generate header hash without signature
	// SigHash(header *types.Header) (hash common.Hash)

	// // SignVote returns an signature of wrapped proposal hash which used as an vote
	// SignVote(proposal Proposal) ([]byte, error)

	// // Recover extracts the proposer address from a signed header.
	// Recover(h *types.Header) (common.Address, error)

	// // PrepareExtra returns a extra-data of the given header and validators, without `Seal` and `CommittedSeal`
	// PrepareExtra(header *types.Header, valSet ValidatorSet) ([]byte, error)

	// // SealBeforeCommit writes the extra-data field of a block header with given seal.
	// SealBeforeCommit(h *types.Header) error

	// // SealAfterCommit writes the extra-data field of a block header with given committed seals.
	// SealAfterCommit(h *types.Header, committedSeals [][]byte) error

	// // VerifyHeader verify proposer signature and committed seals
	// VerifyHeader(header *types.Header, valSet ValidatorSet, seal bool) error

	// // VerifyQC verify quorum cert
	// VerifyQC(qc *QuorumCert, valSet ValidatorSet) error

	// // CheckQCParticipant return nil if `signer` is qc proposer or committer
	// CheckQCParticipant(qc *QuorumCert, signer common.Address) error

	// // CheckSignature extract address from signature and check if the address exist in validator set
	// CheckSignature(valSet ValidatorSet, data []byte, signature []byte) (common.Address, error)
}
