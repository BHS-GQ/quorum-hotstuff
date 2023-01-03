package core

import (
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
	"github.com/ethereum/go-ethereum/core/types"
)

// handleCommitVote implement description as follow:
// ```
// leader wait for (n n f) votes: V ← {v | matchingMsg(v, commit, curView)}
//	commitQC ← QC(V )
//	broadcast Msg(decide, ⊥, commitQC )
// ```
func (c *core) handleCommitVote(data *hs.Message) error {
	var (
		logger = c.newLogger()
		code   = data.Code
		src    = data.Address
		vote   *hs.Vote
	)

	// check message
	if err := data.Decode(&vote); err != nil {
		logger.Trace("Failed to decode", "msg", code, "src", src, "err", err)
		return err
	}
	if err := c.checkMsgDest(); err != nil {
		logger.Trace("Failed to check proposer", "msg", code, "src", src, "err", err)
		return err
	}
	if err := c.checkVote(vote, hs.MsgTypeCommitVote); err != nil {
		logger.Trace("Failed to check vote", "msg", code, "src", src, "err", err)
		return err
	}

	// check locked block's committed seals
	lockedBlock := c.current.LockedBlock()
	if lockedBlock == nil {
		logger.Trace("Failed to get lockBlock", "msg", code, "src", src, "err", "block is nil")
		return errInvalidNode
	}

	// queue vote into messageSet to ensure that at least 2/3 validator vote at the same step.
	if err := c.current.AddCommitVote(data); err != nil {
		logger.Trace("Failed to add vote", "msg", code, "src", src, "err", err)
		return errAddPreCommitVote
	}

	logger.Trace("handleCommitVote", "msg", code, "src", src, "hash", vote)

	// assemble committed signatures to reorg the locked block, and create `commitQC` at the same time.
	if size := c.current.CommitVoteSize(); size >= c.Q() && c.currentState() == hs.StatePreCommitted {
		// [TODO] Do we need block sealing this early?
		// sealedBlock, err := c.backend.SealBlock(lockedBlock)
		// if err != nil {
		// 	logger.Trace("Failed to assemble committed proposal", "msg", code, "err", err)
		// 	return err
		// }
		commitQC, err := c.messages2qc(code)
		if err != nil {
			logger.Trace("Failed to assemble commitQC", "msg", code, "err", err)
			return err
		}
		if err := c.acceptCommitQC(lockedBlock, commitQC); err != nil {
			logger.Trace("Failed to accept commitQC")
		}
		logger.Trace("acceptCommit", "msg", code, "msgSize", size)

		c.sendDecide(commitQC)
	}

	return nil
}

func (c *core) sendDecide(commitQC *hs.QuorumCert) {
	logger := c.newLogger()

	code := hs.MsgTypeDecide
	payload, err := hs.Encode(commitQC)
	if err != nil {
		logger.Trace("Failed to encode", "msg", code, "err", err)
		return
	}
	c.broadcast(code, payload)

	logger.Trace("sendDecide", "msg", code, "node", commitQC.TreeNode)
}

func (c *core) acceptCommitQC(sealedBlock *types.Block, commitQC *hs.QuorumCert) error {
	if err := c.current.SetSealedBlock(sealedBlock); err != nil {
		return err
	}
	if err := c.current.SetCommittedQC(commitQC); err != nil {
		return err
	}
	c.current.SetState(hs.StateCommitted)
	return nil
}
