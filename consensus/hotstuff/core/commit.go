package core

import (
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
)

// handlePreCommitVote implement description as follow:
// ```
//  leader wait for (n n f) votes: V ← {v | matchingMsg(v, pre-commit, curView)}
//	precommitQC ← QC(V )
//	broadcast Msg(commit, ⊥, precommitQC )
// ```
// [NOTE] We follow HotStuff specifications strictly, so whole CmdNode is NOT sent
func (c *Core) handlePreCommitVote(data *hs.Message) error {
	var (
		logger = c.newLogger()
		code   = data.Code
		src    = data.Address
		vote   *hs.Vote
	)

	// check message
	if err := data.Decode(&vote); err != nil {
		logger.Trace("Failed to decode", "msgCode", code, "src", src, "err", err)
		return err
	}
	if err := c.checkView(data.View); err != nil {
		logger.Trace("Failed to check view", "msgCode", code, "src", src, "err", err)
		return err
	}
	if err := c.checkVote(vote, hs.MsgTypePreCommitVote); err != nil {
		logger.Trace("Failed to check vote", "msgCode", code, "src", src, "err", err)
		return err
	}
	if err := c.checkMsgDest(); err != nil {
		logger.Trace("Failed to check proposal", "msgCode", code, "src", src, "err", err)
		return err
	}

	if err := c.current.AddPreCommitVote(data); err != nil {
		logger.Trace("Failed to add vote", "msgCode", code, "src", src, "err", err)
		return hs.ErrAddPreCommitVote
	}

	logger.Trace("handlePreCommitVote", "msgCode", code, "src", src, "hash", vote)

	if size := c.current.PreCommitVoteSize(); size >= c.valSet.Q() && c.currentState() < hs.StatePreCommitted {
		lockQC, err := c.messagesToQC(code)
		if err != nil {
			logger.Trace("Failed to assemble lockQC", "msgCode", code, "err", err)
			return err
		}
		if err := c.acceptLockQC(lockQC); err != nil {
			logger.Trace("Failed to accept lockQC", "msgCode", code, "err", err)
			return err
		}
		logger.Trace("acceptLockQC", "msgCode", code, "msgSize", size)

		c.sendCommit(lockQC)
	}
	return nil
}

func (c *Core) sendCommit(lockQC *hs.QuorumCert) {
	logger := c.newLogger()

	code := hs.MsgTypeCommit
	payload, err := hs.Encode(lockQC)
	if err != nil {
		logger.Error("Failed to encode", "msgCode", code, "err", err)
		return
	}
	c.broadcast(code, payload)
	logger.Trace("sendCommit", "msgCode", code, "node", lockQC.CmdNode)
}

// handleCommit implement description as follow:
// ```
// repo wait for message m : matchingQC(m.justify, pre-commit, curView) from leader(curView)
//	lockedQC ← m.justify
//	send voteMsg(commit, m.justify.node, ⊥) to leader(curView)
// ```
func (c *Core) handleCommit(data *hs.Message) error {
	var (
		logger = c.newLogger()
		code   = data.Code
		src    = data.Address
		lockQC *hs.QuorumCert
	)

	// check message
	if err := data.Decode(&lockQC); err != nil {
		logger.Trace("Failed to decode", "msgCode", code, "src", src, "err", err)
		return hs.ErrFailedDecodeCommit
	}
	if err := c.checkView(lockQC.View); err != nil {
		logger.Trace("Failed to check view", "msgCode", code, "src", src, "err", err)
		return err
	}
	if err := c.checkMsgSource(src); err != nil {
		logger.Trace("Failed to check proposer", "msgCode", code, "src", src, "err", err)
		return err
	}

	// ensure `lockQC` is legal
	if err := c.verifyQC(data, lockQC); err != nil {
		logger.Trace("Failed to verify lockQC", "msgCode", code, "src", src, "err", err)
		return err
	}

	logger.Trace("handleCommit", "msgCode", code, "src", src, "lockQC", lockQC.CmdNode)

	// accept lockQC
	if c.IsProposer() && c.currentState() < hs.StateCommitted {
		c.sendVote(hs.MsgTypeCommitVote)
	}
	if !c.IsProposer() && c.currentState() < hs.StatePreCommitted {
		if err := c.acceptLockQC(lockQC); err != nil {
			logger.Trace("Failed to accept lockQC", "msgCode", code, "err", err)
			return err
		}
		logger.Trace("acceptLockQC", "msgCode", code, "lockQC", lockQC.CmdNode)

		c.sendVote(hs.MsgTypeCommitVote)
	}

	return nil
}

func (c *Core) acceptLockQC(qc *hs.QuorumCert) error {
	if err := c.current.Lock(qc); err != nil {
		return err
	}
	c.current.SetState(hs.StatePreCommitted)
	return nil
}
