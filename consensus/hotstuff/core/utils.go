package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

func (c *core) proposer() common.Address {
	return c.valSet.GetProposer().Address()
}

func (c *core) HeightU64() uint64 {
	if c.current == nil {
		return 0
	} else {
		return c.current.HeightU64()
	}
}

// checkView checks the Message sequence remote msg view should not be nil(local view WONT be nil).
// if the view is ahead of current view we name the Message to be future Message, and if the view is behind
// of current view, we name it as old Message. `old Message` and `invalid Message` will be dropped. and we use t
// he storage of `backlog` to cache the future Message, it only allow the Message height not bigger than
// `current height + 1` to ensure that the `backlog` memory won't be too large, it won't interrupt the consensus
// process, because that the `core` instance will sync block until the current height to the correct value.
//
//
// todo(fuk):if the view is equal the current view, compare the Message type and round state, with the right
// round state sequence, Message ahead of certain state is `old Message`, and Message behind certain
// state is `future Message`. Message type and round state table as follow:
func (c *core) checkView(view *hs.View) error {
	if view == nil || view.Height == nil || view.Round == nil {
		return errInvalidMessage
	}

	if hdiff, rdiff := view.Sub(c.currentView()); hdiff < 0 {
		return errOldMessage
	} else if hdiff > 1 {
		return errFarAwayFutureMessage
	} else if hdiff == 1 {
		return errFutureMessage
	} else if rdiff < 0 {
		return errOldMessage
	} else if rdiff == 0 {
		return nil
	} else {
		return errFutureMessage
	}
}

// sendEvent sends events to mux
func (c *core) sendEvent(ev interface{}) {
	c.backend.EventMux().Post(ev)
}

func (c *core) newLogger() log.Logger {
	logger := c.logger.New("state", c.currentState(), "view", c.currentView())
	return logger
}

func buildRoundStartQC(lastBlock *types.Block) (*hs.QuorumCert, error) {
	qc := &hs.QuorumCert{
		View: &hs.View{
			Round:  big.NewInt(0),
			Height: lastBlock.Number(),
		},
		Code: hs.MsgTypePrepareVote,
	}

	// allow genesis node and proposer to be empty
	if lastBlock.NumberU64() == 0 {
		qc.Proposer = common.Address{}
		qc.TreeNode = common.HexToHash("0x12345")
	} else {
		qc.Proposer = lastBlock.Coinbase()
		qc.TreeNode = lastBlock.Hash()
	}

	// [TODO] Add Aggregated BLSSignature before committing during signer sealing
	extra, err := types.ExtractHotstuffExtra(lastBlock.Header())
	if err != nil {
		return nil, err
	}
	if extra.Seal == nil || extra.BLSSignature == nil {
		return nil, errInvalidNode
	}

	qc.BLSSignature = extra.BLSSignature
	return qc, nil
}
