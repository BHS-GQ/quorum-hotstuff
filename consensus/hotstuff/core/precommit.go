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
func (c *core) handlePrepareVote(data *hs.Message) error {

	var (
		logger = c.newLogger()
		code   = data.Code
		src    = data.Address
		vote   *hs.Vote
	)

	// check message
	if err := data.Decode(&vote); err != nil {
		logger.Trace("Failed to decode", "msg", code, "src", src, "err", err)
		return errFailedDecodePrepare
	}
	if err := c.checkView(data.View); err != nil {
		logger.Trace("Failed to check view", "msg", code, "src", src, "err", err)
		return err
	}
	if err := c.checkVote(vote, hs.MsgTypePrepareVote); err != nil {
		logger.Trace("Failed to check vote", "msg", code, "src", src, "err", err)
		return err
	}
	if err := c.checkMsgDest(); err != nil {
		logger.Trace("Failed to check proposer", "msg", code, "src", src, "err", err)
		return err
	}

	// queued vote into messageSet to ensure that at least 2/3 validators vote on the same step.
	// [TODO] ignore proposer self-vote
	if err := c.current.AddPrepareVote(data); err != nil {
		logger.Trace("Failed to add vote", "msg", code, "src", src, "err", err)
		return errAddPrepareVote
	}

	logger.Trace("handlePrepareVote", "msg", code, "src", src, "vote", vote)

	if size := c.current.PrepareVoteSize(); size >= c.Q() && c.currentState() == hs.StateHighQC {
		prepareQC, err := c.messages2qc(code)
		if err != nil {
			logger.Trace("Failed to assemble prepareQC", "msg", code, "err", err)
			return errInvalidQC
		}
		if err := c.acceptPrepareQC(prepareQC); err != nil {
			logger.Trace("Failed to accept prepareQC", "msg", code, "err", err)
			return err
		}
		logger.Trace("acceptPrepareQC", "msg", code, "prepareQC", prepareQC.TreeNode)

		c.sendPreCommit(prepareQC)
	}

	return nil
}

// sendPreCommit leader send message of `prepareQC`
func (c *core) sendPreCommit(prepareQC *hs.QuorumCert) {
	logger := c.newLogger()

	code := hs.MsgTypePreCommit
	payload, err := hs.Encode(prepareQC)
	if err != nil {
		logger.Trace("Failed to encode", "msg", code, "err", err)
		return
	}
	c.broadcast(code, payload)
	logger.Trace("sendPreCommit", "msg", code, "node", prepareQC.TreeNode)
}

// handlePreCommit implement description as follow:
// ```
//  repo wait for message m : matchingQC(m.justify, prepare, curView) from leader(curView)
//	prepareQC ← m.justify
//	send voteMsg(pre-commit, m.justify.node, ⊥) to leader(curView)
// ```
func (c *core) handlePreCommit(data *hs.Message) error {
	var (
		logger    = c.newLogger()
		code      = data.Code
		src       = data.Address
		prepareQC *hs.QuorumCert
	)

	// check message
	if err := data.Decode(&prepareQC); err != nil {
		logger.Trace("Failed to check decode", "msg", code, "src", src, "err", err)
		return errFailedDecodePreCommit
	}
	if err := c.checkView(data.View); err != nil {
		logger.Trace("Failed to check view", "msg", code, "src", src, "err", err)
		return err
	}
	if err := c.checkMsgSource(src); err != nil {
		logger.Trace("Failed to check proposer", "msg", code, "src", src, "err", err)
		return err
	}

	// ensure `prepareQC` is legal
	if err := c.verifyQC(data, prepareQC); err != nil {
		logger.Trace("Failed to verify prepareQC", "msg", code, "src", src, "err", err)
		return err
	}

	logger.Trace("handlePreCommit", "msg", code, "src", src, "prepareQC", prepareQC.TreeNode)

	// accept msg info and state
	if c.IsProposer() && c.currentState() == hs.StatePrepared {
		c.sendVote(hs.MsgTypePreCommitVote)
	}
	if !c.IsProposer() && c.currentState() == hs.StateHighQC {
		if err := c.acceptPrepareQC(prepareQC); err != nil {
			logger.Trace("Failed to accept prepareQC", "msg", code, "err", err)
			return err
		}
		logger.Trace("acceptPrepareQC", "msg", code, "prepareQC", prepareQC.TreeNode)
		c.sendVote(hs.MsgTypePreCommitVote)
	}

	return nil
}

func (c *core) acceptPrepareQC(prepareQC *hs.QuorumCert) error {
	if err := c.current.SetTreeNode(c.current.TreeNode()); err != nil {
		return err
	}
	if err := c.current.SetPrepareQC(prepareQC); err != nil {
		return err
	}
	c.current.SetState(hs.StatePrepared)
	return nil
}
