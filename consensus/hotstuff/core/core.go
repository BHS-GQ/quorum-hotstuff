package core

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
)

type core struct {
	config *hs.Config
	logger log.Logger

	current *roundState
	backend hs.Backend
	signer  hs.Signer

	valSet      hs.ValidatorSet
	requests    *requestSet
	backlogs    *backlog
	expectedMsg []byte

	events            *event.TypeMuxSubscription
	timeoutSub        *event.TypeMuxSubscription
	finalCommittedSub *event.TypeMuxSubscription

	roundChangeTimer *time.Timer

	validateFn func([]byte, []byte) (common.Address, error)
}

// New creates an HotStuff consensus core
func New(backend hs.Backend, config *hs.Config, signer hs.Signer, valSet hs.ValidatorSet) CoreEngine {
	c := &core{
		config:   config,
		backend:  backend,
		valSet:   valSet,
		signer:   signer,
		logger:   log.New("address", backend.Address()),
		requests: newRequestSet(),
		backlogs: newBackLog(),
	}
	c.validateFn = c.checkValidatorSignature

	return c
}

const maxRetry uint64 = 10

func (c *core) startNewRound(round *big.Int) {
	logger := c.logger.New()

	var (
		changeView = false

		// Gets recently-chained block using chain reader
		lastProposal, lastProposer := c.backend.LastProposal()
	)

	// check last chained block
	if lastProposal == nil {
		logger.Warn("Last proposal should not be nil")
		return
	}

	if c.current == nil {
		logger.Trace("Starting the initial round")
	} else if lastProposal.NumberU64() >= c.HeightU64() {
		logger.Trace("Catch up latest proposal", "number", lastProposal.NumberU64(), "hash", lastProposal.Hash())
	} else if lastProposal.NumberU64() < c.HeightU64() - 1 {
		logger.Warn("New height should be larger than current height", "new_height", lastProposal.NumberU64)
		return
	} else if round.Sign() == 0 {
		logger.Trace("Latest proposal not chained", "chained", lastProposal.NumberU64(), "current", c.HeightU64())
		return
	} else if round.Uint64() < c.RoundU64() {
		logger.Warn("New round should not be smaller than current round", "height", lastProposal.NumberU64(), "new_round", round, "old_round", c.RoundU64())
		return
	} else {
		changeView = true
	}

	newView := &hs.View{
		Height: new(big.Int).Add(lastProposal.Number(), common.Big1),
		Round:  common.Big0,
	}
	if changeView {
		newView.Height = new(big.Int).Set(c.current.Height())
		newView.Round = new(big.Int).Set(round)
	}

	var (
		lastProposalLocked bool
		lastLockedProposal hs.Proposal
		lastPendingRequest *hs.Request
	)
	if c.current != nil {
		lastProposalLocked, lastLockedProposal = c.current.LastLockedProposal()
		lastPendingRequest = c.current.PendingRequest()
	}

	// calculate new proposal and init round state
	c.valSet.CalcProposer(lastProposer, newView.Round.Uint64())
	prepareQC := proposal2QC(lastProposal, common.Big0) // Do we need this? can't we just use c.current.PrepareQC().Copy()

	if err := updateRoundState(lastProposal, newView); err != nil {
		logger.Error("Update round state failed", "state", c.currentState(), "newView", newView, "err", err)
		return
	}
	if changeView && lastProposalLocked && lastLockedProposal != nil {
		c.current.SetProposal(lastLockedProposal)
		c.current.LockProposal()
	}
	if changeView && lastPendingRequest != nil {
		c.current.SetPendingRequest(lastPendingRequest)
	}

	logger.Debug("New round", "state", c.currentState(), "newView", newView, "new_proposer", c.valSet.GetProposer(), "valSet", c.valSet.List(), "size", c.valSet.Size(), "IsProposer", c.IsProposer())

	// process pending request
	c.setCurrentState(StateAcceptRequest)
	c.sendNewView(newView)

	// stop last timer and regenerate new timer
	c.newRoundChangeTimer()
}

func (c *core) updateRoundState(lastProposal *types.Block, newView *View) error {
	if c.current == nil {
		c.current = newRoundState(c.db, c.logger.New(), c.valSet, lastProposal, newView)
		c.current.reload(newView)
	} else {
		c.current = c.current.update(c.valSet, lastProposal, newView)
	}

	prepareQC := c.current.PrepareQC()
	if prepareQC != nil && prepareQC.node == lastProposal.Hash() {
		c.logger.Trace("EpochStartPrepareQC already exist!", "newView", newView, "last block height", lastProposal.NumberU64(), "last block hash", lastProposal.Hash(), "qc.node", prepareQC.node, "qc.view", prepareQC.view, "qc.proposer", prepareQC.proposer)
		return nil
	}

	qc, err := buildRoundStartQC(lastProposal)
	if err != nil {
		return err
	}
	if err := c.current.SetPrepareQC(qc); err != nil {
		return err
	}
	// clear old `prepareQC` and `commitQC`
	c.current.prepareQC = nil
	c.current.committedQC = nil

	// note that `lockQC` is not cleared

	c.logger.Trace("EpochStartPrepareQC settled!", "newView", newView, "last block height", lastProposal.NumberU64(), "last block hash", lastProposal.Hash(), "qc.node", qc.node, "qc.view", qc.view, "qc.proposer", qc.proposer)
	return nil
}

func (c *core) setCurrentState(s State) {
	c.current.SetState(s)
	c.processBacklog()
}

func (c *core) checkValidatorSignature(data []byte, sig []byte) (common.Address, error) {
	return c.signer.CheckSignature(c.valSet, data, sig)
}

func (c *core) Q() int {
	return c.valSet.Q()
}
