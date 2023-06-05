package core

import (
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
)

// handlePreCommitVote
// 	- Leader waits for pre-commit votes
// 	- Quorum: build pre-commitQC and send commit message
// 	- We follow HotStuff specifications strictly, so whole ProposedBlock is NOT sent
//  - BHS Spec: implements leader portion of COMMIT phase (lines 19-22)
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
	logger.Trace("sendCommit", "msgCode", code, "node", lockQC.ProposedBlock)
}

// handleCommit
// 	- Replica waits for commit message and verifies pre-commitQC
// 	- Verified: send commit vote
//  - BHS Spec: implements replica portion of COMMIT phase (lines 23-26)
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

	logger.Trace("handleCommit", "msgCode", code, "src", src, "lockQC", lockQC.ProposedBlock)

	// accept lockQC
	if c.IsProposer() && c.currentState() < hs.StateCommitted {
		c.sendVote(hs.MsgTypeCommitVote)
	}
	if !c.IsProposer() && c.currentState() < hs.StatePreCommitted {
		if err := c.acceptLockQC(lockQC); err != nil {
			logger.Trace("Failed to accept lockQC", "msgCode", code, "err", err)
			return err
		}
		logger.Trace("acceptLockQC", "msgCode", code, "lockQC", lockQC.ProposedBlock)

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
