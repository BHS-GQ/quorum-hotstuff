package backend

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
)

const (
	inmemorySnapshots = 128 // Number of recent vote snapshots to keep in memory
	inmemoryPeers     = 1000
	inmemoryMessages  = 1024
)

// HotStuff protocol constants.
var (
	defaultDifficulty = big.NewInt(1)
	nilUncleHash      = types.CalcUncleHash(nil) // Always Keccak256(RLP([])) as uncles are meaningless outside of PoW.
	emptyNonce        = types.BlockNonce{}
	now               = time.Now
)

func (s *Backend) Author(header *types.Header) (common.Address, error) {
	return s.signer.RecoverSigner(header)
}

func (s *Backend) VerifyHeader(chain consensus.ChainHeaderReader, header *types.Header, seal bool) error {
	return s.verifyHeader(chain, header, nil, seal)
}

func (s *Backend) VerifyHeaders(chain consensus.ChainHeaderReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	abort := make(chan struct{})
	results := make(chan error, len(headers))
	go func() {
		for i, header := range headers {
			seal := false
			if seals != nil && len(seals) > i {
				seal = seals[i]
			}
			err := s.verifyHeader(chain, header, headers[:i], seal)

			select {
			case <-abort:
				return
			case results <- err:
			}
		}
	}()
	return abort, results
}

func (s *Backend) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	if len(block.Uncles()) > 0 {
		return hs.ErrInvalidUncleHash
	}
	return nil
}

func (s *Backend) Prepare(chain consensus.ChainHeaderReader, header *types.Header) error {
	// unused fields, force to set to empty
	header.Coinbase = s.Address()
	header.Nonce = emptyNonce
	header.MixDigest = types.HotstuffDigest

	// copy the parent extra data as the header extra data
	number := header.Number.Uint64()
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	// use the same difficulty for all blocks
	header.Difficulty = defaultDifficulty

	// add validators in snapshot to extraData's validators section
	valset := s.snap()
	extra, err := s.signer.BuildPrepareExtra(header, valset)
	if err != nil {
		return err
	}
	header.Extra = extra

	// set header's timestamp
	header.Time = parent.Time + s.config.BlockPeriod
	if header.Time < uint64(time.Now().Unix()) {
		header.Time = uint64(time.Now().Unix())
	}

	return nil
}

func (s *Backend) Finalize(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header) {
	// No block rewards in HotStuff, so the state remains as is and uncles are dropped
	header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))
	header.UncleHash = nilUncleHash
}

func (s *Backend) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs []*types.Transaction,
	uncles []*types.Header, receipts []*types.Receipt) (*types.Block, error) {
	/// No block rewards in HotStuff, so the state remains as is and uncles are dropped
	header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))
	header.UncleHash = nilUncleHash

	// Assemble and return the final block for sealing
	return types.NewBlock(header, txs, nil, receipts, trie.NewStackTrie(nil)), nil
}

func (s *Backend) Seal(chain consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) (err error) {
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

	delay := time.Until(time.Unix(int64(block.Header().Time), 0))

	s.logger.Trace("WorkerSealNewBlock", "address", s.Address(), "hash", block.Hash(), "number", block.Number(), "delay", delay.Seconds())

	go func() {
		// wait for the timestamp of header, use this to adjust the block period
		select {
		case <-time.After(delay):
		case <-stop:
			results <- nil
			return
		}

		// get the proposed block hash and clear it if the seal() is completed.
		s.sealMu.Lock()
		s.proposedBlockHash = block.Hash()

		defer func() {
			s.proposedBlockHash = common.Hash{}
			s.sealMu.Unlock()
		}()
		// post block into HotStuff engine
		go s.EventMux().Post(hs.RequestEvent{
			Block: block,
		})
		for {
			select {
			case result := <-s.commitCh:
				// if the block hash and the hash from channel are the same,
				// return the result. Otherwise, keep waiting the next hash.
				if result != nil && block.Hash() == result.Hash() {
					results <- result
					return
				}
			case <-stop:
				results <- nil
				return
			}
		}
	}()

	return nil
}

func (s *Backend) SealHash(header *types.Header) common.Hash {
	return s.signer.HeaderHash(header)
}

// useless
func (s *Backend) CalcDifficulty(chain consensus.ChainHeaderReader, time uint64, parent *types.Header) *big.Int {
	return new(big.Int)
}

func (s *Backend) APIs(chain consensus.ChainHeaderReader) []rpc.API {
	return []rpc.API{{
		Namespace: "HotStuff",
		Version:   "1.0",
		Service:   &API{chain: chain, hotstuff: s},
		Public:    true,
	}}
}

// Start implements consensus.HotStuff.Start
func (s *Backend) Start(chain consensus.ChainReader, currentBlock func() *types.Block, hasBadBlock func(db ethdb.Reader, hash common.Hash) bool) error {
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	if s.coreStarted {
		return ErrStartedEngine
	}

	// clear previous data
	s.proposedBlockHash = common.Hash{}
	if s.commitCh != nil {
		close(s.commitCh)
	}
	s.commitCh = make(chan *types.Block, 1)

	s.chain = chain
	s.currentBlock = currentBlock
	s.hasBadBlock = hasBadBlock

	if err := s.core.Start(); err != nil {
		return err
	}

	s.coreStarted = true
	return nil
}

// Stop implements consensus.HotStuff.Stop
func (s *Backend) Stop() error {
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	if !s.coreStarted {
		return ErrStoppedEngine
	}
	if err := s.core.Stop(); err != nil {
		return err
	}
	s.coreStarted = false
	return nil
}

func (s *Backend) Close() error {
	return nil
}

// verifyHeader checks whether a header conforms to the consensus rules.The
// caller may optionally pass in a batch of parents (ascending order) to avoid
// looking those up from the database. This is useful for concurrently verifying
// a batch of new headers.
func (s *Backend) verifyHeader(chain consensus.ChainHeaderReader, header *types.Header, parents []*types.Header, seal bool) error {
	if header.Number == nil {
		return hs.ErrUnknownBlock
	}
	// Ensure that the mix digest is zero as we don't have fork protection currently
	if header.MixDigest != types.HotstuffDigest {
		return hs.ErrInvalidMixDigest
	}
	// Ensure that the block doesn't contain any uncles which are meaningless in HotStuff
	if header.UncleHash != nilUncleHash {
		return hs.ErrInvalidUncleHash
	}
	// Ensure that the block's difficulty is meaningful (may not be correct at this point)
	if header.Difficulty == nil || header.Difficulty.Cmp(defaultDifficulty) != 0 {
		return hs.ErrInvalidDifficulty
	}

	// The genesis block is the always valid dead-end
	number := header.Number.Uint64()
	if number == 0 {
		return nil
	}

	var parent *types.Header
	if len(parents) > 0 {
		parent = parents[len(parents)-1]
	} else {
		parent = chain.GetHeader(header.ParentHash, number-1)
	}
	if parent == nil || parent.Number.Uint64() != number-1 || parent.Hash() != header.ParentHash {
		return consensus.ErrUnknownAncestor
	}

	if header.Time > uint64(now().Unix()) {
		return consensus.ErrFutureBlock
	}

	if header.Time < parent.Time+s.config.BlockPeriod {
		s.logger.Debug("TIME DIFF", "header", header.Time, "parent + BP", parent.Time+s.config.BlockPeriod)
		return hs.ErrInvalidTimestamp
	}

	// [TODO] Verify validators in extraData. Validators in snapshot and extraData should be the same.

	// Resolve auth key and check against signers
	if _, err := s.signer.RecoverSigner(header); err != nil {
		return err
	}

	snap := s.snap().Copy()
	return s.signer.VerifyHeader(header, snap, seal)
}
