package core

import (
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/prque"
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
)

type core struct {
	db     ethdb.Database
	config *hs.Config
	logger log.Logger

	current *roundState
	backend hs.Backend
	signer  hs.Signer

	valSet      hs.ValidatorSet
	backlogs    *backlog
	expectedMsg []byte

	events            *event.TypeMuxSubscription
	timeoutSub        *event.TypeMuxSubscription
	finalCommittedSub *event.TypeMuxSubscription

	roundChangeTimer *time.Timer

	pendingRequests   *prque.Prque
	pendingRequestsMu *sync.Mutex

	validateFn func([]byte, []byte) (common.Address, error)
	isRunning  bool
}

// New creates an HotStuff consensus core
func New(backend hs.Backend, config *hs.Config, signer hs.Signer, db ethdb.Database, valSet hs.ValidatorSet) *core {
	c := &core{
		db:                db,
		config:            config,
		backend:           backend,
		valSet:            valSet,
		signer:            signer,
		logger:            log.New("address", backend.Address()),
		backlogs:          newBackLog(),
		pendingRequests:   prque.New(nil),
		pendingRequestsMu: new(sync.Mutex),
	}
	c.validateFn = c.checkValidatorSignature

	return c
}

func (c *core) startNewRound(round *big.Int) {
	logger := c.logger.New()

	if !c.isRunning {
		logger.Trace("Start engine first")
		return
	}

	var (
		changeView = false

		// Gets recently-chained block using chain reader
		lastProposal, lastProposer = c.backend.LastProposal() // [TODO]
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
	} else if lastProposal.NumberU64() < c.HeightU64()-1 {
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

	// calculate new proposal and init round state
	c.valSet.CalcProposer(lastProposer, newView.Round.Uint64())

	// update smr and try to unlock at the round0
	if err := c.updateRoundState(lastProposal, newView); err != nil {
		logger.Error("Update round state failed", "state", c.currentState(), "newView", newView, "err", err)
		return
	}
	if !changeView {
		if err := c.current.Unlock(); err != nil {
			logger.Error("Unlock node failed", "newView", newView, "err", err)
			return
		}
	}

	logger.Debug("New round", "state", c.currentState(), "newView", newView, "new_proposer", c.valSet.GetProposer(), "valSet", c.valSet.List(), "size", c.valSet.Size(), "IsProposer", c.IsProposer())

	// stop last timer and regenerate new timer
	c.newRoundChangeTimer()

	// process pending request
	c.setCurrentState(hs.StateAcceptRequest)
	// start new round from message of `newView`
	c.sendNewView()

}

func (c *core) updateRoundState(lastProposal *types.Block, newView *hs.View) error {
	if c.current == nil {
		c.current = newRoundState(c.db, c.logger.New(), c.valSet, lastProposal, newView)
		c.current.reload(newView)
	} else {
		c.current = c.current.update(c.valSet, lastProposal, newView)
	}

	prepareQC := c.current.PrepareQC()
	if prepareQC != nil && prepareQC.TreeNode == lastProposal.Hash() {
		c.logger.Trace("EpochStartPrepareQC already exist!", "newView", newView, "last block height", lastProposal.NumberU64(), "last block hash", lastProposal.Hash(), "qc.node", prepareQC.TreeNode, "qc.view", prepareQC.View, "qc.proposer", prepareQC.Proposer)
		return nil
	}

	qc, err := buildRoundStartQC(lastProposal)
	if err != nil {
		return err
	}
	if err := c.current.SetPrepareQC(qc); err != nil {
		return err
	}

	// clear old `lockQC` and `commitQC`
	c.current.lockQC = nil
	c.current.committedQC = nil

	c.logger.Trace("EpochStartPrepareQC settled!", "newView", newView, "last block height", lastProposal.NumberU64(), "last block hash", lastProposal.Hash(), "qc.node", qc.TreeNode, "qc.view", qc.View, "qc.proposer", qc.Proposer)
	return nil
}

// setCurrentState handle backlog message after round state settled.
func (c *core) setCurrentState(s hs.State) {
	c.current.SetState(s)
	if s == hs.StateAcceptRequest || s == hs.StateHighQC {
		c.processPendingRequests()
	}
	c.processBacklog()
}

func (c *core) checkValidatorSignature(data []byte, sig []byte) (common.Address, error) {
	return c.signer.CheckSignature(c.valSet, data, sig)
}
