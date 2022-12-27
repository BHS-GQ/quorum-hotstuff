package core

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/hotstuff"
	"github.com/ethereum/go-ethereum/core/types"
)

func (c *core) handleCommitVote(data *hotstuff.Message, src hotstuff.Validator) error {
	logger := c.newLogger()

	var (
		vote   *Vote
		msgTyp = MsgTypeCommitVote
	)
	if err := data.Decode(&vote); err != nil {
		logger.Trace("Failed to decode", "msg", msgTyp, "err", err)
		return errFailedDecodeCommitVote
	}
	if err := c.checkView(msgTyp, vote.View); err != nil {
		logger.Trace("Failed to check view", "msg", msgTyp, "err", err)
		return err
	}
	if err := c.checkVote(vote); err != nil {
		logger.Trace("Failed to check vote", "msg", msgTyp, "err", err)
		return err
	}
	if vote.Digest != c.current.PreCommittedQC().Hash {
		logger.Trace("Failed to check hash", "msg", msgTyp, "expect vote", c.current.PreCommittedQC().Hash, "got", vote.Digest)
		return errInvalidDigest
	}
	if err := c.checkMsgToProposer(); err != nil {
		logger.Trace("Failed to check proposer", "msg", msgTyp, "err", err)
		return err
	}
	if data.Address != c.Address() {
		if err := c.current.AddCommitVote(data); err != nil {
			logger.Trace("Failed to add vote", "msg", msgTyp, "err", err)
			return errAddPreCommitVote
		}
	}

	logger.Trace("handleCommitVote", "msg", msgTyp, "src", src.Address(), "hash", vote.Digest)

	if size := c.current.CommitVoteSize(); size >= c.Q() && c.currentState() < StateCommitted {
		commitQC, err := c.messages2qc(msgTyp)
		if err != nil {
			logger.Trace("Failed to assemble commitQC", "msg", msgTyp, "err", err)
			return err
		}
		c.current.SetState(StateCommitted)
		c.current.SetCommittedQC(commitQC)
		logger.Trace("acceptCommit", "msg", msgTyp, "src", src.Address(), "hash", vote.Digest, "msgSize", size)

		c.sendDecide()
	}

	return nil
}

func (c *core) sendDecide() {
	logger := c.newLogger()

	msgTyp := MsgTypeDecide
	sub := c.current.CommittedQC()

	payload, err := Encode(sub)
	if err != nil {
		logger.Error("Failed to encode", "msg", msgTyp, "err", err)
		return
	}
	c.broadcast(msgTyp, payload)
	logger.Trace("sendDecide", "msg view", sub.View, "proposal", sub.Hash)
}

func (c *core) handleDecide(data *hotstuff.Message, src hotstuff.Validator) error {
	logger := c.newLogger()

	var (
		msg    *hotstuff.QuorumCert
		msgTyp = MsgTypeDecide
	)
	if err := data.Decode(&msg); err != nil {
		logger.Trace("Failed to decode", "msg", msgTyp, "err", err)
		return errFailedDecodeCommit
	}
	if err := c.checkView(msgTyp, msg.View); err != nil {
		logger.Trace("Failed to check view", "msg", msgTyp, "err", err)
		return err
	}
	if err := c.checkMsgFromProposer(src); err != nil {
		logger.Trace("Failed to check proposer", "msg", msgTyp, "err", err)
		return err
	}
	if err := c.checkPreCommittedQC(msg); err != nil {
		logger.Trace("Failed to check prepareQC", "msg", msgTyp, "err", err)
		return err
	}
	if err := c.signer.VerifyQC(msg); err != nil {
		logger.Trace("Failed to check verify qc", "msg", msgTyp, "err", err)
		return err
	}

	// ensure the block hash is the correct one
	blockHash := msg.Hash
	lockedBlock := c.current.Proposal().(*types.Block)
	emptyHash := common.Hash{}
	if lockedBlock == nil {
		logger.Trace("Locked block is nil", "msg", msgTyp, "src", src)
		return fmt.Errorf("invalid block")
	} else if blockHash == emptyHash || lockedBlock.Hash() != blockHash {
		logger.Trace("Failed to check block hash", "msg", msgTyp, "src", src, "expect block", lockedBlock.Hash(), "got", blockHash)
		return fmt.Errorf("invalid block")
	}

	currHash := c.current.Proposal().Hash()
	if currHash != msg.Hash {
		logger.Trace("Failed to check commitQC", "expect node", currHash, "got", msg.Hash)
		return fmt.Errorf("invalid block")
	}
	logger.Trace("handleDecide", "msg", msgTyp, "address", src.Address(), "msg view", msg.View, "proposal", msg.Hash)

	if c.IsProposer() && c.currentState() == StateCommitted {
		if err := c.backend.Commit(c.current.Proposal()); err != nil {
			logger.Trace("Failed to commit proposal", "err", err)
			return err
		}
	}

	if !c.IsProposer() && c.currentState() >= StatePreCommitted && c.currentState() < StateCommitted {
		c.current.SetState(StateCommitted)
		c.current.SetCommittedQC(c.current.PreCommittedQC())
		if err := c.backend.Commit(c.current.Proposal()); err != nil {
			logger.Trace("Failed to commit proposal", "err", err)
			return err
		}
	}

	c.startNewRound(common.Big0)
	return nil
}

// handleFinalCommitted start new round if consensus engine accept notify signal from miner.worker.
// signals should be related with sync header or body. in fact, we DONT need this function to start an new round,
// because that the function `startNewRound` will sync header to preparing new consensus round args.
// we just kept it here for backup.
func (c *core) handleFinalCommitted(header *types.Header) error {
	logger := c.newLogger()
	logger.Trace("handleFinalCommitted")
	if header.Number.Uint64() > c.currentView().Height.Uint64() {
		c.startNewRound(common.Big0)
	}
	return nil
}
