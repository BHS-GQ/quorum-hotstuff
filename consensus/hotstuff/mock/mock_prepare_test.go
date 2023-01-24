package mock

import (
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
)

// go test -v -count=1 github.com/ethereum/go-ethereum/consensus/hotstuff/mock -run TestMockPrepareCase1
// net scale is 4, leader send fake message of prepare with wrong height, repos change view.
func TestMockPrepareCase1(t *testing.T) {
	H, R, fH := uint64(4), uint64(0), uint64(5)

	sys := makeSystem(4)
	sys.Start()
	time.Sleep(2 * time.Second)

	hook := func(node *Geth, data []byte) ([]byte, bool) {
		if h, r := node.api.CurrentSequence(); h == H && r == R {
			if !node.IsProposer() {
				return data, true
			}

			var ori hs.Message
			if err := rlp.DecodeBytes(data, &ori); err != nil {
				log.Error("failed to decode message", "err", err)
				return data, true
			}
			if ori.Code != hs.MsgTypePrepare {
				return data, true
			}
			msg := ori.Copy()
			msg.View.Height = new(big.Int).SetUint64(fH)
			payload, err := node.resignMsg(msg)
			if err != nil {
				log.Error("failed to resign message")
				return data, true
			}
			view := &hs.View{
				Round:  new(big.Int).SetUint64(r),
				Height: new(big.Int).SetUint64(h),
			}
			log.Info("-----fake message", "address", node.addr, "msg", msg.Code, "view", view, "msg", msg)
			return payload, true
		}
		return data, true
	}

	for _, node := range sys.nodes {
		node.setHook(hook)
	}
	sys.Close(10)
}

// go test -v -count=1 github.com/ethereum/go-ethereum/consensus/hotstuff/mock -run TestMockPrepareCase2
// net scale is 4, leader send fake message of prepare with wrong height, repos change view.
func TestMockPrepareCase2(t *testing.T) {
	H, R, fR := uint64(4), uint64(0), uint64(1)

	sys := makeSystem(4)
	sys.Start()
	time.Sleep(2 * time.Second)

	hook := func(node *Geth, data []byte) ([]byte, bool) {
		if h, r := node.api.CurrentSequence(); h == H && r == R {
			if !node.IsProposer() {
				return data, true
			}

			var ori hs.Message
			if err := rlp.DecodeBytes(data, &ori); err != nil {
				log.Error("failed to decode message", "err", err)
				return data, true
			}
			if ori.Code != hs.MsgTypePrepare {
				return data, true
			}
			msg := ori.Copy()
			msg.View.Round = new(big.Int).SetUint64(fR)
			payload, err := node.resignMsg(msg)
			if err != nil {
				log.Error("failed to resign message")
				return data, true
			}

			view := &hs.View{
				Round:  new(big.Int).SetUint64(r),
				Height: new(big.Int).SetUint64(h),
			}
			log.Info("-----fake message", "address", node.addr, "msg", msg.Code, "view", view, "msg", msg)
			return payload, true
		}
		return data, true
	}

	for _, node := range sys.nodes {
		node.setHook(hook)
	}
	sys.Close(10)
}

// go test -v -count=1 github.com/ethereum/go-ethereum/consensus/hotstuff/mock -run TestMockPrepareCase3
// net scale is 4, leader send fake message of prepare with wrong qc.view.height, repos change view.
func TestMockPrepareCase3(t *testing.T) {
	H, R, fH := uint64(4), uint64(0), uint64(4)

	sys := makeSystem(4)
	sys.Start()
	time.Sleep(2 * time.Second)

	hook := func(node *Geth, data []byte) ([]byte, bool) {
		if h, r := node.api.CurrentSequence(); h == H && r == R {
			if !node.IsProposer() {
				return data, true
			}

			var ori hs.Message
			if err := rlp.DecodeBytes(data, &ori); err != nil {
				log.Error("failed to decode message", "err", err)
				return data, true
			}
			if ori.Code != hs.MsgTypePrepare {
				return data, true
			}
			msg := ori.Copy()
			var sub hs.PackagedQC
			if err := rlp.DecodeBytes(msg.Msg, &sub); err != nil {
				log.Error("failed to decode subject", "err", err)
				return data, true
			}
			var qc hs.QuorumCert
			if raw, err := rlp.EncodeToBytes(sub.QC); err != nil {
				log.Error("failed to encode prepareQC", "err", err)
				return data, true
			} else if err := rlp.DecodeBytes(raw, &qc); err != nil {
				log.Error("failed to decode prepareQC", "err", err)
				return data, true
			}
			qc.View.Height = new(big.Int).SetUint64(fH)
			var newSub = struct {
				TreeNode *hs.TreeNode
				QC       *hs.QuorumCert
			}{
				sub.TreeNode,
				&qc,
			}
			if raw, err := rlp.EncodeToBytes(newSub); err != nil {
				log.Error("failed to encode new subject", "err", err)
				return data, true
			} else {
				msg.Msg = raw
			}

			payload, err := node.resignMsg(msg)
			if err != nil {
				log.Error("failed to resign message")
				return data, true
			}

			view := &hs.View{
				Round:  new(big.Int).SetUint64(r),
				Height: new(big.Int).SetUint64(h),
			}
			log.Info("-----fake message", "address", node.addr, "msg", msg.Code, "view", view, "msg", msg)
			return payload, true
		}
		return data, true
	}

	for _, node := range sys.nodes {
		node.setHook(hook)
	}
	sys.Close(10)
}

// go test -v -count=1 github.com/ethereum/go-ethereum/consensus/hotstuff/mock -run TestMockPrepareCase4
// net scale is 4, leader send fake message of prepare with wrong qc.view.round, repos change view.
func TestMockPrepareCase4(t *testing.T) {
	H, R, fR := uint64(4), uint64(0), uint64(1)

	sys := makeSystem(4)
	sys.Start()
	time.Sleep(2 * time.Second)

	hook := func(node *Geth, data []byte) ([]byte, bool) {
		if h, r := node.api.CurrentSequence(); h == H && r == R {
			if !node.IsProposer() {
				return data, true
			}

			var ori hs.Message
			if err := rlp.DecodeBytes(data, &ori); err != nil {
				log.Error("failed to decode message", "err", err)
				return data, true
			}
			if ori.Code != hs.MsgTypePrepare {
				return data, true
			}
			msg := ori.Copy()
			var sub hs.PackagedQC
			if err := rlp.DecodeBytes(msg.Msg, &sub); err != nil {
				log.Error("failed to decode subject", "err", err)
				return data, true
			}
			var qc hs.QuorumCert
			if raw, err := rlp.EncodeToBytes(sub.QC); err != nil {
				log.Error("failed to encode prepareQC", "err", err)
				return data, true
			} else if err := rlp.DecodeBytes(raw, &qc); err != nil {
				log.Error("failed to decode prepareQC", "err", err)
				return data, true
			}
			qc.View.Round = new(big.Int).SetUint64(fR)
			var newSub = struct {
				Node *hs.TreeNode
				QC   *hs.QuorumCert
			}{
				sub.TreeNode,
				&qc,
			}
			if raw, err := rlp.EncodeToBytes(newSub); err != nil {
				log.Error("failed to encode new subject", "err", err)
				return data, true
			} else {
				msg.Msg = raw
			}

			payload, err := node.resignMsg(msg)
			if err != nil {
				log.Error("failed to resign message")
				return data, true
			}

			view := &hs.View{
				Round:  new(big.Int).SetUint64(r),
				Height: new(big.Int).SetUint64(h),
			}
			log.Info("-----fake message", "address", node.addr, "msg", msg.Code, "view", view, "msg", msg)
			return payload, true
		}
		return data, true
	}

	for _, node := range sys.nodes {
		node.setHook(hook)
	}
	sys.Close(10)
}

// go test -v -count=1 github.com/ethereum/go-ethereum/consensus/hotstuff/mock -run TestMockPrepareCase5
// net scale is 4, leader send fake message of prepare with wrong qc.hash, repos change view.
func TestMockPrepareCase5(t *testing.T) {
	H, R := uint64(4), uint64(0)

	sys := makeSystem(4)
	sys.Start()
	time.Sleep(2 * time.Second)

	hook := func(node *Geth, data []byte) ([]byte, bool) {
		if h, r := node.api.CurrentSequence(); h == H && r == R {
			if !node.IsProposer() {
				return data, true
			}

			var ori hs.Message
			if err := rlp.DecodeBytes(data, &ori); err != nil {
				log.Error("failed to decode message", "err", err)
				return data, true
			}
			if ori.Code != hs.MsgTypePrepare {
				return data, true
			}
			msg := ori.Copy()
			var sub hs.PackagedQC
			if err := rlp.DecodeBytes(msg.Msg, &sub); err != nil {
				log.Error("failed to decode subject", "err", err)
				return data, true
			}
			var qc hs.QuorumCert
			if raw, err := rlp.EncodeToBytes(sub.QC); err != nil {
				log.Error("failed to encode prepareQC", "err", err)
				return data, true
			} else if err := rlp.DecodeBytes(raw, &qc); err != nil {
				log.Error("failed to decode prepareQC", "err", err)
				return data, true
			}
			qc.TreeNode = common.HexToHash("0x124")
			var newSub = struct {
				Node *hs.TreeNode
				QC   *hs.QuorumCert
			}{
				sub.TreeNode,
				&qc,
			}
			if raw, err := rlp.EncodeToBytes(newSub); err != nil {
				log.Error("failed to encode new subject", "err", err)
				return data, true
			} else {
				msg.Msg = raw
			}

			payload, err := node.resignMsg(msg)
			if err != nil {
				log.Error("failed to resign message")
				return data, true
			}

			view := &hs.View{
				Round:  new(big.Int).SetUint64(r),
				Height: new(big.Int).SetUint64(h),
			}
			log.Info("-----fake message", "address", node.addr, "msg", msg.Code, "view", view, "msg", msg)
			return payload, true
		}
		return data, true
	}

	for _, node := range sys.nodes {
		node.setHook(hook)
	}
	sys.Close(10)
}

// go test -v -count=1 github.com/ethereum/go-ethereum/consensus/hotstuff/mock -run TestMockPrepareVoteCase1
// net scale is 4, leader is sent fake message of prepareVote with wrong height.
func TestMockPrepareVoteCase1(t *testing.T) {
	H, R, fH, fN := uint64(4), uint64(0), uint64(5), 1
	fakeNodes := make(map[common.Address]struct{})
	mu := new(sync.Mutex)

	sys := makeSystem(4)
	sys.Start()
	time.Sleep(2 * time.Second)

	hook := func(node *Geth, data []byte) ([]byte, bool) {
		if h, r := node.api.CurrentSequence(); h == H && r == R {
			if node.IsProposer() {
				return data, true
			}

			mu.Lock()
			if _, ok := fakeNodes[node.addr]; ok {
				mu.Unlock()
				return data, true
			}
			mu.Unlock()

			var ori hs.Message
			if err := rlp.DecodeBytes(data, &ori); err != nil {
				log.Error("failed to decode message", "err", err)
				return data, true
			}
			if ori.Code != hs.MsgTypePrepareVote {
				return data, true
			}
			msg := ori.Copy()
			msg.View.Height = new(big.Int).SetUint64(fH)

			payload, err := node.resignMsg(msg)
			if err != nil {
				log.Error("failed to resign message", "err", err)
				return data, true
			}

			mu.Lock()
			fakeNodes[node.addr] = struct{}{}
			if len(fakeNodes) > fN {
				mu.Unlock()
				return data, true
			}
			mu.Unlock()

			view := &hs.View{
				Round:  new(big.Int).SetUint64(r),
				Height: new(big.Int).SetUint64(h),
			}
			log.Info("-----fake message", "address", node.addr, "msg", msg.Code, "view", view, "msg", msg)
			return payload, true
		}
		return data, true
	}

	for _, node := range sys.nodes {
		node.setHook(hook)
	}
	sys.Close(10)
}
