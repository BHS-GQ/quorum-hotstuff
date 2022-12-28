package core

import "github.com/ethereum/go-ethereum/common"

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
	if proposal := c.current.Proposal(); proposal != nil && proposal.Hash() == blockHash {
		return true
	}
	if req := c.current.PendingRequest(); req != nil && req.Proposal != nil && req.Proposal.Hash() == blockHash {
		return true
	}
	return false
}