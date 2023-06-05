package core

import (
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
)

// handlePrepareVote implement basic hotstuff description as follow:
// ```
//  leader wait for (n n f) votes: V ← {v | matchingMsg(v, prepare, curView)}
//	prepareQC ← QC(V )
//	broadcast Msg(pre-commit, ⊥, prepareQC )
// ```
// [NOTE] We follow HotStuff specifications strictly, so whole ProposedBlock is NOT sent
func (c *Core) handlePrepareVote(data *hs.Message) error {

	var (
		logger = c.newLogger()
		code   = data.Code
		src    = data.Address
		vote   *hs.Vote
	)

	// check message
	if err := data.Decode(&vote); err != nil {
		logger.Trace("Failed to decode", "msgCode", code, "src", src, "err", err)
		return hs.ErrFailedDecodePrepare
	}
	if err := c.checkView(data.View); err != nil {
		logger.Trace("Failed to check view", "msgCode", code, "src", src, "err", err)
		return err
	}
	if err := c.checkVote(vote, hs.MsgTypePrepareVote); err != nil {
		logger.Trace("Failed to check vote", "msgCode", code, "src", src, "err", err)
		return err
	}
	if err := c.checkMsgDest(); err != nil {
		logger.Trace("Failed to check proposer", "msgCode", code, "src", src, "err", err)
		return err
	}

	// queued vote into messageSet to ensure that at least 2/3 validators vote on the same step.
	if err := c.current.AddPrepareVote(data); err != nil {
		logger.Trace("Failed to add vote", "msgCode", code, "src", src, "err", err)
		return hs.ErrAddPrepareVote
	}

	logger.Trace("handlePrepareVote", "msgCode", code, "src", src, "vote", vote)

	if size := c.current.PrepareVoteSize(); size >= c.valSet.Q() && c.currentState() == hs.StateHighQC {
		prepareQC, err := c.messagesToQC(code)
		if err != nil {
			logger.Trace("Failed to assemble prepareQC", "msgCode", code, "err", err)
			return hs.ErrInvalidQC
		}
		if err := c.acceptPrepareQC(prepareQC); err != nil {
			logger.Trace("Failed to accept prepareQC", "msgCode", code, "err", err)
			return err
		}
		logger.Trace("acceptPrepareQC", "msgCode", code, "prepareQC", prepareQC.ProposedBlock)

		c.sendPreCommit(prepareQC)
	}

	return nil
}

// sendPreCommit leader send message of `prepareQC`
func (c *Core) sendPreCommit(prepareQC *hs.QuorumCert) {
	logger := c.newLogger()

	code := hs.MsgTypePreCommit
	payload, err := hs.Encode(prepareQC)
	if err != nil {
		logger.Trace("Failed to encode", "msgCode", code, "err", err)
		return
	}
	c.broadcast(code, payload)
	logger.Trace("sendPreCommit", "msgCode", code, "node", prepareQC.ProposedBlock)
}

// handlePreCommit implement description as follow:
// ```
//  repo wait for message m : matchingQC(m.justify, prepare, curView) from leader(curView)
//	prepareQC ← m.justify
//	send voteMsg(pre-commit, m.justify.node, ⊥) to leader(curView)
// ```
func (c *Core) handlePreCommit(data *hs.Message) error {
	var (
		logger    = c.newLogger()
		code      = data.Code
		src       = data.Address
		prepareQC *hs.QuorumCert
	)

	// check message
	if err := data.Decode(&prepareQC); err != nil {
		logger.Trace("Failed to check decode", "msgCode", code, "src", src, "err", err)
		return hs.ErrFailedDecodePreCommit
	}
	if err := c.checkView(data.View); err != nil {
		logger.Trace("Failed to check view", "msgCode", code, "src", src, "err", err)
		return err
	}
	if err := c.checkMsgSource(src); err != nil {
		logger.Trace("Failed to check proposer", "msgCode", code, "src", src, "err", err)
		return err
	}

	// ensure `prepareQC` is legal
	if err := c.verifyQC(data, prepareQC); err != nil {
		logger.Trace("Failed to verify prepareQC", "msgCode", code, "src", src, "err", err)
		return err
	}

	logger.Trace("handlePreCommit", "msgCode", code, "src", src, "prepareQC", prepareQC.ProposedBlock)

	// accept msg info and state
	if c.IsProposer() && c.currentState() == hs.StatePrepared {
		c.sendVote(hs.MsgTypePreCommitVote)
	}
	if !c.IsProposer() && c.currentState() == hs.StateHighQC {
		if err := c.acceptPrepareQC(prepareQC); err != nil {
			logger.Trace("Failed to accept prepareQC", "msgCode", code, "err", err)
			return err
		}
		logger.Trace("acceptPrepareQC", "msgCode", code, "prepareQC", prepareQC.ProposedBlock)
		c.sendVote(hs.MsgTypePreCommitVote)
	}

	return nil
}

func (c *Core) acceptPrepareQC(prepareQC *hs.QuorumCert) error {
	if err := c.current.SetPrepareQC(prepareQC); err != nil {
		return err
	}
	c.current.SetState(hs.StatePrepared)
	return nil
}
