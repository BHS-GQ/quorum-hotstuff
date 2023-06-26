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

func TestSimple(t *testing.T) {
	sys := makeSystem(7)
	sys.Start()
	sys.Close(10)
}

func TestMockNewViewCase1(t *testing.T) {
	H, R, fR, fN := uint64(4), uint64(0), uint64(1), int(1)
	fakeNodes := make(map[common.Address]struct{})
	mu := new(sync.Mutex)

	sys := makeSystem(7)
	sys.Start()
	time.Sleep(2 * time.Second)

	hasViewChange := false
	hook := func(node *Geth, data []byte) ([]byte, bool) {
		h, r := node.api.CurrentSequence()
		if h == H && r == R {
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
			if ori.Code != hs.MsgTypeNewView {
				return data, true
			}
			msg := ori.Copy()
			msg.View.Round = new(big.Int).SetUint64(fR)
			payload, err := node.resignMsg(msg)
			if err != nil {
				log.Error("failed to resign message")
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
		if h == H && r == R+1 {
			hasViewChange = true
		}
		return data, true
	}

	for _, node := range sys.nodes {
		node.setHook(hook)
	}
	sys.Close(10)

	if hasViewChange {
		t.Fail()
	}
}

// Err thrown should be "Failed to verify prepareQC"
func TestMockNewViewCase2(t *testing.T) {
	H, R, fN := uint64(5), uint64(0), 1
	fakeNodes := make(map[common.Address]struct{})
	mu := new(sync.Mutex)

	sys := makeSystem(4)
	sys.Start()
	time.Sleep(2 * time.Second)

	hasViewChange := false
	hook := func(node *Geth, data []byte) ([]byte, bool) {
		h, r := node.api.CurrentSequence()
		if h == H && r == R {
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
			if ori.Code != hs.MsgTypeNewView {
				return data, true
			}
			msg := ori.Copy()
			var qc hs.QuorumCert
			if err := rlp.DecodeBytes(msg.Msg, &qc); err != nil {
				log.Error("failed to decode prepareQC", "err", err)
				return data, true
			}
			qc.ProposedBlock = common.HexToHash("0x123")
			raw, err := rlp.EncodeToBytes(&qc)
			if err != nil {
				log.Error("encode prepareQC failed", "err", err)
				return data, true
			}
			msg.Msg = raw
			payload, err := node.resignMsg(msg)
			if err != nil {
				log.Error("failed to resign message")
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
		if h == H && r == R+1 {
			hasViewChange = true
		}
		return data, true
	}

	for _, node := range sys.nodes {
		node.setHook(hook)
	}
	sys.Close(10)

	if hasViewChange {
		t.Fail()
	}
}

func TestMockNewViewCase3(t *testing.T) {
	H, R, fN := uint64(5), uint64(0), 1
	fakeNodes := make(map[common.Address]struct{})
	mu := new(sync.Mutex)

	sys := makeSystem(4)
	sys.Start()
	time.Sleep(2 * time.Second)

	hasViewChange := false
	hook := func(node *Geth, data []byte) ([]byte, bool) {
		h, r := node.api.CurrentSequence()
		if h == H && r == R {
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
			if ori.Code != hs.MsgTypeNewView {
				return data, true
			}

			// send to other repo
			for _, peer := range node.broadcaster.peers {
				if !peer.geth.IsProposer() && peer.geth.addr != node.addr {
					peer.Send(hotstuffMsg, data)
				}
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
			log.Info("-----fake message", "address", node.addr, "msg", ori.Code, "view", view)
			return data, false
		}
		if h == H && r == R+1 {
			hasViewChange = true
		}
		return data, true
	}

	for _, node := range sys.nodes {
		node.setHook(hook)
	}
	sys.Close(10)

	if hasViewChange {
		t.Fail()
	}
}

// Err thrown should be "failed to verify prepareQC"
func TestMockNewViewCase4(t *testing.T) {
	H, R, fH, fN := uint64(5), uint64(0), uint64(5), 1
	fakeNodes := make(map[common.Address]struct{})
	mu := new(sync.Mutex)

	sys := makeSystem(4)
	sys.Start()
	time.Sleep(2 * time.Second)

	hasViewChange := false
	hook := func(node *Geth, data []byte) ([]byte, bool) {
		h, r := node.api.CurrentSequence()
		if h == H && r == R {
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
			if ori.Code != hs.MsgTypeNewView {
				return data, true
			}
			msg := ori.Copy()
			var qc hs.QuorumCert
			if err := rlp.DecodeBytes(msg.Msg, &qc); err != nil {
				log.Error("failed to decode prepareQC", "err", err)
				return data, true
			}
			qc.View.Height = new(big.Int).SetUint64(fH)
			raw, err := rlp.EncodeToBytes(&qc)
			if err != nil {
				log.Error("encode prepareQC failed", "err", err)
				return data, true
			}
			msg.Msg = raw
			payload, err := node.resignMsg(msg)
			if err != nil {
				log.Error("failed to resign message")
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
		if h == H && r == R+1 {
			hasViewChange = true
		}
		return data, true
	}

	for _, node := range sys.nodes {
		node.setHook(hook)
	}
	sys.Close(10)

	if hasViewChange {
		t.Fail()
	}
}

// Err thrown should be "failed to verify prepareQC"
func TestMockNewViewCase5(t *testing.T) {
	H, R, fR, fN := uint64(5), uint64(0), uint64(1), 1
	fakeNodes := make(map[common.Address]struct{})
	mu := new(sync.Mutex)
	sys := makeSystem(4)
	sys.Start()
	time.Sleep(2 * time.Second)

	hasViewChange := false
	hook := func(node *Geth, data []byte) ([]byte, bool) {
		h, r := node.api.CurrentSequence()
		if h == H && r == R {
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
			if ori.Code != hs.MsgTypeNewView {
				return data, true
			}
			msg := ori.Copy()
			var qc hs.QuorumCert
			if err := rlp.DecodeBytes(msg.Msg, &qc); err != nil {
				log.Error("failed to decode prepareQC", "err", err)
				return data, true
			}
			qc.View.Round = new(big.Int).SetUint64(fR)
			raw, err := rlp.EncodeToBytes(&qc)
			if err != nil {
				log.Error("encode prepareQC failed", "err", err)
				return data, true
			}
			msg.Msg = raw
			payload, err := node.resignMsg(msg)
			if err != nil {
				log.Error("failed to resign message")
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
		if h == H && r == R+1 {
			hasViewChange = true
		}
		return data, true
	}

	for _, node := range sys.nodes {
		node.setHook(hook)
	}
	sys.Close(10)

	if hasViewChange {
		t.Fail()
	}
}

// Err thrown should be "failed to verify prepareQC"
func TestMockNewViewCase6(t *testing.T) {
	H, R, fN := uint64(5), uint64(0), 1
	fakeNodes := make(map[common.Address]struct{})
	mu := new(sync.Mutex)

	sys := makeSystem(4)
	sys.Start()
	time.Sleep(2 * time.Second)

	hasViewChange := false
	hook := func(node *Geth, data []byte) ([]byte, bool) {
		h, r := node.api.CurrentSequence()
		if h == H && r == R {
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
			if ori.Code != hs.MsgTypeNewView {
				return data, true
			}
			msg := ori.Copy()
			var qc hs.QuorumCert
			if err := rlp.DecodeBytes(msg.Msg, &qc); err != nil {
				log.Error("failed to decode prepareQC", "err", err)
				return data, true
			}
			qc.BLSSignature[0] += 1
			raw, err := rlp.EncodeToBytes(&qc)
			if err != nil {
				log.Error("encode prepareQC failed", "err", err)
				return data, true
			}
			msg.Msg = raw
			payload, err := node.resignMsg(msg)
			if err != nil {
				log.Error("failed to resign message")
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
		if h == H && r == R+1 {
			hasViewChange = true
		}
		return data, true
	}

	for _, node := range sys.nodes {
		node.setHook(hook)
	}
	sys.Close(10)

	if hasViewChange {
		t.Fail()
	}
}
