package core

import (
	"time"

	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
	"github.com/ethereum/go-ethereum/core/types"
)

// sendPrepare leader send message of prepare(view, node, highQC)
func (c *core) sendPrepare() {

	// filter incorrect proposer and state
	if !c.IsProposer() || c.currentState() != hs.StateHighQC {
		return
	}

	var (
		block  *types.Block
		code   = hs.MsgTypePrepare
		highQC = c.current.HighQC()
		logger = c.newLogger()
	)

	// fetch block with locked node or miner pending request
	if lockedBlock := c.current.LockedBlock(); lockedBlock != nil {
		if lockedBlock.NumberU64() != c.HeightU64() {
			logger.Trace("Locked block height invalid", "msg", code, "expect", c.HeightU64(), "got", lockedBlock.NumberU64())
			return
		}
		block = lockedBlock
		logger.Trace("Reuse lock block", "msg", code, "hash", block.Hash(), "number", block.NumberU64())
	} else {
		request := c.current.PendingRequest()
		if request == nil || request.Block == nil || request.Block.NumberU64() != c.HeightU64() {
			logger.Trace("Pending request invalid", "msg", code)
			return
		} else {
			block = c.current.PendingRequest().Block
			logger.Trace("Use pending request", "msg", code, "hash", block.Hash(), "number", block.NumberU64())
		}
	}

	// consensus spent time always less than a block period, waiting for `delay` time to catch up the system time.
	// todo(fuk): waiting in `startNewRound`
	if block.Time() > uint64(time.Now().Unix()) {
		delay := time.Unix(int64(block.Time()), 0).Sub(time.Now())
		time.Sleep(delay)
		logger.Trace("delay to broadcast proposal", "msg", code, "time", delay.Milliseconds())
	}

	// assemble message as formula: MSG(view, node, prepareQC)
	parent := highQC.TreeNode
	node := hs.NewTreeNode(parent, block)
	prepare := hs.NewPackagedQC(node, highQC)
	payload, err := hs.Encode(prepare)
	if err != nil {
		logger.Trace("Failed to encode", "msg", code, "err", err)
		return
	}

	// store the node before `handlePrepare` to prevent the replica from receiving the message and voting earlier
	// than the leader, and finally causing `handlePrepareVote` to fail.
	if err := c.current.SetTreeNode(node); err != nil {
		logger.Trace("Failed to set node", "msg", code, "err", err)
		return
	}

	c.broadcast(code, payload)
	logger.Trace("sendPrepare", "msg", code, "node", node.Hash(), "block", block.Hash())
}
