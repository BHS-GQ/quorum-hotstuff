package faulty

import (
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/consensus"
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
	"github.com/ethereum/go-ethereum/core/types"
)

// sendPrepare leader send message of prepare(view, node, highQC)
func (c *Core) sendPrepare() {

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
	parent := highQC.CmdNode
	node := hs.NewCmdNode(parent, block)
	prepareSubject := hs.NewPackagedQC(node, highQC)
	payload, err := hs.Encode(prepareSubject)
	if err != nil {
		logger.Trace("Failed to encode", "msg", code, "err", err)
		return
	}

	// store the node before `handlePrepare` to prevent the replica from receiving the message and voting earlier
	// than the leader, and finally causing `handlePrepareVote` to fail.
	if err := c.current.SetCmdNode(node); err != nil {
		logger.Trace("Failed to set node", "msg", code, "err", err)
		return
	}

	c.broadcast(code, payload)
	logger.Trace("sendPrepare", "msg", code, "node", node.Hash(), "block", block.Hash())
}

// handlePrepare implement description as follow:
// ```
//  repo wait for message m : matchingMsg(m, prepare, curView) from leader(curView)
//	if m.node extends from m.justify.node ∧
//	safeNode(m.node, m.justify) then
//	send voteMsg(prepare, m.node, ⊥) to leader(curView)
// ```
func (c *Core) handlePrepare(data *hs.Message) error {
	var (
		logger  = c.newLogger()
		code    = data.Code
		src     = data.Address
		subject *hs.PackagedQC
	)

	// check message
	if err := data.Decode(&subject); err != nil {
		logger.Trace("Failed to decode", "msg", code, "src", src, "err", err)
		return hs.ErrFailedDecodePrepare
	}
	if err := c.checkView(data.View); err != nil {
		logger.Trace("Failed to check view", "msg", code, "src", src, "err", err)
		return err
	}
	if err := c.checkMsgSource(src); err != nil {
		logger.Trace("Failed to check proposer", "msg", code, "src", src, "err", err)
		return err
	}

	// local node is nil before `handlePrepare`, only check fields here.
	node := subject.CmdNode
	if err := c.checkNode(node, false); err != nil {
		logger.Trace("Failed to check node", "msg", code, "src", src, "err", err)
		return err
	}

	// ensure remote block is legal.
	block := node.Block
	if err := c.checkBlock(block); err != nil {
		logger.Trace("Failed to check block", "msg", code, "src", src, "err", err)
		return err
	}
	if duration, err := c.backend.Verify(block); err != nil {
		logger.Trace("Failed to verify unsealed proposal", "msg", code, "src", src, "err", err, "duration", duration)
		return hs.ErrVerifyUnsealedProposal
	}
	if err := c.executeBlock(block); err != nil {
		logger.Trace("Failed to execute block", "msg", code, "src", src, "err", err)
		return err
	}

	// safety and liveness rules judgement.
	highQC := subject.QC
	if err := c.verifyQC(data, highQC); err != nil {
		logger.Trace("Failed to verify highQC", "msg", code, "src", src, "err", err, "highQC", highQC)
		return err
	}
	if err := c.safeNode(node, highQC); err != nil {
		logger.Trace("Failed to check safeNode", "msg", code, "src", src, "err", err)
		return hs.ErrSafeNode
	}

	logger.Trace("handlePrepare", "msg", code, "src", src, "node", node.Hash(), "block", block.Hash())

	// accept msg info, DONT persist node before accept `prepareQC`
	if c.IsProposer() && c.currentState() == hs.StateHighQC {
		c.sendVote(hs.MsgTypePrepareVote)
	}
	if !c.IsProposer() && c.currentState() < hs.StateHighQC {
		// Update round state to new CmdNode
		if err := c.current.SetCmdNode(node); err != nil {
			logger.Trace("Failed to set node", "msg", code, "err", err)
			return err
		}
		c.setCurrentState(hs.StateHighQC)
		logger.Trace("acceptHighQC", "msg", code, "highQC", highQC.CmdNode, "node", node.Hash())

		// Node for vote-building is fetched from round state
		c.sendVote(hs.MsgTypePrepareVote)
	}

	return nil
}

// proposer do not need execute block again after miner.worker commitNewWork.
func (c *Core) executeBlock(block *types.Block) error {
	if c.IsProposer() {
		c.current.executed = &consensus.ExecutedBlock{Block: block}
		return nil
	}

	executed, err := c.backend.ExecuteBlock(block)
	if err != nil {
		return err
	}
	c.current.executed = executed
	return nil
}

func (c *Core) safeNode(node *hs.CmdNode, highQC *hs.QuorumCert) error {
	// Data checks
	if highQC == nil || highQC.View == nil {
		return hs.ErrInvalidQC
	}
	if highQC.CmdNode != node.Parent {
		return fmt.Errorf("expect parent %v, got %v", highQC.CmdNode, node.Parent)
	}

	lockQC := c.current.LockQC()
	if lockQC == nil {
		c.logger.Warn("LockQC be nil should only happen at `startUp`")
		return nil
	}

	// Safety
	if lockQC.CmdNode == node.Parent {
		return nil
	}

	// Liveliness
	if highQC.View.Cmp(lockQC.View) > 0 {
		return nil
	}

	return hs.ErrSafeNode
}
