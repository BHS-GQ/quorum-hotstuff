package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/hotstuff"
	"github.com/ethereum/go-ethereum/core/types"
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

func buildRoundStartQC(lastBlock *types.Block) (*hotstuff.QuorumCert, error) {
	qc := &hotstuff.QuorumCert{
		View: &hotstuff.View{
			Round:  big.NewInt(0),
			Height: lastBlock.Number(),
		},
		Code: hotstuff.MsgTypePrepareVote,
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
