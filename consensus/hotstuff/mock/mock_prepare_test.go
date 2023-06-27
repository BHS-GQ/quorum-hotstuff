package mock

import (
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
)

func TestPrepareFaultyHeightBad(t *testing.T) {
	H, R, fH := uint64(4), uint64(0), uint64(5)

	sys := makeSystem(4)
	sys.Start()
	time.Sleep(2 * time.Second)

	hasViewChange := false
	hook := func(node *Geth, data []byte) ([]byte, bool) {
		h, r := node.api.CurrentSequence()
		if h == H && r == R {
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
		if h == H && r == R+1 {
			hasViewChange = true
		}
		return data, true
	}

	for _, node := range sys.nodes {
		node.setHook(hook)
	}
	sys.Close(10)

	if !hasViewChange {
		t.Fail()
	}
}

func TestPrepareFaultyRoundBad(t *testing.T) {
	H, R, fR := uint64(4), uint64(0), uint64(1)

	sys := makeSystem(4)
	sys.Start()
	time.Sleep(2 * time.Second)

	hasViewChange := false
	hook := func(node *Geth, data []byte) ([]byte, bool) {
		h, r := node.api.CurrentSequence()
		if h == H && r == R {
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
		if h == H && r == R+1 {
			hasViewChange = true
		}
		return data, true
	}

	for _, node := range sys.nodes {
		node.setHook(hook)
	}
	sys.Close(10)

	if !hasViewChange {
		t.Fail()
	}
}

func TestPrepareFaultyQCHeightBad(t *testing.T) {
	H, R, fH := uint64(4), uint64(0), uint64(4)

	sys := makeSystem(4)
	sys.Start()
	time.Sleep(2 * time.Second)

	hasViewChange := false
	hook := func(node *Geth, data []byte) ([]byte, bool) {
		h, r := node.api.CurrentSequence()
		if h == H && r == R {
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
				log.Error("failed to decode PackagedQC", "err", err)
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
				ProposedBlock *hs.ProposedBlock
				QC            *hs.QuorumCert
			}{
				sub.ProposedBlock,
				&qc,
			}
			if raw, err := rlp.EncodeToBytes(newSub); err != nil {
				log.Error("failed to encode new PackagedQC", "err", err)
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
		if h == H && r == R+1 {
			hasViewChange = true
		}
		return data, true
	}

	for _, node := range sys.nodes {
		node.setHook(hook)
	}
	sys.Close(10)

	if !hasViewChange {
		t.Fail()
	}
}

func TestPrepareFaultyQCRoundBad(t *testing.T) {
	H, R, fR := uint64(4), uint64(0), uint64(1)

	sys := makeSystem(4)
	sys.Start()
	time.Sleep(2 * time.Second)

	hasViewChange := false
	hook := func(node *Geth, data []byte) ([]byte, bool) {
		h, r := node.api.CurrentSequence()
		if h == H && r == R {
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
				log.Error("failed to decode PackagedQC", "err", err)
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
				Node *hs.ProposedBlock
				QC   *hs.QuorumCert
			}{
				sub.ProposedBlock,
				&qc,
			}
			if raw, err := rlp.EncodeToBytes(newSub); err != nil {
				log.Error("failed to encode new PackagedQC", "err", err)
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
		if h == H && r == R+1 {
			hasViewChange = true
		}
		return data, true
	}

	for _, node := range sys.nodes {
		node.setHook(hook)
	}
	sys.Close(10)

	if !hasViewChange {
		t.Fail()
	}
}

func TestPrepareFaultyQCBlockBad(t *testing.T) {
	H, R := uint64(4), uint64(0)

	sys := makeSystem(4)
	sys.Start()
	time.Sleep(2 * time.Second)

	hasViewChange := false
	hook := func(node *Geth, data []byte) ([]byte, bool) {
		h, r := node.api.CurrentSequence()
		if h == H && r == R {
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
				log.Error("failed to decode PackagedQC", "err", err)
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
			qc.ProposedBlock = common.HexToHash("0x124")
			var newSub = struct {
				Node *hs.ProposedBlock
				QC   *hs.QuorumCert
			}{
				sub.ProposedBlock,
				&qc,
			}
			if raw, err := rlp.EncodeToBytes(newSub); err != nil {
				log.Error("failed to encode new PackagedQC", "err", err)
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
		if h == H && r == R+1 {
			hasViewChange = true
		}
		return data, true
	}

	for _, node := range sys.nodes {
		node.setHook(hook)
	}
	sys.Close(10)

	if !hasViewChange {
		t.Fail()
	}
}

func TestPrepareFaultyQCSigBad(t *testing.T) {
	H, R, fN := uint64(4), uint64(0), int32(1)

	var locked int32
	atomic.StoreInt32(&locked, 0)

	sys := makeSystem(4)
	sys.Start()
	time.Sleep(2 * time.Second)

	hasViewChange := false
	hook := func(node *Geth, data []byte) ([]byte, bool) {
		h, r := node.api.CurrentSequence()
		if h == H && r == R {
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
				log.Error("failed to decode PackagedQC", "err", err)
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

			qc.BLSSignature[0] += 1

			var newSub = struct {
				Node *hs.ProposedBlock
				QC   *hs.QuorumCert
			}{
				sub.ProposedBlock,
				&qc,
			}
			if raw, err := rlp.EncodeToBytes(newSub); err != nil {
				log.Error("failed to encode new PackagedQC", "err", err)
				return data, true
			} else {
				msg.Msg = raw
			}

			payload, err := node.resignMsg(msg)
			if err != nil {
				log.Error("failed to resign message")
				return data, true
			}

			if value := atomic.LoadInt32(&locked); value >= fN {
				return data, true
			} else {
				atomic.StoreInt32(&locked, value+1)
				view := &hs.View{
					Round:  new(big.Int).SetUint64(r),
					Height: new(big.Int).SetUint64(h),
				}
				log.Info("-----fake message", "address", node.addr, "msg", msg.Code, "view", view, "msg", msg)
				return payload, true
			}
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

	if !hasViewChange {
		t.Fail()
	}
}

func TestPrepareVoteFaultyHeightOk(t *testing.T) {
	H, R, fH, fN := uint64(4), uint64(0), uint64(5), 1
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
			if ori.Code != hs.MsgTypePrepareVote {
				return data, true
			}
			msg := ori.Copy()
			var vote hs.Vote
			if err := rlp.DecodeBytes(msg.Msg, &vote); err != nil {
				log.Error("failed to decode prepareQC", "err", err)
				return data, true
			}

			vote.View.Height = new(big.Int).SetUint64(fH)
			if raw, err := rlp.EncodeToBytes(&vote); err != nil {
				log.Error("failed to encode prepareQC", "err", err)
				return data, true
			} else {
				msg.Msg = raw
			}

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

func TestPrepareVoteFaultyHeightBad(t *testing.T) {
	H, R, fH, fN := uint64(4), uint64(0), uint64(5), 2
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
			if ori.Code != hs.MsgTypePrepareVote {
				return data, true
			}
			msg := ori.Copy()

			var vote hs.Vote
			if err := rlp.DecodeBytes(msg.Msg, &vote); err != nil {
				log.Error("failed to decode prepareQC", "err", err)
				return data, true
			}

			vote.View.Height = new(big.Int).SetUint64(fH)
			if raw, err := rlp.EncodeToBytes(&vote); err != nil {
				log.Error("failed to encode prepareQC", "err", err)
				return data, true
			} else {
				msg.Msg = raw
			}

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
		if h == H && r == R+1 {
			hasViewChange = true
		}
		return data, true
	}

	for _, node := range sys.nodes {
		node.setHook(hook)
	}
	sys.Close(10)

	if !hasViewChange {
		t.Fail()
	}
}

func TestPrepareVoteFaultyRoundOk(t *testing.T) {
	H, R, fR, fN := uint64(4), uint64(0), uint64(1), 1
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
			if ori.Code != hs.MsgTypePrepareVote {
				return data, true
			}
			msg := ori.Copy()
			var vote hs.Vote
			if err := rlp.DecodeBytes(msg.Msg, &vote); err != nil {
				log.Error("failed to decode prepareQC", "err", err)
				return data, true
			}

			vote.View.Round = new(big.Int).SetUint64(fR)
			if raw, err := rlp.EncodeToBytes(&vote); err != nil {
				log.Error("failed to encode prepareQC", "err", err)
				return data, true
			} else {
				msg.Msg = raw
			}

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

func TestPrepareVoteFaultyRoundBad(t *testing.T) {
	H, R, fR, fN := uint64(4), uint64(0), uint64(1), 2
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
			if ori.Code != hs.MsgTypePrepareVote {
				return data, true
			}
			msg := ori.Copy()
			msg.View.Round = new(big.Int).SetUint64(fR)

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
		if h == H && r == R+1 {
			hasViewChange = true
		}
		return data, true
	}

	for _, node := range sys.nodes {
		node.setHook(hook)
	}
	sys.Close(10)

	if !hasViewChange {
		t.Fail()
	}
}

func TestPrepareVoteFaultyPayloadOk(t *testing.T) {
	H, R, fN := uint64(4), uint64(0), 1
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
			if ori.Code != hs.MsgTypePrepareVote {
				return data, true
			}
			msg := ori.Copy()
			msg.Msg = common.HexToHash("0x12345").Bytes()

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

func TestPrepareVoteFaultyPayloadBad(t *testing.T) {
	H, R, fN := uint64(4), uint64(0), 2
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
			if ori.Code != hs.MsgTypePrepareVote {
				return data, true
			}
			msg := ori.Copy()
			msg.Msg = common.HexToHash("0x12345").Bytes()

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
		if h == H && r == R+1 {
			hasViewChange = true
		}
		return data, true
	}

	for _, node := range sys.nodes {
		node.setHook(hook)
	}
	sys.Close(10)

	if !hasViewChange {
		t.Fail()
	}
}
