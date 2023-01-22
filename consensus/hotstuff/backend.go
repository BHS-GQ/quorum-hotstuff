package hotstuff

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
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
	Commit(executed *consensus.ExecutedBlock) error

	// Verify verifies the proposal. If a consensus.ErrFutureBlock error is returned,
	// the time difference of the proposal and current time is also returned.
	Verify(*types.Block) (time.Duration, error)

	// LastProposal retrieves latest committed proposal and the address of proposer
	LastProposal() (*types.Block, common.Address)

	// HasProposal checks if the combination of the given hash and height matches any existing blocks
	HasProposal(hash common.Hash, number *big.Int) bool

	// GetProposer returns the proposer of the given block height
	GetProposer(number uint64) common.Address

	// HasBadBlock returns whether the block with the hash is a bad block
	HasBadProposal(hash common.Hash) bool

	// ExecuteBlock execute block which contained in prepare message, and validate block state
	ExecuteBlock(block *types.Block) (*consensus.ExecutedBlock, error)

	SealBlock(block *types.Block, prepareQC *QuorumCert) (*types.Block, error)

	Close() error
}

type CoreEngine interface {
	Start() error

	Stop() error

	// IsProposer return true if self address equal leader/proposer address in current round/height
	IsProposer() bool

	// CurrentSequence return current proposal height and consensus round
	CurrentSequence() (uint64, uint64)

	// verify if a hash is the same as the proposed block in the current pending request
	//
	// this is useful when the engine is currently the speaker
	//
	// pending request is populated right at the request stage so this would give us the earliest verification
	// to avoid any race condition of coming propagated blocks
	IsCurrentProposal(blockHash common.Hash) bool
}

type HotstuffProtocol string

const (
	HOTSTUFF_PROTOCOL_BASIC        HotstuffProtocol = "basic"
	HOTSTUFF_PROTOCOL_EVENT_DRIVEN HotstuffProtocol = "event_driven"
)
