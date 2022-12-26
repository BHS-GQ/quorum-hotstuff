package core

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/hotstuff"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

func (c *core) checkMsgFromProposer(src hotstuff.Validator) error {
	if !c.valSet.IsProposer(src.Address()) {
		return errNotFromProposer
	}
	return nil
}

func (c *core) checkMsgToProposer() error {
	if !c.IsProposer() {
		return errNotToProposer
	}
	return nil
}

func (c *core) checkPrepareQC(qc *hotstuff.QuorumCert) error {
	if qc == nil {
		return fmt.Errorf("external prepare qc is nil")
	}

	localQC := c.current.PrepareQC()
	if localQC == nil {
		return fmt.Errorf("current prepare qc is nil")
	}

	if localQC.View.Cmp(qc.View) != 0 {
		return fmt.Errorf("view unsame, expect %v, got %v", localQC.View, qc.View)
	}
	if localQC.Proposer != qc.Proposer {
		return fmt.Errorf("proposer unsame, expect %v, got %v", localQC.Proposer, qc.Proposer)
	}
	if localQC.Hash != qc.Hash {
		return fmt.Errorf("expect %v, got %v", localQC.Hash, qc.Hash)
	}
	return nil
}

func (c *core) checkPreCommittedQC(qc *hotstuff.QuorumCert) error {
	if qc == nil {
		return fmt.Errorf("external pre-committed qc is nil")
	}
	localQC := c.current.PreCommittedQC()
	if localQC == nil {
		return fmt.Errorf("current prepare qc is nil")
	}

	if localQC.View.Cmp(qc.View) != 0 {
		return fmt.Errorf("view unsame, expect %v, got %v", localQC.View, qc.View)
	}
	if localQC.Proposer != qc.Proposer {
		return fmt.Errorf("proposer unsame, expect %v, got %v", localQC.Proposer, qc.Proposer)
	}
	if localQC.Hash != qc.Hash {
		return fmt.Errorf("expect %v, got %v", localQC.Hash, qc.Hash)
	}
	return nil
}

func (c *core) checkVote(vote *Vote) error {
	currentVote := c.current.Vote(MsgType(vote.Code.Value()))
	if vote == nil {
		return fmt.Errorf("external vote is nil")
	}
	if currentVote == nil {
		return fmt.Errorf("current vote is nil")
	}
	if !reflect.DeepEqual(currentVote, vote) {
		return fmt.Errorf("expect %s, got %s", currentVote.String(), vote.String())
	}
	return nil
}

func (c *core) checkLockedProposal(msg hotstuff.Proposal) error {
	isLocked, proposal := c.current.LastLockedProposal()
	if !isLocked {
		return nil
	}
	if proposal == nil {
		return fmt.Errorf("current locked proposal is nil")
	}
	if !reflect.DeepEqual(proposal, msg) {
		return fmt.Errorf("expect %s, got %s", proposal.Hash().Hex(), msg.Hash().Hex())
	}
	return nil
}

// checkView checks the Message state, remote msg view should not be nil(local view WONT be nil).
// if the view is ahead of current view we name the Message to be future Message, and if the view
// is behind of current view, we name it as old Message. `old Message` and `invalid Message` will
// be dropped. and we use the storage of `backlog` to cache the future Message, it only allow the
// Message height not bigger than `current height + 1` to ensure that the `backlog` memory won't be
// too large, it won't interrupt the consensus process, because that the `core` instance will sync
// block until the current height to the correct value.
//
// if the view is equal the current view, compare the Message type and round state, with the right
// round state sequence, Message ahead of certain state is `old Message`, and Message behind certain
// state is `future Message`. Message type and round state table as follow:
func (c *core) checkView(msgCode hotstuff.MsgType, view *hotstuff.View) error {
	if view == nil || view.Height == nil || view.Round == nil {
		return errInvalidMessage
	}

	// validators not in the same view
	if hdiff, rdiff := view.Sub(c.currentView()); hdiff < 0 {
		return errOldMessage
	} else if hdiff > 1 {
		return errFarAwayFutureMessage
	} else if hdiff == 1 {
		return errFutureMessage
	} else if rdiff < 0 {
		return errOldMessage
	} else if rdiff == 0 {
		return nil
	} else {
		return errFutureMessage
	}
}

func (c *core) finalizeMessage(msg *hotstuff.Message) ([]byte, error) {
	var err error

	// Add proof of consensus
	switch msg.Code {
	case MsgTypePrepareVote, MsgTypePreCommitVote, MsgTypeCommitVote:
		// Sign PAYLOAD
		// Payload is usually a vote e
		msg.BLSSignature, err = c.signer.BLSSign(msg.Msg)
		if err != nil {
			return nil, err
		}

	case MsgTypePreCommit, MsgTypeCommit, MsgTypeDecide:
		// Add Threshold Signature as proof
		// MsgTypePreCommit re-signing is redundant; find way to get from block

	default:

	}

	// Sign Message
	data, err := msg.PayloadNoSig()
	if err != nil {
		return nil, err
	}
	msg.Signature, err = c.signer.Sign(data)
	if err != nil {
		return nil, err
	}

	// Convert to payload
	payload, err := msg.Payload()
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func (c *core) broadcast(code MsgType, payload []byte) {
	logger := c.logger.New("state", c.currentState())

	msg := &hotstuff.Message{
		Address: c.Address(),
		View:    c.currentView(),
		Code:    code,
		Msg:     payload,
	}
	payload, err := c.finalizeMessage(msg)
	if err != nil {
		logger.Error("Failed to finalize Message", "msg", msg, "err", err)
		return
	}

	switch msg.Code {
	case MsgTypeNewView, MsgTypePrepareVote, MsgTypePreCommitVote, MsgTypeCommitVote:
		if err := c.backend.Unicast(c.valSet, payload); err != nil {
			logger.Error("Failed to unicast Message", "msg", msg, "err", err)
		}
	case MsgTypePrepare, MsgTypePreCommit, MsgTypeCommit, MsgTypeDecide:
		if err := c.backend.Broadcast(c.valSet, payload); err != nil {
			logger.Error("Failed to broadcast Message", "msg", msg, "err", err)
		}
	default:
		logger.Error("invalid msg type", "msg", msg)
	}
}

func (c *core) checkValidatorSignature(data []byte, sig []byte) (common.Address, error) {
	return c.signer.CheckSignature(c.valSet, data, sig)
}

func (c *core) newLogger() log.Logger {
	logger := c.logger.New("state", c.currentState(), "view", c.currentView())
	return logger
}

func proposal2QC(proposal hotstuff.Proposal, round *big.Int) *hotstuff.QuorumCert {
	block := proposal.(*types.Block)
	h := block.Header()
	qc := new(hotstuff.QuorumCert)
	qc.Code = MsgTypeNewView
	qc.View = &hotstuff.View{
		Height: block.Number(),
		Round:  round,
	}
	qc.Hash = h.Hash()
	qc.Proposer = h.Coinbase
	qc.BLSSignature = []byte{}
	return qc
}

// assemble messages to quorum cert.
func (c *core) messages2qc(code MsgType) (*hotstuff.QuorumCert, error) {
	var msgs []*hotstuff.Message
	switch code {
	case MsgTypePrepareVote:
		msgs = c.current.PrepareVotes()
	case MsgTypePreCommitVote:
		msgs = c.current.PreCommitVotes()
	case MsgTypeCommitVote:
		msgs = c.current.CommitVotes()
	default:
		return nil, fmt.Errorf("Invalid code")
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("assemble qc: not enough message")
	}

	// Get signatures from votes
	sigShares := make([][]byte, 0)
	view := c.currentView()
	proposalHash := c.current.proposal.Hash()
	expectedVote := &Vote{
		Code:   code,
		View:   view,
		Digest: proposalHash, // Instead of sending entire proposal, use hash
	}
	expectedVoteBytes, err := Encode(expectedVote)
	c.logger.Trace("Expected vote", "vote", expectedVote, "byte", hex.EncodeToString(expectedVoteBytes))
	if err != nil {
		return nil, err
	}

	for _, msg := range msgs {
		var vote Vote
		// checking might not be needed, already done at handle...()
		msg.Decode(&vote)
		c.logger.Trace("Checking vote", "vote", vote, "byte", hex.EncodeToString(msg.Msg), "msgAddr", msg.Address)
		if !bytes.Equal(expectedVoteBytes, msg.Msg) {
			c.logger.Trace("SOMETHING WRONG")
			return nil, fmt.Errorf("Vote bytes not equal!")
		}
		sigShares = append(sigShares, msg.BLSSignature)
	}

	aggSig, err := c.signer.BLSRecoverAggSig(expectedVoteBytes, sigShares)
	if err != nil {
		return nil, err
	}

	// Build QC
	c.logger.Trace("Building QC")
	block := c.current.Proposal().(*types.Block) // Be careful of pointers
	h := block.Header()

	qc := new(hotstuff.QuorumCert)
	qc.View = expectedVote.View // might err
	qc.Code = expectedVote.Code
	qc.Hash = expectedVote.Digest
	qc.Proposer = h.Coinbase
	qc.BLSSignature = aggSig
	// [TODO] Decide if h.Extra is needed!

	return qc, nil
}
