package backend

import (
	"github.com/ethereum/go-ethereum/consensus"
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
	"github.com/ethereum/go-ethereum/core/types"
)

func (s *Backend) MockSeal(chain consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) (err error) {
	// update the block header timestamp and signature and propose the block to core engine
	header := block.Header()
	number := header.Number.Uint64()

	// Bail out if we're unauthorized to sign a block
	snap := s.snap()
	if _, v := snap.GetByAddress(s.Address()); v == nil {
		return hs.ErrUnauthorized
	}

	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}

	// Sign the HotstuffExtra.Seal portion with ECDSA
	if err = s.signer.SignerSeal(header); err != nil {
		return err
	}
	block = block.WithSeal(header)

	go s.EventMux().Post(hs.RequestEvent{Block: block})

	s.logger.Trace("WorkerSealNewBlock", "address", s.Address(), "hash", block.Hash(), "number", block.Number())

	return nil
}
