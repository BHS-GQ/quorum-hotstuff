package hotstuff

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type Signer interface {
	Address() common.Address

	// BLS
	BLSSign(data []byte) ([]byte, error)
	BLSRecoverAggSig(data []byte, sigShares [][]byte) ([]byte, error)
	BLSVerifyAggSig(data []byte, aggSig []byte) error

	// Not BLS
	Sign(data []byte) ([]byte, error)
	SealHash(header *types.Header) (hash common.Hash)
	Recover(header *types.Header) (common.Address, error)
	SealBeforeCommit(h *types.Header) error
	CheckSignature(valSet ValidatorSet, data []byte, sig []byte) (common.Address, error)
	PrepareExtra(header *types.Header, valSet ValidatorSet) ([]byte, error)
	VerifyQC(qc *QuorumCert) error
}
