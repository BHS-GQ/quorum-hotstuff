package core

import (
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/consensus"
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
	"github.com/ethereum/go-ethereum/core/types"
)

// sendPrepare
//   - Leader builds prepare message. Picks proposal from locked block or
//     pending request from miner
//   - We make sure to delay until intended block time before sending
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

	// get locked block or miner pending request
	if lockedBlock := c.current.LockedBlock(); lockedBlock != nil {
		if lockedBlock.NumberU64() != c.HeightU64() {
			logger.Trace("Locked block height invalid", "msgCode", code, "expect", c.HeightU64(), "got", lockedBlock.NumberU64())
			return
		}
		block = lockedBlock
		logger.Trace("Reuse lock block", "msgCode", code, "hash", block.Hash(), "number", block.NumberU64())
	} else {
		request := c.current.PendingRequest()
		if request == nil || request.Block == nil || request.Block.NumberU64() != c.HeightU64() {
			logger.Trace("Pending request invalid", "msgCode", code)
			return
		} else {
			block = c.current.PendingRequest().Block
			logger.Trace("Use pending request", "msgCode", code, "hash", block.Hash(), "number", block.NumberU64())
		}
	}

	// consensus spent time always less than a block period, waiting for `delay` time to catch up the system time.
	// todo(fuk): waiting in `startNewRound`
	if block.Time() > uint64(time.Now().Unix()) {
		delay := time.Unix(int64(block.Time()), 0).Sub(time.Now())
		time.Sleep(delay)
		logger.Trace("delay to broadcast proposal", "msgCode", code, "time", delay.Milliseconds())
	}

	// assemble message
	parent := highQC.ProposedBlock
	node := hs.NewProposedBlock(parent, block)
	prepareSubject := hs.NewPackagedQC(node, highQC)
	payload, err := hs.Encode(prepareSubject)
	if err != nil {
		logger.Trace("Failed to encode", "msgCode", code, "err", err)
		return
	}

	// store the node before `handlePrepare` to prevent the replica from receiving the message and voting earlier
	// than the leader, and finally causing `handlePrepareVote` to fail.
	if err := c.current.SetProposedBlock(node); err != nil {
		logger.Trace("Failed to set node", "msgCode", code, "err", err)
		return
	}

	c.broadcast(code, payload)
	logger.Trace("sendPrepare", "msgCode", code, "node", node.Hash(), "block", block.Hash())
}

// handlePrepare
//   - Replica waits for prepare message and verifies proposed block
//   - Verified: accept proposal
func (c *Core) handlePrepare(data *hs.Message) error {
	var (
		logger  = c.newLogger()
		code    = data.Code
		src     = data.Address
		subject *hs.PackagedQC
	)

	// check message
	if err := data.Decode(&subject); err != nil {
		logger.Trace("Failed to decode", "msgCode", code, "src", src, "err", err)
		return hs.ErrFailedDecodePrepare
	}
	if err := c.checkView(data.View); err != nil {
		logger.Trace("Failed to check view", "msgCode", code, "src", src, "err", err)
		return err
	}
	if err := c.checkMsgSource(src); err != nil {
		logger.Trace("Failed to check proposer", "msgCode", code, "src", src, "err", err)
		return err
	}

	// local node is nil before `handlePrepare`, only check fields here.
	node := subject.ProposedBlock
	if err := c.checkNode(node, false); err != nil {
		logger.Trace("Failed to check node", "msgCode", code, "src", src, "err", err)
		return err
	}

	// ensure remote block is legal.
	block := node.Block
	if err := c.checkBlock(block); err != nil {
		logger.Trace("Failed to check block", "msgCode", code, "src", src, "err", err)
		return err
	}
	if duration, err := c.backend.Verify(block); err != nil {
		logger.Trace("Failed to verify unsealed proposal", "msgCode", code, "src", src, "err", err, "duration", duration)
		return hs.ErrVerifyUnsealedProposal
	}
	if err := c.executeBlock(block); err != nil {
		logger.Trace("Failed to execute block", "msgCode", code, "src", src, "err", err)
		return err
	}

	// safety and liveness rules judgement.
	highQC := subject.QC
	if err := c.verifyQC(data, highQC); err != nil {
		logger.Trace("Failed to verify highQC", "msgCode", code, "src", src, "err", err, "highQC", highQC)
		return err
	}
	if err := c.safeNode(node, highQC); err != nil {
		logger.Trace("Failed to check safeNode", "msgCode", code, "src", src, "err", err)
		return hs.ErrSafeNode
	}

	logger.Trace("handlePrepare", "msgCode", code, "src", src, "node", node.Hash(), "block", block.Hash())

	// accept msg info, DONT persist node before accept `prepareQC`
	if c.IsProposer() && c.currentState() == hs.StateHighQC {
		c.sendVote(hs.MsgTypePrepareVote)
	}
	if !c.IsProposer() && c.currentState() < hs.StateHighQC {
		// Update round state to new ProposedBlock
		if err := c.current.SetProposedBlock(node); err != nil {
			logger.Trace("Failed to set node", "msgCode", code, "err", err)
			return err
		}
		c.setCurrentState(hs.StateHighQC)
		logger.Trace("acceptHighQC", "msgCode", code, "highQC", highQC.ProposedBlock, "node", node.Hash())

		// Node for vote-building is fetched from round state
		c.sendVote(hs.MsgTypePrepareVote)
	}

	return nil
}

func (c *Core) executeBlock(block *types.Block) error {
	// proposer doesn't execute block again after miner.worker commitNewWork

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

func (c *Core) safeNode(node *hs.ProposedBlock, highQC *hs.QuorumCert) error {
	// check fields
	if highQC == nil || highQC.View == nil {
		return hs.ErrInvalidQC
	}
	if highQC.ProposedBlock != node.Parent {
		return fmt.Errorf("expect parent %v, got %v", highQC.ProposedBlock, node.Parent)
	}

	lockQC := c.current.LockQC()
	if lockQC == nil {
		c.logger.Warn("LockQC be nil should only happen at `startUp`")
		return nil
	}

	// safety
	if lockQC.ProposedBlock == node.Parent {
		return nil
	}

	// liveliness
	if highQC.View.Cmp(lockQC.View) > 0 {
		return nil
	}

	return hs.ErrSafeNode
}
