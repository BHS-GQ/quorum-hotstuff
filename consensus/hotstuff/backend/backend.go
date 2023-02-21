// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package hotstuff implements the scalable hotstuff consensus algorithm.

package backend

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
	hsc "github.com/ethereum/go-ethereum/consensus/hotstuff/core"
	snr "github.com/ethereum/go-ethereum/consensus/hotstuff/signer"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/trie"
	lru "github.com/hashicorp/golang-lru"
)

const (
	// fetcherID is the ID indicates the block is from HotStuff engine
	fetcherID = "hotstuff"
)

// HotStuff is the scalable hotstuff consensus engine
type Backend struct {
	config       *hs.Config
	db           ethdb.Database // Database to store and retrieve necessary information
	core         hs.CoreEngine
	signer       hs.Signer
	chain        consensus.ChainReader
	currentBlock func() *types.Block
	hasBadBlock  func(db ethdb.Reader, hash common.Hash) bool
	logger       log.Logger

	valset         hs.ValidatorSet
	recents        *lru.ARCCache // Snapshots for recent block to speed up reorgs
	recentMessages *lru.ARCCache // the cache of peer's messages
	knownMessages  *lru.ARCCache // the cache of self messages

	// The channels for hotstuff engine notifications
	sealMu            sync.Mutex
	commitCh          chan *types.Block
	proposedBlockHash common.Hash
	coreStarted       bool
	sigMu             sync.RWMutex // Protects the address fields
	consenMu          sync.Mutex   // Ensure a round can only start after the last one has finished
	coreMu            sync.RWMutex

	// event subscription for ChainHeadEvent event
	broadcaster consensus.Broadcaster

	executeFeed event.Feed // event subscription for executed state
	eventMux    *event.TypeMux

	proposals map[common.Address]bool // Current list of proposals we are pushing
}

func New(
	config *hs.Config,
	privateKey *ecdsa.PrivateKey,
	db ethdb.Database,
	valset hs.ValidatorSet,
	blsInfo *types.BLSInfo,
) *Backend {
	recents, _ := lru.NewARC(inmemorySnapshots)
	recentMessages, _ := lru.NewARC(inmemoryPeers)
	knownMessages, _ := lru.NewARC(inmemoryMessages)

	signer := snr.NewSigner(privateKey, byte(hs.MsgTypePrepareVote), blsInfo)
	backend := &Backend{
		config:         config,
		db:             db,
		logger:         log.New(),
		valset:         valset,
		commitCh:       make(chan *types.Block, 1),
		coreStarted:    false,
		eventMux:       new(event.TypeMux),
		signer:         signer,
		recentMessages: recentMessages,
		knownMessages:  knownMessages,
		recents:        recents,
		proposals:      make(map[common.Address]bool),
	}

	backend.core = hsc.New(backend, config, signer, db, valset)
	return backend
}

// Address implements hs.Backend.Address
func (s *Backend) Address() common.Address {
	return s.signer.Address()
}

// Validators implements hs.Backend.Validators
func (s *Backend) Validators() hs.ValidatorSet {
	return s.snap()
}

// EventMux implements hs.Backend.EventMux
func (s *Backend) EventMux() *event.TypeMux {
	return s.eventMux
}

// Broadcast implements hs.Backend.Broadcast
func (s *Backend) Broadcast(valSet hs.ValidatorSet, payload []byte) error {
	// send to others
	if err := s.Gossip(valSet, payload); err != nil {
		return err
	}
	// send to self
	msg := hs.MessageEvent{
		Src:     s.Address(),
		Payload: payload,
	}
	go s.EventMux().Post(msg)
	return nil
}

// Broadcast implements hs.Backend.Gossip
func (s *Backend) Gossip(valSet hs.ValidatorSet, payload []byte) error {
	hash := hs.RLPHash(payload)
	s.knownMessages.Add(hash, true)

	targets := make(map[common.Address]bool)
	for _, val := range valSet.List() { // hotstuff/validator/default.go - defaultValidator
		if val.Address() != s.Address() {
			targets[val.Address()] = true
		}
	}
	if s.broadcaster != nil && len(targets) > 0 {
		ps := s.broadcaster.FindPeers(targets)
		for addr, p := range ps {
			ms, ok := s.recentMessages.Get(addr)
			var m *lru.ARCCache
			if ok {
				m, _ = ms.(*lru.ARCCache)
				if _, k := m.Get(hash); k {
					// This peer had this event, skip it
					continue
				}
			} else {
				m, _ = lru.NewARC(inmemoryMessages)
			}

			m.Add(hash, true)
			s.recentMessages.Add(addr, m)
			go p.SendConsensus(hotstuffMsg, payload)
		}
	}
	return nil
}

// Unicast implements hs.Backend.Unicast
func (s *Backend) Unicast(valSet hs.ValidatorSet, payload []byte) error {
	msg := hs.MessageEvent{Src: s.Address(), Payload: payload}
	leader := valSet.GetProposer()
	target := leader.Address()
	hash := hs.RLPHash(payload)
	s.knownMessages.Add(hash, true)

	// send to self
	if s.Address() == target {
		go s.EventMux().Post(msg)
		return nil
	}

	// send to other peer
	if s.broadcaster != nil {
		if p := s.broadcaster.FindPeer(target); p != nil {
			ms, ok := s.recentMessages.Get(target)
			var m *lru.ARCCache
			if ok {
				m, _ = ms.(*lru.ARCCache)
				if _, k := m.Get(hash); k {
					return nil
				}
			} else {
				m, _ = lru.NewARC(inmemoryMessages)
			}
			m.Add(hash, true)
			s.recentMessages.Add(target, m)
			go func() {
				if err := p.SendConsensus(hotstuffMsg, payload); err != nil {
					s.logger.Error("unicast message failed", "err", err)
				}
			}()
		}
	}
	return nil
}

// SealBlock seals block within consensus by
// adding PrepareQC BLS AggSig to block header
func (s *Backend) SealBlock(block *types.Block, commitQC *hs.QuorumCert) (*types.Block, error) {

	// check proposal
	h := block.Header()
	if h == nil {
		s.logger.Error("Invalid proposal precommit")
		return nil, errInvalidProposal
	}

	encodedQC, err := hs.Encode(commitQC)
	if err != nil {
		return nil, err
	}
	if err := h.SetEncodedQC(encodedQC); err != nil {
		return nil, err
	}

	return block.WithSeal(h), nil
}

func (s *Backend) Commit(executed *consensus.ExecutedBlock) error {
	block := executed.Block

	if executed == nil || executed.Block == nil {
		return fmt.Errorf("invalid executed block")
	}
	s.executeFeed.Send(*executed)

	s.logger.Info("Committed", "address", s.Address(), "hash", block.Hash(), "number", block.Number().Uint64())

	// - if the proposed and committed blocks are the same, send the proposed hash
	//   to commit channel, which is being watched inside the engine.Seal() function.
	// - otherwise, we try to insert the block.
	// -- if success, the ChainHeadEvent event will be broadcasted, try to build
	//    the next block and the previous Seal() will be stopped (need to check this --- saber).
	// -- otherwise, an error will be returned and a round change event will be fired.
	if s.proposedBlockHash == block.Hash() {
		// feed block hash to Seal() and wait the Seal() result
		s.commitCh <- block
		return nil
	}
	if s.broadcaster != nil {
		s.broadcaster.Enqueue(fetcherID, block)
	}
	return nil
}

// Verify implements hs.Backend.Verify
func (s *Backend) Verify(block *types.Block) (time.Duration, error) {
	// check bad block
	if s.HasBadProposal(block.Hash()) {
		return 0, core.ErrBlacklistedHash
	}

	// check block body
	txnHash := types.DeriveSha(block.Transactions(), trie.NewStackTrie(nil))
	uncleHash := types.CalcUncleHash(block.Uncles())
	if txnHash != block.Header().TxHash {
		return 0, errMismatchTxhashes
	}
	if uncleHash != nilUncleHash {
		return 0, errInvalidUncleHash
	}

	// verify the header of proposed block
	err := s.VerifyHeader(s.chain, block.Header(), false)
	if err == nil {
		return 0, nil
	} else if err == consensus.ErrFutureBlock {
		return time.Unix(int64(block.Header().Time), 0).Sub(now()), consensus.ErrFutureBlock
	}
	return 0, err
}

func (s *Backend) LastProposal() (*types.Block, common.Address) {
	var (
		proposer common.Address
		err      error
	)

	if s.currentBlock == nil {
		return nil, common.Address{}
	}

	block := s.currentBlock()
	if block.Number().Cmp(common.Big0) > 0 {
		if proposer, err = s.Author(block.Header()); err != nil {
			s.logger.Error("Failed to get block proposer", "err", err)
			return nil, common.Address{}
		}
	}

	// Return header only block here since we don't need block body
	return block, proposer
}

// HasProposal implements hs.Backend.HashBlock
func (s *Backend) HasProposal(hash common.Hash, number *big.Int) bool {
	return s.chain.GetHeader(hash, number.Uint64()) != nil
}

// GetSpeaker implements hs.Backend.GetProposer
func (s *Backend) GetProposer(number uint64) common.Address {
	if header := s.chain.GetHeaderByNumber(number); header != nil {
		a, _ := s.Author(header)
		return a
	}
	return common.Address{}
}

func (s *Backend) HasBadProposal(hash common.Hash) bool {
	if s.hasBadBlock == nil {
		return false
	}
	return s.hasBadBlock(s.db, hash)
}

func (s *Backend) ExecuteBlock(block *types.Block) (*consensus.ExecutedBlock, error) {
	state, receipts, allLogs, err := s.chain.ExecuteBlock(block)
	if err != nil {
		return nil, err
	}
	return &consensus.ExecutedBlock{
		State:    state,
		Block:    block,
		Receipts: receipts,
		Logs:     allLogs,
	}, nil
}
