package hotstuff

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type Signer interface {
	Address() common.Address

	// BLS

	// BLSSign signs data with partial private key of validator
	BLSSign(data []byte) ([]byte, error)

	// BLSRecoverAggSig recovers aggregated signature for data
	// given partially-signed signatures in sigShares. Intended
	// for HotStuff leaders
	BLSRecoverAggSig(data []byte, sigShares [][]byte) ([]byte, error)

	// BLSVerifyAggSig verifies aggregated signature over data.
	// Intended for HotStuff replicas
	BLSVerifyAggSig(data []byte, aggSig []byte) error

	// VerifyQC verifies a QC code, view, and hash given the aggsig
	// Intended for HotStuff replicas
	VerifyQC(qc *QuorumCert) error

	// Not BLS

	// Sign signs data for ECDSA authetication
	Sign(hash common.Hash) ([]byte, error)

	// SealHash returns block hash before sealing
	HeaderHash(header *types.Header) (hash common.Hash)

	// Recover extracts the proposer address from a signed header
	HeaderRecoverProposer(header *types.Header) (common.Address, error)

	// SignerSeal proposer signs header with ECDSA
	SignerSeal(h *types.Header) error

	// CheckSignature checks if ECDSA-signed data comes from a validator
	CheckSignature(valSet ValidatorSet, hash common.Hash, sig []byte) (common.Address, error)

	// BuildPrepareExtra builds a block's Extra field for Prepare stage
	BuildPrepareExtra(header *types.Header, valSet ValidatorSet) ([]byte, error)
}
