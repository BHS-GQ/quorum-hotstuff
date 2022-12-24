package core

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/hotstuff"
)

func (c *core) handlePrepareVote(data *hotstuff.Message, src hotstuff.Validator) error {
	logger := c.newLogger()

	var (
		vote   *Vote
		msgTyp = MsgTypePrepareVote
	)
	if err := data.Decode(&vote); err != nil {
		logger.Trace("Failed to decode", "msg", msgTyp, "err", err)
		return errFailedDecodePrepareVote
	}
	if err := c.checkView(msgTyp, vote.View); err != nil {
		logger.Trace("Failed to check view", "msg", msgTyp, "err", err)
		return err
	}
	if err := c.checkVote(vote); err != nil {
		logger.Trace("Failed to check vote", "msg", msgTyp, "err", err)
		return err
	}
	if err := c.checkMsgToProposer(); err != nil {
		logger.Trace("Failed to check proposer", "msg", msgTyp, "err", err)
		return err
	}
	if err := c.current.AddPrepareVote(data); err != nil {
		logger.Trace("Failed to add vote", "msg", msgTyp, "err", err)
		return errAddPrepareVote
	}

	logger.Trace("handlePrepareVote", "msg", msgTyp, "src", src.Address(), "hash", vote.Digest)

	if size := c.current.PrepareVoteSize(); size >= c.Q() && c.currentState() < StatePrepared {
		collectionPub := make(map[common.Address][]byte)
		collectionSig := make(map[common.Address][]byte)

		for _, msg := range c.current.PrepareVotes() {
			// Notes: msg.AggPub and msg.AggSign are assigned by calling core.go/finalizeMessage at the delegators' sides
			collectionPub[msg.Address], collectionSig[msg.Address] = msg.AggPub, msg.AggSign
		}

		newProposal, err := c.backend.PreCommit(c.current.Proposal(), c.valSet, collectionPub, collectionSig)
		if err != nil {
			logger.Trace("Failed to assemble partial sigs", "err", err)
			return err
		}

		prepareQC := proposal2QC(newProposal, c.current.Round())
		c.acceptPrepare(prepareQC, newProposal)
		logger.Trace("acceptPrepare", "msg", msgTyp, "src", src.Address(), "hash", newProposal.Hash(), "msgSize", size)

		c.sendPreCommit()
	}

	return nil
}

func (c *core) sendPreCommit() {
	logger := c.newLogger()

	msgTyp := MsgTypePreCommit
	msg := &MsgPreCommit{
		View:      c.currentView(),
		Proposal:  c.current.Proposal(),
		PrepareQC: c.current.PrepareQC(),
	}
	payload, err := Encode(msg)

	collectionPub := make(map[common.Address][]byte)
	collectionSig := make(map[common.Address][]byte)
	for _, msg := range c.current.preCommitVotes.Values() {
		// Notes: msg.AggPub and msg.AggSign are assigned by calling core.go/finalizeMessage at the delegators' sides
		collectionPub[msg.Address], collectionSig[msg.Address] = msg.AggPub, msg.AggSign
	}
	// Use mask aggsign and aggpub fields for aggsig and aggkey
	_, aggSig, aggKey, err := c.signer.AggregateSignature(c.valSet, collectionPub, collectionSig)
	if err != nil {
		logger.Error("Failed to aggregate", "msg", msgTyp, "err", err)
	}

	if err != nil {
		logger.Trace("Failed to encode", "msg", msgTyp, "err", err)
		return
	}
	c.broadcast(&hotstuff.Message{
		Code:    msgTyp,
		Msg:     payload,
		AggSign: aggSig,
		AggPub:  aggKey,
	})
	logger.Trace("sendPreCommit", "msg view", msg.View, "proposal", msg.Proposal.Hash())
}

func (c *core) handlePreCommit(data *hotstuff.Message, src hotstuff.Validator) error {
	logger := c.newLogger()

	var (
		msg    *MsgPreCommit
		msgTyp = MsgTypePreCommit
	)
	if err := data.Decode(&msg); err != nil {
		logger.Trace("Failed to check decode", "msg", msgTyp, "err", err)
		return errFailedDecodePreCommit
	}
	if err := c.checkView(MsgTypePreCommit, msg.View); err != nil {
		logger.Trace("Failed to check view", "msg", msgTyp, "err", err)
		return err
	}
	if err := c.checkMsgFromProposer(src); err != nil {
		logger.Trace("Failed to check proposer", "msg", msgTyp, "err", err)
		return err
	}
	if msg.Proposal.Hash() != msg.PrepareQC.Hash {
		logger.Trace("Failed to check msg", "msg", msgTyp, "expect prepareQC hash", msg.Proposal.Hash().Hex(), "got", msg.PrepareQC.Hash.Hex())
		return errInvalidProposal
	}
	if _, err := c.backend.Verify(msg.Proposal); err != nil {
		logger.Trace("Failed to check verify proposal", "msg", msgTyp, "err", err)
		return err
	}
	if err := c.signer.VerifyQC(data, c.expectedMsg, msg.PrepareQC, c.valSet); err != nil {
		logger.Trace("Failed to verify prepareQC", "msg", msgTyp, "err", err)
		return err
	}

	logger.Trace("handlePreCommit", "msg", msgTyp, "src", src.Address(), "hash", msg.Proposal.Hash())

	if c.IsProposer() && c.currentState() < StatePreCommitted {
		c.sendPreCommitVote()
	}
	if !c.IsProposer() && c.currentState() < StatePrepared {
		c.acceptPrepare(msg.PrepareQC, msg.Proposal)
		logger.Trace("acceptPrepare", "msg", msgTyp, "src", src.Address(), "prepareQC", msg.PrepareQC.Hash)

		c.sendPreCommitVote()
	}

	return nil
}

func (c *core) acceptPrepare(prepareQC *hotstuff.QuorumCert, proposal hotstuff.Proposal) {
	c.current.SetPrepareQC(prepareQC)
	c.current.SetProposal(proposal)
	c.current.SetState(StatePrepared)
}

func (c *core) sendPreCommitVote() {
	logger := c.newLogger()

	msgTyp := MsgTypePreCommitVote
	vote := c.current.Vote()
	if vote == nil {
		logger.Trace("Failed to send vote", "msg", msgTyp, "err", "current vote is nil")
		return
	}
	payload, err := Encode(vote)
	if err != nil {
		logger.Error("Failed to encode", "msg", msgTyp, "err", err)
		return
	}
	c.broadcast(&hotstuff.Message{Code: msgTyp, Msg: payload})
	logger.Trace("sendPreCommitVote", "vote view", vote.View, "vote", vote.Digest)
}
