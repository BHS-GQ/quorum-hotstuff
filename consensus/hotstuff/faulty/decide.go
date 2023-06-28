package faulty

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
	"github.com/ethereum/go-ethereum/core/types"
)

// handleCommitVote implement description as follow:
// ```
// leader wait for (n n f) votes: V ← {v | matchingMsg(v, commit, curView)}
//
//	commitQC ← QC(V )
//	broadcast Msg(decide, ⊥, commitQC )
//
// ```
func (c *Core) handleCommitVote(data *hs.Message) error {
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
	if err := c.checkView(data.View); err != nil {
		logger.Trace("Failed to check view", "msg", code, "src", src, "err", err)
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
		return hs.ErrInvalidNode
	}

	// queue vote into messageSet to ensure that at least 2/3 validator vote at the same step.
	if err := c.current.AddCommitVote(data); err != nil {
		logger.Trace("Failed to add vote", "msg", code, "src", src, "err", err)
		return hs.ErrAddPreCommitVote
	}

	logger.Trace("handleCommitVote", "msg", code, "src", src, "hash", vote)

	// assemble committed signatures to reorg the locked block, and create `commitQC` at the same time.
	if size := c.current.CommitVoteSize(); size >= c.valSet.Q() && c.currentState() == hs.StatePreCommitted {
		commitQC, err := c.messagesToQC(code)
		if err != nil {
			logger.Trace("Failed to assemble commitQC", "msg", code, "err", err)
			return err
		}

		if c.isFaultTriggered(hs.BadDecideBadBlock, uint64(4), uint64(0)) {
			logger.Trace("FAULT TRIGGERED -- Send WrongDecide")

			// Leader sends bad QuorumCert; we ruin BLSSignature
			// to imitate not enough votes being provided
			commitQC.BLSSignature[0] += 1
		}

		sealedBlock, err := c.backend.SealBlock(lockedBlock, commitQC)
		if err != nil {
			logger.Trace("Failed to assemble committed proposal", "msg", code, "err", err)
			return err
		}

		if err := c.acceptCommitQC(sealedBlock, commitQC); err != nil {
			logger.Trace("Failed to accept commitQC", "msg", code, "err", err)
			return err
		}
		logger.Trace("acceptCommit", "msg", code, "msgSize", size)

		c.sendDecide(sealedBlock.Hash(), commitQC)
	}

	return nil
}

func (c *Core) sendDecide(block common.Hash, commitQC *hs.QuorumCert) {
	logger := c.newLogger()

	code := hs.MsgTypeDecide
	msg := &hs.Diploma{
		BlockHash: block,
		CommitQC:  commitQC,
	}
	payload, err := hs.Encode(msg)
	if err != nil {
		logger.Trace("Failed to encode", "msg", code, "err", err)
		return
	}
	c.broadcast(code, payload)

	logger.Trace("sendDecide", "msg", code, "node", commitQC.ProposedBlock)
}

// handleDecide repo receive MsgDecide and try to commit the final block.
func (c *Core) handleDecide(data *hs.Message) error {
	var (
		logger = c.newLogger()
		code   = data.Code
		src    = data.Address
		msg    *hs.Diploma
	)

	// Accept bad QuorumCert and start new round
	if c.isFaultTriggered(hs.BadDecideBadBlock, uint64(4), uint64(0)) && c.IsProposer() {
		logger.Trace("FAULT TRIGGERED -- Accept WrongDecide")
		if err := c.commit(c.current.LockedBlock()); err != nil {
			logger.Trace("Failed to commit proposal", "msg", code, "err", err)
			return err
		}
		c.startNewRound(common.Big0)
		return nil
	}

	// check message
	if err := data.Decode(&msg); err != nil {
		logger.Trace("Failed to decode", "msg", code, "src", src, "err", err)
		return hs.ErrFailedDecodeCommit
	}
	if err := c.checkView(data.View); err != nil {
		logger.Trace("Failed to check view", "msg", code, "src", src, "err", err)
		return err
	}
	if err := c.checkMsgSource(src); err != nil {
		logger.Trace("Failed to check proposer", "msg", code, "src", src, "err", err)
		return err
	}

	// ensure commitQC is legal
	commitQC := msg.CommitQC
	if err := c.verifyQC(data, commitQC); err != nil {
		logger.Trace("Failed to verify commitQC", "msg", code, "src", src, "err", err)
		return err
	}

	// ensure the block hash is the correct one
	// [NOTE] Header comparison does not include seal
	blockHash := msg.BlockHash
	lockedBlock := c.current.LockedBlock()
	if lockedBlock == nil {
		logger.Trace("Locked block is nil", "msg", code, "src", src)
		return hs.ErrInvalidBlock
	} else if blockHash == hs.EmptyHash || lockedBlock.Hash() != blockHash {
		logger.Trace("Failed to check block hash", "msg", code, "src", src, "expect block", lockedBlock.Hash(), "got", blockHash)
		return hs.ErrInvalidBlock
	}

	if curNode := c.current.ProposedBlock(); curNode == nil || curNode.Block == nil {
		logger.Trace("Current node is nil")
		return hs.ErrInvalidNode
	} else if curNode.Hash() != commitQC.ProposedBlock {
		logger.Trace("Failed to check commitQC", "expect node", curNode.Hash(), "got", commitQC.ProposedBlock)
		return hs.ErrInvalidQC
	} else if curNode.Block.Hash() != blockHash {
		logger.Trace("Failed to check node", "expect node block hash", curNode.Block.Hash(), "got", blockHash)
		return hs.ErrInvalidBlock
	}

	logger.Trace("handleDecide", "msg", code, "src", src, "node", commitQC.ProposedBlock)

	// accept commitQC and commit block to miner
	if c.IsProposer() && c.currentState() == hs.StateCommitted {
		if err := c.commit(c.current.LockedBlock()); err != nil {
			logger.Trace("Failed to commit proposal", "msg", code, "err", err)
			return err
		}
	}
	if !c.IsProposer() && c.currentState() == hs.StatePreCommitted {
		sealedBlock, err := c.backend.SealBlock(lockedBlock, commitQC)
		if err != nil {
			logger.Trace("Failed to assemble committed proposal", "msg", code, "err", err)
			return err
		}
		if err := c.acceptCommitQC(sealedBlock, commitQC); err != nil {
			logger.Trace("Failed to accept commitQC", "msg", code, "err", err)
			return err
		}
		if err := c.commit(sealedBlock); err != nil {
			logger.Trace("Failed to commit proposal", "err", err)
			return err
		}
	}

	c.startNewRound(common.Big0)
	return nil
}

func (c *Core) acceptCommitQC(sealedBlock *types.Block, commitQC *hs.QuorumCert) error {
	if err := c.current.SetSealedBlock(sealedBlock); err != nil {
		return err
	}
	if err := c.current.SetCommittedQC(commitQC); err != nil {
		return err
	}
	c.current.SetState(hs.StateCommitted)
	return nil
}

func (c *Core) commit(sealedBlock *types.Block) error {
	if lockedBlock := c.current.LockedBlock(); lockedBlock == nil {
		return fmt.Errorf("locked block is nil")
	} else if lockedBlock.Hash() != sealedBlock.Hash() {
		return fmt.Errorf("expect locked block %v, got %v", lockedBlock.Hash(), sealedBlock.Hash())
	}

	if c.current.executed == nil || c.current.executed.Block == nil || c.current.executed.Block.Hash() != sealedBlock.Hash() {
		if c.IsProposer() {
			c.current.executed = &consensus.ExecutedBlock{Block: sealedBlock}
		} else {
			executed, err := c.backend.ExecuteBlock(sealedBlock)
			if err != nil {
				return fmt.Errorf("failed to execute block %v, err: %v", sealedBlock.Hash(), err)
			}
			executed.Block = sealedBlock
			c.current.executed = executed
		}
	}

	return c.backend.Commit(c.current.executed)
}

// handleFinalCommitted start new round if consensus engine accept notify signal from miner.worker.
// signals should be related with sync header or body. in fact, we DONT need this function to start an new round,
// because that the function `startNewRound` will sync header to preparing new consensus round args.
// we just kept it here for backup.
func (c *Core) handleFinalCommitted(header *types.Header) error {
	logger := c.newLogger()
	if height := header.Number.Uint64(); height >= c.current.HeightU64() {
		logger.Trace("handleFinalCommitted", "height", height)
		c.startNewRound(common.Big0)
	}
	return nil
}
