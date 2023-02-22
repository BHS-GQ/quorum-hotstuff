package core

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/consensus"
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
)

func (c *Core) currentView() *hs.View {
	return &hs.View{
		Height: new(big.Int).Set(c.current.Height()),
		Round:  new(big.Int).Set(c.current.Round()),
	}
}

func (c *Core) currentState() hs.State {
	return c.current.State()
}

func (c *Core) currentProposer() hs.Validator {
	return c.valSet.GetProposer()
}

type roundState struct {
	db     ethdb.Database
	logger log.Logger
	vs     hs.ValidatorSet

	round  *big.Int
	height *big.Int
	state  hs.State

	lastChainedBlock *types.Block
	pendingRequest   *hs.Request
	node             *hs.TreeNode
	lockedBlock      *types.Block // validator's prepare proposal
	executed         *consensus.ExecutedBlock
	proposalLocked   bool

	// o(4n)
	newViews       *MessageSet // data set for newView message
	prepareVotes   *MessageSet // data set for prepareVote message
	preCommitVotes *MessageSet // data set for preCommitVote message
	commitVotes    *MessageSet // data set for commitVote message

	highQC      *hs.QuorumCert // leader highQC
	prepareQC   *hs.QuorumCert // prepareQC for repo and leader
	lockQC      *hs.QuorumCert // lockQC for repo and leader
	committedQC *hs.QuorumCert // committedQC for repo and leader
}

// newRoundState creates a new roundState instance with the given view and validatorSet
func newRoundState(db ethdb.Database, logger log.Logger, validatorSet hs.ValidatorSet, lastChainedBlock *types.Block, view *hs.View) *roundState {
	rs := &roundState{
		db:               db,
		logger:           logger,
		vs:               validatorSet.Copy(),
		round:            view.Round,
		height:           view.Height,
		state:            hs.StateAcceptRequest,
		node:             new(hs.TreeNode),
		lastChainedBlock: lastChainedBlock,
		newViews:         NewMessageSet(validatorSet),
		prepareVotes:     NewMessageSet(validatorSet),
		preCommitVotes:   NewMessageSet(validatorSet),
		commitVotes:      NewMessageSet(validatorSet),
	}
	return rs
}

// clean all votes message set for new round
func (s *roundState) update(vs hs.ValidatorSet, lastChainedBlock *types.Block, view *hs.View) *roundState {
	s.vs = vs.Copy()
	s.height = view.Height
	s.round = view.Round
	s.lastChainedBlock = lastChainedBlock
	s.newViews = NewMessageSet(vs)
	s.prepareVotes = NewMessageSet(vs)
	s.preCommitVotes = NewMessageSet(vs)
	s.commitVotes = NewMessageSet(vs)

	return s
}

func (s *roundState) View() *hs.View {
	return &hs.View{
		Round:  s.round,
		Height: s.height,
	}
}

func (s *roundState) Height() *big.Int {
	return s.height
}

func (s *roundState) HeightU64() uint64 {
	return s.height.Uint64()
}

func (s *roundState) Round() *big.Int {
	return s.round
}

func (s *roundState) RoundU64() uint64 {
	return s.round.Uint64()
}

func (s *roundState) SetState(state hs.State) {
	s.state = state
}

func (s *roundState) State() hs.State {
	return s.state
}

func (s *roundState) LastChainedBlock() *types.Block {
	return s.lastChainedBlock
}

// accept pending request from miner only for once.
func (s *roundState) SetPendingRequest(req *hs.Request) {
	if s.pendingRequest == nil {
		s.pendingRequest = req
	}
}

func (s *roundState) PendingRequest() *hs.Request {
	return s.pendingRequest
}

func (s *roundState) SetTreeNode(node *hs.TreeNode) error {
	if node == nil || node.Block == nil {
		return hs.ErrInvalidNode
	}

	s.node = node
	return nil
}

func (s *roundState) TreeNode() *hs.TreeNode {
	return s.node
}

func (s *roundState) Lock(qc *hs.QuorumCert) error {
	if s.node == nil || s.node == nil {
		return hs.ErrInvalidNode
	}

	if err := s.storeLockQC(qc); err != nil {
		return err
	}
	if err := s.storeNode(s.node); err != nil {
		return err
	}

	s.lockQC = qc
	s.lockedBlock = s.node.Block
	s.proposalLocked = true
	return nil
}

func (s *roundState) LockQC() *hs.QuorumCert {
	return s.lockQC
}

// Unlock it's happened at the start of new round, new state is `StateAcceptRequest`, and `lockQC` keep to judge safety rule
func (s *roundState) Unlock() error {
	s.pendingRequest = nil
	s.proposalLocked = false
	s.lockedBlock = nil
	s.node = nil
	s.executed = nil
	return nil
}

func (s *roundState) LockedBlock() *types.Block {
	if s.proposalLocked && s.lockedBlock != nil {
		return s.lockedBlock
	}
	return nil
}

func (s *roundState) SetSealedBlock(block *types.Block) error {
	if s.node == nil || s.node.Block == nil {
		return fmt.Errorf("locked block is nil")
	}
	if s.node.Block.Hash() != block.Hash() {
		return fmt.Errorf("node block not equal to multi-seal block %s vs. %s", s.node.Block.Hash().String(), block.Hash().String())
	}
	s.node.Block = block
	if err := s.storeNode(s.node); err != nil {
		return err
	}
	s.lockedBlock = block
	if s.executed != nil && s.executed.Block != nil && s.executed.Block.Hash() == block.Hash() {
		s.executed.Block = block
	}

	return nil
}

func (s *roundState) UnsignedVote(code hs.MsgType) *hs.Vote {
	node := s.TreeNode()
	if node == nil || node.Hash() == hs.EmptyHash {
		return nil
	}

	// Build unsigned Vote
	vote := &hs.Vote{
		Code: code,
		View: &hs.View{
			Round:  new(big.Int).Set(s.round),
			Height: new(big.Int).Set(s.height),
		},
		TreeNode:     node.Hash(), // Instead of sending entire block, use hash
		BLSSignature: []byte{},    // Sign later
	}

	return vote
}

func (s *roundState) SetHighQC(qc *hs.QuorumCert) {
	s.highQC = qc
}

func (s *roundState) HighQC() *hs.QuorumCert {
	return s.highQC
}

func (s *roundState) SetPrepareQC(qc *hs.QuorumCert) error {
	if err := s.storePrepareQC(qc); err != nil {
		return err
	}
	s.prepareQC = qc
	return nil
}

func (s *roundState) PrepareQC() *hs.QuorumCert {
	return s.prepareQC
}

func (s *roundState) SetCommittedQC(qc *hs.QuorumCert) error {
	if err := s.storeCommitQC(qc); err != nil {
		return err
	}
	s.committedQC = qc
	return nil
}

func (s *roundState) CommittedQC() *hs.QuorumCert {
	return s.committedQC
}

// -----------------------------------------------------------------------
//
// leader collect votes
//
// -----------------------------------------------------------------------
func (s *roundState) AddNewViews(msg *hs.Message) error {
	return s.newViews.Add(msg)
}

func (s *roundState) NewViewSize() int {
	return s.newViews.Size()
}

func (s *roundState) NewViews() []*hs.Message {
	return s.newViews.Values()
}

func (s *roundState) AddPrepareVote(msg *hs.Message) error {
	return s.prepareVotes.Add(msg)
}

func (s *roundState) PrepareVotes() []*hs.Message {
	return s.prepareVotes.Values()
}

func (s *roundState) PrepareVoteSize() int {
	return s.prepareVotes.Size()
}

func (s *roundState) AddPreCommitVote(msg *hs.Message) error {
	return s.preCommitVotes.Add(msg)
}

func (s *roundState) PreCommitVotes() []*hs.Message {
	return s.preCommitVotes.Values()
}

func (s *roundState) PreCommitVoteSize() int {
	return s.preCommitVotes.Size()
}

func (s *roundState) AddCommitVote(msg *hs.Message) error {
	return s.commitVotes.Add(msg)
}

func (s *roundState) CommitVotes() []*hs.Message {
	return s.commitVotes.Values()
}

func (s *roundState) CommitVoteSize() int {
	return s.commitVotes.Size()
}

// -----------------------------------------------------------------------
//
// store round state as snapshot
//
// -----------------------------------------------------------------------

const (
	dbRoundStatePrefix = "round-state-"
	viewSuffix         = "view"
	prepareQCSuffix    = "prepareQC"
	lockQCSuffix       = "lockQC"
	commitQCSuffix     = "commitQC"
	nodeSuffix         = "node"
	blockSuffix        = "block"
)

// todo(fuk): add comments
func (s *roundState) reload(view *hs.View) {
	var (
		err      error
		printErr = s.logger != nil && s.height.Uint64() > 1
	)

	if err = s.loadView(view); err != nil && printErr {
		s.logger.Warn("Load view failed", "err", err)
	}
	if err = s.loadPrepareQC(); err != nil && printErr {
		s.logger.Warn("Load prepareQC failed", "err", err)
	}
	if err = s.loadLockQC(); err != nil && printErr {
		s.logger.Warn("Load lockQC failed", "err", err)
	}
	if err = s.loadCommitQC(); err != nil && printErr {
		s.logger.Warn("Load commitQC failed", "err", err)
	}
	if err = s.loadNode(); err != nil && printErr {
		s.logger.Warn("Load node failed", "err", err)
	}

	// reset locked node
	if s.lockQC != nil && s.node != nil && s.node.Block != nil && s.lockQC.TreeNode == s.node.Hash() {
		s.lockedBlock = s.node.Block
		s.proposalLocked = true
	}
}

func (s *roundState) storeView(view *hs.View) error {
	if s.db == nil {
		return nil
	}

	raw, err := hs.Encode(view)
	if err != nil {
		return err
	}
	return s.db.Put(viewKey(), raw)
}

func (s *roundState) loadView(cur *hs.View) error {
	if s.db == nil {
		return nil
	}

	view := new(hs.View)
	raw, err := s.db.Get(viewKey())
	if err != nil {
		return err
	}
	if err = rlp.DecodeBytes(raw, view); err != nil {
		return err
	}
	if view.Cmp(cur) > 0 {
		s.height = view.Height
		s.round = view.Round
	}
	return nil
}

func (s *roundState) storePrepareQC(qc *hs.QuorumCert) error {
	if s.db == nil {
		return nil
	}

	raw, err := hs.Encode(qc)
	if err != nil {
		return err
	}
	return s.db.Put(prepareQCKey(), raw)
}

func (s *roundState) loadPrepareQC() error {
	if s.db == nil {
		return nil
	}

	data := new(hs.QuorumCert)
	raw, err := s.db.Get(prepareQCKey())
	if err != nil {
		return err
	}
	if err = rlp.DecodeBytes(raw, data); err != nil {
		return err
	}
	s.prepareQC = data
	return nil
}

func (s *roundState) storeLockQC(qc *hs.QuorumCert) error {
	if s.db == nil {
		return nil
	}

	raw, err := hs.Encode(qc)
	if err != nil {
		return err
	}
	return s.db.Put(lockQCKey(), raw)
}

func (s *roundState) loadLockQC() error {
	if s.db == nil {
		return nil
	}

	data := new(hs.QuorumCert)
	raw, err := s.db.Get(lockQCKey())
	if err != nil {
		return err
	}
	if err = rlp.DecodeBytes(raw, data); err != nil {
		return err
	}
	s.lockQC = data
	return nil
}

func (s *roundState) storeCommitQC(qc *hs.QuorumCert) error {
	if s.db == nil {
		return nil
	}

	raw, err := hs.Encode(qc)
	if err != nil {
		return err
	}
	return s.db.Put(commitQCKey(), raw)
}

func (s *roundState) loadCommitQC() error {
	if s.db == nil {
		return nil
	}

	data := new(hs.QuorumCert)
	raw, err := s.db.Get(commitQCKey())
	if err != nil {
		return err
	}
	if err = rlp.DecodeBytes(raw, data); err != nil {
		return err
	}
	s.committedQC = data
	return nil
}

func (s *roundState) storeNode(node *hs.TreeNode) error {
	if s.db == nil {
		return nil
	}

	raw, err := hs.Encode(node)
	if err != nil {
		return err
	}
	return s.db.Put(nodeKey(), raw)
}

func (s *roundState) loadNode() error {
	if s.db == nil {
		return nil
	}

	data := new(hs.TreeNode)
	raw, err := s.db.Get(nodeKey())
	if err != nil {
		return err
	}
	if err = rlp.DecodeBytes(raw, data); err != nil {
		return err
	}
	s.node = data
	return nil
}

func viewKey() []byte {
	return append([]byte(dbRoundStatePrefix), []byte(viewSuffix)...)
}

func prepareQCKey() []byte {
	return append([]byte(dbRoundStatePrefix), []byte(prepareQCSuffix)...)
}

func lockQCKey() []byte {
	return append([]byte(dbRoundStatePrefix), []byte(lockQCSuffix)...)
}

func commitQCKey() []byte {
	return append([]byte(dbRoundStatePrefix), []byte(commitQCSuffix)...)
}

func nodeKey() []byte {
	return append([]byte(dbRoundStatePrefix), []byte(nodeSuffix)...)
}

func blockKey() []byte {
	return append([]byte(dbRoundStatePrefix), []byte(blockSuffix)...)
}
