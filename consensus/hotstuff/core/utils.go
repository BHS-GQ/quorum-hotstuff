package core

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

func (c *core) proposer() common.Address {
	return c.valSet.GetProposer().Address()
}

func (c *core) HeightU64() uint64 {
	if c.current == nil {
		return 0
	} else {
		return c.current.HeightU64()
	}
}

// checkView checks the Message sequence remote msg view should not be nil(local view WONT be nil).
// if the view is ahead of current view we name the Message to be future Message, and if the view is behind
// of current view, we name it as old Message. `old Message` and `invalid Message` will be dropped. and we use t
// he storage of `backlog` to cache the future Message, it only allow the Message height not bigger than
// `current height + 1` to ensure that the `backlog` memory won't be too large, it won't interrupt the consensus
// process, because that the `core` instance will sync block until the current height to the correct value.
//
//
// todo(fuk):if the view is equal the current view, compare the Message type and round state, with the right
// round state sequence, Message ahead of certain state is `old Message`, and Message behind certain
// state is `future Message`. Message type and round state table as follow:
func (c *core) checkView(view *hs.View) error {
	if view == nil || view.Height == nil || view.Round == nil {
		return errInvalidMessage
	}

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

// sendEvent sends events to mux
func (c *core) sendEvent(ev interface{}) {
	c.backend.EventMux().Post(ev)
}

func (c *core) newLogger() log.Logger {
	logger := c.logger.New("state", c.currentState(), "view", c.currentView())
	return logger
}

func (c *core) checkMsgDest() error {
	if !c.IsProposer() {
		return errNotToProposer
	}
	return nil
}

// verifyQC check and validate qc.
func (c *core) verifyQC(data *hs.Message, qc *hs.QuorumCert) error {
	if qc == nil || qc.View == nil {
		return fmt.Errorf("qc or qc.View is nil")
	}

	// skip genesis block
	if qc.HeightU64() == 0 {
		return nil
	}

	// qc fields checking
	if qc.TreeNode == hs.EmptyHash || qc.Proposer == hs.EmptyAddress || qc.BLSSignature == nil {
		return fmt.Errorf("qc.TreeNode, Proposer, Seal or BLSSig is nil")
	}

	// matching code
	if (data.Code == hs.MsgTypeNewView && qc.Code != hs.MsgTypePrepareVote) ||
		(data.Code == hs.MsgTypePrepare && qc.Code != hs.MsgTypePrepareVote) ||
		(data.Code == hs.MsgTypePreCommit && qc.Code != hs.MsgTypePrepareVote) ||
		(data.Code == hs.MsgTypeCommit && qc.Code != hs.MsgTypePreCommitVote) ||
		(data.Code == hs.MsgTypeDecide && qc.Code != hs.MsgTypeCommitVote) {
		return fmt.Errorf("qc.Code %s not matching message code", qc.Code.String())
	}

	// prepareQC's view should lower than message's view
	if data.Code == hs.MsgTypeNewView || data.Code == hs.MsgTypePrepare {
		if hdiff, rdiff := data.View.Sub(qc.View); hdiff < 0 || (hdiff == 0 && rdiff < 0) {
			return fmt.Errorf("view is invalid")
		}
	}

	// // qc.TreeNode is not node hash but block hash, only used for epoch change.
	// if c.isEpochStartQC(nil, qc) {
	// 	return c.verifyEpochStartQC(qc)
	// }

	// matching view and compare proposer and local node
	if data.Code == hs.MsgTypePreCommit || data.Code == hs.MsgTypeCommit || data.Code == hs.MsgTypeDecide {
		if qc.View.Cmp(data.View) != 0 {
			return fmt.Errorf("qc.View %v not matching message view", qc.View)
		}
		if qc.Proposer != c.proposer() {
			return fmt.Errorf("expect proposer %v, got %v", c.proposer(), qc.Proposer)
		}
		if node := c.current.TreeNode(); node == nil {
			return fmt.Errorf("current node is nil")
		} else if node.Hash() != qc.TreeNode {
			return fmt.Errorf("expect node %v, got %v", node.Hash(), qc.TreeNode)
		}
	}

	// resturct msg payload and compare msg.hash with qc.hash
	msg := hs.NewCleanMessage(qc.View, qc.Code, qc.TreeNode.Bytes())
	if _, err := msg.PayloadNoSig(); err != nil {
		return fmt.Errorf("payload no sig")
	}

	sealHash := qc.SealHash()
	msgHash, err := msg.Hash()
	if err != nil {
		return err
	}
	if msgHash != sealHash {
		return fmt.Errorf("expect qc hash %v, got %v", msgHash, sealHash)
	}

	// find the correct validator set and verify seal & committed seals
	return c.signer.VerifyQC(qc)
}

func buildRoundStartQC(lastBlock *types.Block) (*hs.QuorumCert, error) {
	qc := &hs.QuorumCert{
		View: &hs.View{
			Round:  big.NewInt(0),
			Height: lastBlock.Number(),
		},
		Code: hs.MsgTypePrepareVote,
	}

	// allow genesis node and proposer to be empty
	if lastBlock.NumberU64() == 0 {
		qc.Proposer = common.Address{}
		qc.TreeNode = common.HexToHash("0x12345")
	} else {
		qc.Proposer = lastBlock.Coinbase()
		qc.TreeNode = lastBlock.Hash()
	}

	// [TODO] Add Aggregated BLSSignature before committing during signer sealing
	extra, err := types.ExtractHotstuffExtra(lastBlock.Header())
	if err != nil {
		return nil, err
	}
	if extra.Seal == nil || extra.BLSSignature == nil {
		return nil, errInvalidNode
	}

	qc.BLSSignature = extra.BLSSignature
	return qc, nil
}

// sendVote repo send kinds of vote to leader, use `current.node` after repo `prepared`.
func (c *core) sendVote(code hs.MsgType) {
	logger := c.newLogger()

	// Fetch and sign vote
	vote := c.current.UnsignedVote(code)
	if vote == nil {
		logger.Error("Failed to send vote", "msg", code, "err", "current vote is nil")
		return
	}
	unsignedVoteBytes, err := hs.Encode(vote)
	if err != nil {
		logger.Error("Failed to send vote", "msg", code, "err", "could not encode unsigned vote")
		return
	}
	signedVoteBytes, err := c.signer.BLSSign(unsignedVoteBytes)
	if err != nil {
		logger.Error("Failed to send vote", "msg", code, "err", "could not sign unsigned vote bytes")
		return
	}
	vote.BLSSignature = signedVoteBytes
	payload, err := hs.Encode(vote)
	if err != nil {
		logger.Error("Failed to encode", "msg", code, "err", err)
		return
	}

	c.broadcast(code, payload)
	prefix := fmt.Sprintf("send%s", code.String())
	logger.Trace(prefix, "msg", code, "hash", vote)
}

func (c *core) checkMsgSource(src common.Address) error {
	if !c.valSet.IsProposer(src) {
		return errNotFromProposer
	}
	return nil
}

// checkNode repo compare remote node with local node
func (c *core) checkNode(node *hs.TreeNode, compare bool) error {
	if node == nil || node.Parent == hs.EmptyHash ||
		node.Block == nil || node.Block.Header() == nil {
		return errInvalidNode
	}

	if !compare {
		return nil
	}

	local := c.current.TreeNode()
	if local == nil {
		return fmt.Errorf("current node is nil")
	}
	if local.Hash() != node.Hash() {
		return fmt.Errorf("expect node %v but got %v", local.Hash(), node.Hash())
	}
	if local.Block.Hash() != node.Block.Hash() {
		return fmt.Errorf("expect block %v but got %v", local.Block.Hash(), node.Block.Hash())
	}
	return nil
}

// checkBlock check the extend relationship between remote block and latest chained block.
// ensure that the remote block equals to locked block if it exist.
func (c *core) checkBlock(block *types.Block) error {
	lastChainedBlock := c.current.LastChainedBlock()
	if lastChainedBlock.NumberU64()+1 != block.NumberU64() {
		return fmt.Errorf("expect block number %v, got %v", lastChainedBlock.NumberU64()+1, block.NumberU64())
	}
	if lastChainedBlock.Hash() != block.ParentHash() {
		return fmt.Errorf("expect parent hash %v, got %v", lastChainedBlock.Hash(), block.ParentHash())
	}

	if lockedBlock := c.current.LockedBlock(); lockedBlock != nil {
		if block.NumberU64() != lockedBlock.NumberU64() {
			return fmt.Errorf("expect locked block number %v, got %v", lockedBlock.NumberU64(), block.NumberU64())
		}
		if block.Hash() != lockedBlock.Hash() {
			return fmt.Errorf("expect locked block hash %v, got %v", lockedBlock.Hash(), block.Hash())
		}
	}

	return nil
}
