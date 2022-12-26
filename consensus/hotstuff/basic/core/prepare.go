package core

import (
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/consensus/hotstuff"
	"github.com/ethereum/go-ethereum/core/types"
)

func (c *core) sendPrepare() {
	logger := c.newLogger()

	if !c.IsProposer() {
		return
	}

	// filter dump proposal
	if proposal := c.current.Proposal(); proposal != nil &&
		proposal.Number().Uint64() == c.currentView().Height.Uint64() &&
		proposal.Coinbase() == c.Address() {

		logger.Trace("Failed to send prepare", "err", "proposal already been sent")
		return
	}

	msgTyp := MsgTypePrepare
	if !c.current.IsProposalLocked() {
		proposal, err := c.createNewProposal()
		if err != nil {
			logger.Trace("Failed to create proposal", "err", err, "request set size", c.requests.Size(),
				"pendingRequest", c.current.PendingRequest(), "view", c.currentView())
			return
		}
		c.current.SetProposal(proposal)
	} else if c.current.Proposal() == nil {
		logger.Error("Failed to get locked proposal", "err", "locked proposal is nil")
		return
	}

	// MsgPrepare to be agreed-upon
	prepare := &MsgPrepare{
		View:     c.currentView(),
		Proposal: c.current.Proposal(), // Block
		HighQC:   c.current.HighQC(),   // No need to sign
	}
	payload, err := Encode(prepare)
	if err != nil {
		logger.Trace("Failed to encode", "msg", msgTyp, "err", err)
		return
	}

	// consensus spent time always less than a block period, waiting for `delay` time to catch up the system time.
	delay := time.Unix(int64(prepare.Proposal.Time()), 0).Sub(time.Now())
	time.Sleep(delay)
	logger.Trace("delay to broadcast proposal", "time", delay.Milliseconds())

	// No need to sign PREPARE with BLS
	c.broadcast(msgTyp, payload)

	logger.Trace("sendPrepare", "prepare view", prepare.View, "proposal", prepare.Proposal.Hash())
}

func (c *core) handlePrepare(data *hotstuff.Message, src hotstuff.Validator) error {
	logger := c.newLogger()

	var (
		msg    *MsgPrepare
		msgTyp = MsgTypePrepare
	)

	// MatchingMSG: prepare check
	// Decode msg.Msg (aka. Payload) to MsgPrepare
	if err := data.Decode(&msg); err != nil {
		logger.Trace("Failed to decode", "type", msgTyp, "err", err)
		return errFailedDecodePrepare
	}

	// MatchingMSG: view number check
	if err := c.checkView(msgTyp, msg.View); err != nil {
		logger.Trace("Failed to check view", "msg", msgTyp, "err", err)
		return err
	}
	if err := c.checkMsgFromProposer(src); err != nil {
		logger.Trace("Failed to check proposer", "msg", msgTyp, "err", err)
		return err
	}

	if _, err := c.backend.VerifyUnsealedProposal(msg.Proposal); err != nil {
		logger.Trace("Failed to verify unsealed proposal", "msg", msgTyp, "err", err)
		return errVerifyUnsealedProposal
	}
	if err := c.extend(msg.Proposal, msg.HighQC); err != nil {
		logger.Trace("Failed to check extend", "msg", msgTyp, "err", err)
		return errExtend
	}
	if err := c.safeNode(msg.Proposal, msg.HighQC); err != nil {
		logger.Trace("Failed to check safeNode", "msg", msgTyp, "err", err)
		return errSafeNode
	}
	if err := c.checkLockedProposal(msg.Proposal); err != nil { // Necessary?
		logger.Trace("Failed to check locked proposal", "msg", msgTyp, "err", err)
		return err
	}
	if err := c.signer.VerifyQC(msg.HighQC); err != nil {
		logger.Trace("Failed to verify highQC", "msg", msgTyp, "src", src, "err", err, "highQC", msg.HighQC.Hash)
		return err
	}

	logger.Trace("handlePrepare", "msg", msgTyp, "src", src.Address(), "hash", msg.Proposal.Hash())

	if c.IsProposer() && c.currentState() < StatePrepared {
		c.sendPrepareVote()
	}
	if !c.IsProposer() && c.currentState() < StateHighQC {
		c.current.SetHighQC(msg.HighQC)
		c.current.SetProposal(msg.Proposal)
		c.current.SetState(StateHighQC)
		logger.Trace("acceptHighQC", "msg", msgTyp, "src", src.Address(), "highQC", msg.HighQC.Hash)

		c.sendPrepareVote()
	}

	return nil
}

func (c *core) sendPrepareVote() {
	logger := c.newLogger()

	msgTyp := MsgTypePrepareVote
	vote := c.current.Vote(msgTyp)
	if vote == nil {
		logger.Trace("Failed to send vote", "msg", msgTyp, "err", "current vote is nil")
		return
	}
	payload, err := Encode(vote)
	if err != nil {
		logger.Trace("Failed to encode", "msg", msgTyp, "err", err)
		return
	}
	c.broadcast(msgTyp, payload)
	logger.Trace("sendPrepareVote", "vote view", vote.View, "vote", vote.Digest)
}

func (c *core) createNewProposal() (hotstuff.Proposal, error) {
	var req *hotstuff.Request
	if c.current.PendingRequest() != nil && c.current.PendingRequest().Proposal.Number().Cmp(c.current.Height()) == 0 {
		req = c.current.PendingRequest()
	} else {
		if req = c.requests.GetRequest(c.currentView()); req != nil {
			c.current.SetPendingRequest(req)
		} else {
			return nil, errNoRequest
		}
	}
	return req.Proposal, nil
}

func (c *core) extend(proposal hotstuff.Proposal, highQC *hotstuff.QuorumCert) error {
	if highQC == nil || highQC.View == nil {
		return fmt.Errorf("invalid qc")
	}

	proposedBlock, ok := proposal.(*types.Block)
	if !ok {
		return fmt.Errorf("invalid proposal: hash %s", proposal.Hash())
	}
	if highQC.Hash != proposedBlock.ParentHash() {
		return fmt.Errorf("block %v (parent %v) not extend hiqhQC %v", proposedBlock.Hash(), proposedBlock.ParentHash(), highQC.Hash)
	}
	return nil
}

// proposal extend lockedQC `OR` hiqhQC.view > lockedQC.view
func (c *core) safeNode(proposal hotstuff.Proposal, highQC *hotstuff.QuorumCert) error {
	logger := c.newLogger()

	if proposal.Number().Uint64() == 1 {
		return nil
	}

	lockQC := c.current.PreCommittedQC()
	if lockQC == nil {
		logger.Trace("safeNodeChecking", "lockQC", "is nil")
		return errSafeNode
	}

	proposedBlock, ok := proposal.(*types.Block)
	if !ok {
		return fmt.Errorf("invalid proposal: hash %s", proposal.Hash())
	}
	if highQC.View.Cmp(lockQC.View) > 0 || proposedBlock.ParentHash() == lockQC.Hash {
		return nil
	}

	return errSafeNode
}
