package hotstuff

import (
	"bytes"
	"fmt"
	"io"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"golang.org/x/crypto/sha3"
)

var (
	EmptyHash    = common.Hash{}
	EmptyAddress = common.Address{}
)

type MsgType uint64

const (
	MsgTypeUnknown       MsgType = 0
	MsgTypeNewView       MsgType = 1
	MsgTypePrepare       MsgType = 2
	MsgTypePrepareVote   MsgType = 3
	MsgTypePreCommit     MsgType = 4
	MsgTypePreCommitVote MsgType = 5
	MsgTypeCommit        MsgType = 6
	MsgTypeCommitVote    MsgType = 7
	MsgTypeDecide        MsgType = 8
)

func (m MsgType) String() string {
	switch m {
	case MsgTypeNewView:
		return "NewView"
	case MsgTypePrepare:
		return "Prepare"
	case MsgTypePrepareVote:
		return "PrepareVote"
	case MsgTypePreCommit:
		return "PreCommit"
	case MsgTypePreCommitVote:
		return "PreCommitVote"
	case MsgTypeCommit:
		return "Commit"
	case MsgTypeCommitVote:
		return "CommitVote"
	case MsgTypeDecide:
		return "Decide"
	default:
		return "Unknown"
	}
}

func (m MsgType) Value() uint64 {
	return uint64(m)
}

type State uint64

const (
	StateAcceptRequest State = 1
	StateHighQC        State = 2
	StatePrepared      State = 3
	StatePreCommitted  State = 4
	StateCommitted     State = 5
)

func (s State) String() string {
	if s == StateAcceptRequest {
		return "StateAcceptRequest"
	} else if s == StateHighQC {
		return "StateHighQC"
	} else if s == StatePrepared {
		return "StatePrepared"
	} else if s == StatePreCommitted {
		return "StatePreCommitted"
	} else if s == StateCommitted {
		return "Committed"
	} else {
		return "Unknown"
	}
}

// Cmp compares s and y and returns:
//   -1 if s is the previous state of y
//    0 if s and y are the same state
//   +1 if s is the next state of y
func (s State) Cmp(y State) int {
	if uint64(s) < uint64(y) {
		return -1
	}
	if uint64(s) > uint64(y) {
		return 1
	}
	return 0
}

// Proposal supports retrieving height and serialized block to be used during HotStuff consensus.
// It is the interface that abstracts different message structure. (consensus/hotstuff/core/core.go)
type Proposal interface {
	// Number retrieves the block height number of this proposal.
	Number() *big.Int

	// Hash retrieves the hash of this proposal.
	Hash() common.Hash

	ParentHash() common.Hash

	Coinbase() common.Address

	Time() uint64

	EncodeRLP(w io.Writer) error

	DecodeRLP(s *rlp.Stream) error
}

// View includes a round number and a block height number.
// Height is the block height number we'd like to commit.
//
// If the given block is not accepted by validators, a round change will occur
// and the validators start a new round with round+1.
type View struct {
	Round  *big.Int
	Height *big.Int
}

func (v *View) HeightU64() uint64 {
	if v.Height == nil {
		return 0
	}
	return v.Height.Uint64()
}

func (v *View) RoundU64() uint64 {
	if v.Round == nil {
		return 0
	}
	return v.Round.Uint64()
}

// EncodeRLP serializes b into the Ethereum RLP format.
func (v *View) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{v.Round, v.Height})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (v *View) DecodeRLP(s *rlp.Stream) error {
	var data struct {
		Round  *big.Int
		Height *big.Int
	}

	if err := s.Decode(&data); err != nil {
		return err
	}
	v.Round, v.Height = data.Round, data.Height
	return nil
}

func (v *View) String() string {
	return fmt.Sprintf("{Round: %d, Height: %d}", v.Round.Uint64(), v.Height.Uint64())
}

// Cmp compares v and y and returns:
//   -1 if v <  y
//    0 if v == y
//   +1 if v >  y
func (v *View) Cmp(y *View) int {
	if v.Height.Cmp(y.Height) != 0 {
		return v.Height.Cmp(y.Height)
	}
	if v.Round.Cmp(y.Round) != 0 {
		return v.Round.Cmp(y.Round)
	}
	return 0
}

func (v *View) Sub(y *View) (int64, int64) {
	h := new(big.Int).Sub(v.Height, y.Height).Int64()
	r := new(big.Int).Sub(v.Round, y.Round).Int64()
	return h, r
}

type TreeNode struct {
	hash common.Hash

	Parent common.Hash  // Parent TreeTreeNode hash
	Block  *types.Block // Command to agree on
}

func NewTreeNode(parent common.Hash, block *types.Block) *TreeNode {
	TreeNode := &TreeNode{
		Parent: parent,
		Block:  block,
	}
	TreeNode.Hash()
	return TreeNode
}

func (n *TreeNode) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{n.Parent, n.Block})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (n *TreeNode) DecodeRLP(s *rlp.Stream) error {
	var data struct {
		Parent common.Hash
		Block  *types.Block
	}

	if err := s.Decode(&data); err != nil {
		return err
	}

	n.Parent, n.Block = data.Parent, data.Block
	return nil
}

func (n *TreeNode) Hash() common.Hash {
	if n.hash == EmptyHash {
		n.hash = RLPHash([]common.Hash{n.Parent, n.Block.Hash()})
	}
	return n.hash
}

func (n *TreeNode) String() string {
	return fmt.Sprintf("{TreeNode: %v, parent: %v, block: %v}", n.Hash(), n.Parent, n.Block.Hash())
}

type QuorumCert struct {
	View         *View
	Code         MsgType
	TreeNode     common.Hash // block header sig hash
	Proposer     common.Address
	BLSSignature []byte
}

// EncodeRLP serializes b into the Ethereum RLP format.
func (qc *QuorumCert) EncodeRLP(w io.Writer) error {
	code := qc.Code.Value()
	return rlp.Encode(w, []interface{}{qc.View, code, qc.TreeNode, qc.Proposer, qc.BLSSignature})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (qc *QuorumCert) DecodeRLP(s *rlp.Stream) error {
	var data struct {
		View         *View
		Code         MsgType
		TreeNode     common.Hash
		Proposer     common.Address
		BLSSignature []byte
	}

	if err := s.Decode(&data); err != nil {
		return err
	}

	qc.View, qc.Code, qc.TreeNode, qc.Proposer, qc.BLSSignature = data.View, data.Code, data.TreeNode, data.Proposer, data.BLSSignature
	return nil
}

func (qc *QuorumCert) Height() *big.Int {
	if qc.View == nil {
		return common.Big0
	}
	return qc.View.Height
}

func (qc *QuorumCert) HeightU64() uint64 {
	return qc.Height().Uint64()
}

func (qc *QuorumCert) Round() *big.Int {
	if qc.View == nil {
		return common.Big0
	}
	return qc.View.Round
}

func (qc *QuorumCert) RoundU64() uint64 {
	return qc.Round().Uint64()
}

func (qc *QuorumCert) String() string {
	return fmt.Sprintf("{QuorumCert Code: %v, View: %v, Hash: %v, Proposer: %v}", qc.Code.String(), qc.View, qc.TreeNode.String(), qc.Proposer.Hex())
}

func (qc *QuorumCert) Copy() *QuorumCert {
	enc, err := rlp.EncodeToBytes(qc)
	if err != nil {
		return nil
	}
	newQC := new(QuorumCert)
	if err := rlp.DecodeBytes(enc, &newQC); err != nil {
		return nil
	}
	return newQC
}

// Wrapper for various payloads
type Message struct {
	hash common.Hash // Hash of Code, View, Msg

	Address common.Address

	Code MsgType
	View *View
	Msg  []byte

	Signature []byte // Used for ECDSA signature
}

// EncodeRLP serializes m into the Ethereum RLP format.
func (m *Message) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{m.Code.Value(), m.View, m.Msg, m.Address, m.Signature})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (m *Message) DecodeRLP(s *rlp.Stream) error {
	var data struct {
		Code      uint64
		View      *View
		Msg       []byte
		Address   common.Address
		Signature []byte
	}

	if err := s.Decode(&data); err != nil {
		return err
	}

	m.Code, m.View, m.Msg, m.Address, m.Signature = MsgType(data.Code), data.View, data.Msg, data.Address, data.Signature
	return nil
}

func (m *Message) FromPayload(payload []byte, validateFn func(common.Hash, []byte) (common.Address, error)) error {
	// Decode Message
	var err error

	if err = rlp.DecodeBytes(payload, &m); err != nil {
		return err
	}

	// Check for nil fields
	if m.View == nil || m.Address == EmptyAddress || m.Msg == nil {
		return errInvalidMessage
	}

	// Validate unsigned Message
	if _, err := m.PayloadNoSig(); err != nil {
		return err
	}
	if validateFn != nil {
		signer, err := validateFn(m.hash, m.Signature)
		if err != nil {
			return err
		}
		if !bytes.Equal(signer.Bytes(), m.Address.Bytes()) {
			return errInvalidSigner
		}
	}
	return nil
}

func (m *Message) Payload() ([]byte, error) {
	return rlp.EncodeToBytes(m)
}

func (m *Message) PayloadNoSig() ([]byte, error) {
	data, err := rlp.EncodeToBytes(&Message{
		Address:   common.Address{},
		Code:      m.Code,
		View:      m.View,
		Msg:       m.Msg,
		Signature: []byte{},
	})
	if err != nil {
		return nil, err
	}

	m.hash = crypto.Keccak256Hash(data)
	return data, nil
}

func (m *Message) Decode(val interface{}) error {
	return rlp.DecodeBytes(m.Msg, val)
}

func (m *Message) Hash() (common.Hash, error) {
	if m.hash != EmptyHash {
		return m.hash, nil
	}
	if _, err := m.PayloadNoSig(); err != nil {
		return EmptyHash, err
	}
	return m.hash, nil
}

func (m *Message) Copy() *Message {
	view := &View{
		Height: new(big.Int).SetUint64(m.View.HeightU64()),
		Round:  new(big.Int).SetUint64(m.View.RoundU64()),
	}
	msg := make([]byte, len(m.Msg))
	copy(msg[:], m.Msg[:])

	return &Message{
		Code: m.Code,
		View: view,
		Msg:  msg,
	}
}

func (m *Message) String() string {
	return fmt.Sprintf("{MsgType: %s, Address: %s, View: %v}", m.Code.String(), m.Address.Hex(), m.View)
}

type Vote struct {
	Code   MsgType
	View   *View
	Digest common.Hash // Hash of Proposal/Block

	BLSSignature []byte
}

// EncodeRLP serializes b into the Ethereum RLP format.
func (b *Vote) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{b.Code, b.View, b.Digest, b.BLSSignature})
}

// DecodeRLP implements rlp.Decoder, and load the consensus fields from a RLP stream.
func (b *Vote) DecodeRLP(s *rlp.Stream) error {
	var data struct {
		Code   MsgType
		View   *View
		Digest common.Hash

		BLSSignature []byte
	}

	if err := s.Decode(&data); err != nil {
		return err
	}
	b.Code, b.View, b.Digest, b.BLSSignature = data.Code, data.View, data.Digest, data.BLSSignature
	return nil
}

func (b *Vote) String() string {
	return fmt.Sprintf("{Code: %v, View: %v, Digest: %v}", b.Code.String(), b.View, b.Digest.String())
}

type PackagedQC struct {
	Proposal Proposal
	QC       *QuorumCert // QuorumCert only contains Proposal's hash
}

func (m *PackagedQC) EncodeRLP(w io.Writer) error {
	block, ok := m.Proposal.(*types.Block)
	if !ok {
		return errInvalidProposal
	}
	return rlp.Encode(w, []interface{}{block, m.QC})
}

func (m *PackagedQC) DecodeRLP(s *rlp.Stream) error {
	var data struct {
		Proposal *types.Block
		QC       *QuorumCert
	}

	if err := s.Decode(&data); err != nil {
		return err
	}
	m.Proposal, m.QC = data.Proposal, data.QC
	return nil
}

type timeoutEvent struct{}
type backlogEvent struct {
	src Validator
	msg *Message
}

func RLPHash(v interface{}) (h common.Hash) {
	hw := sha3.NewLegacyKeccak256()
	rlp.Encode(hw, v)
	hw.Sum(h[:0])
	return h
}
