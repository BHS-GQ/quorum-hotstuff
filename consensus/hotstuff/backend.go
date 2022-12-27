package hotstuff

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
)

// Backend provides application specific functions for HotStuff core
type Backend interface {
	// Address returns the owner's address
	Address() common.Address

	// Validators returns the validator set
	Validators() ValidatorSet

	// EventMux returns the event mux in backend
	EventMux() *event.TypeMux

	// Broadcast sends a message to all validators (include self)
	Broadcast(valSet ValidatorSet, payload []byte) error

	// Gossip sends a message to all validators (exclude self)
	Gossip(valSet ValidatorSet, payload []byte) error

	// Unicast send a message to single peer
	Unicast(valSet ValidatorSet, payload []byte) error

	// Commit delivers an approved proposal to backend.
	// The delivered proposal will be put into blockchain.
	Commit(proposal Proposal) error

	// Verify verifies the proposal. If a consensus.ErrFutureBlock error is returned,
	// the time difference of the proposal and current time is also returned.
	Verify(Proposal) (time.Duration, error)

	// LastProposal retrieves latest committed proposal and the address of proposer
	LastProposal() (Proposal, common.Address)

	// HasProposal checks if the combination of the given hash and height matches any existing blocks
	HasProposal(hash common.Hash, number *big.Int) bool

	// GetProposer returns the proposer of the given block height
	GetProposer(number uint64) common.Address

	// ParentValidators returns the validator set of the given proposal's parent block
	ParentValidators(proposal Proposal) ValidatorSet

	// HasBadBlock returns whether the block with the hash is a bad block
	HasBadProposal(hash common.Hash) bool

	Close() error
}
