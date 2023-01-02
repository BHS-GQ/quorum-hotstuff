package core

import (
	"github.com/ethereum/go-ethereum/common"
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
)

func (c *core) Address() common.Address {
	return c.signer.Address()
}

func (c *core) IsProposer() bool {
	return c.valSet.IsProposer(c.backend.Address())
}

func (c *core) IsCurrentProposal(blockHash common.Hash) bool {
	if c.current == nil {
		return false
	}
	if proposal := c.current.TreeNode(); proposal != nil && proposal.Hash() == blockHash {
		return true
	}
	if req := c.current.PendingRequest(); req != nil && req.Block != nil && req.Block.Hash() == blockHash {
		return true
	}
	return false
}

func (c *core) broadcast(code hs.MsgType, payload []byte) {
	logger := c.logger.New("state", c.currentState())

	// Forbid non-validator nodest to send message to leader
	if index, _ := c.valSet.GetByAddress(c.Address()); index < 0 {
		return
	}

	msg := hs.NewCleanMessage(c.currentView(), code, payload)
	payload, err := c.finalizeMessage(msg)
	if err != nil {
		logger.Error("Failed to finalize Message", "msg", msg, "err", err)
		return
	}

	switch msg.Code {
	case hs.MsgTypeNewView, hs.MsgTypePrepareVote, hs.MsgTypePreCommitVote, hs.MsgTypeCommitVote:
		// Send a vote-type message to leader

		if err = c.backend.Unicast(c.valSet, payload); err != nil {
			logger.Error("Failed to unicast Message", "msg", msg, "err", err)
		}

	case hs.MsgTypePrepare, hs.MsgTypePreCommit, hs.MsgTypeCommit, hs.MsgTypeDecide:
		// Leader broadcasts decision to replicas

		if err = c.backend.Broadcast(c.valSet, payload); err != nil {
			logger.Error("Failed to broadcast Message", "msg", msg, "err", err)
		}
	default:
		logger.Error("invalid msg type", "msg", msg)
	}
}

func (c *core) finalizeMessage(msg *hs.Message) ([]byte, error) {
	var (
		sig     []byte
		msgHash common.Hash
		err     error
	)

	// Sign Message (ECDSA)
	if _, err = msg.PayloadNoSig(); err != nil {
		return nil, err
	}
	msgHash, err = msg.Hash()
	if sig, err = c.signer.Sign(msgHash); err != nil {
		return nil, err
	} else {
		msg.Signature = sig
	}

	// Convert to payload
	return msg.Payload()
}
