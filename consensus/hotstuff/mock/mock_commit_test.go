package mock

import (
	"math/big"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
)

func TestCommitFaultyHeightBad(t *testing.T) {
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
			if ori.Code != hs.MsgTypeCommit {
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

func TestCommitFaultyRoundBad(t *testing.T) {
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
			if ori.Code != hs.MsgTypeCommit {
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

func TestCommitFaultyQCHeightBad(t *testing.T) {
	H, R, fH := uint64(4), uint64(0), uint64(3)

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
			if ori.Code != hs.MsgTypeCommit {
				return data, true
			}
			msg := ori.Copy()

			var qc hs.QuorumCert
			if err := msg.Decode(&qc); err != nil {
				log.Error("failed to decode prepareQC", "err", err)
				return data, true
			}
			qc.View.Height = new(big.Int).SetUint64(fH)
			if raw, err := rlp.EncodeToBytes(&qc); err != nil {
				log.Error("failed to encode prepareQC", "err", err)
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

func TestCommitFaultyQCRoundBad(t *testing.T) {
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
			if ori.Code != hs.MsgTypeCommit {
				return data, true
			}
			msg := ori.Copy()

			var qc hs.QuorumCert
			if err := rlp.DecodeBytes(msg.Msg, &qc); err != nil {
				log.Error("failed to decode prepareQC", "err", err)
				return data, true
			}
			qc.View.Round = new(big.Int).SetUint64(fR)
			if raw, err := rlp.EncodeToBytes(&qc); err != nil {
				log.Error("failed to encode prepareQC", "err", err)
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

func TestCommitFaultyQCBlockBad(t *testing.T) {
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
			if ori.Code != hs.MsgTypeCommit {
				return data, true
			}
			msg := ori.Copy()

			var qc hs.QuorumCert
			if err := rlp.DecodeBytes(msg.Msg, &qc); err != nil {
				log.Error("failed to decode prepareQC", "err", err)
				return data, true
			}
			qc.ProposedBlock = common.HexToHash("0x12345")
			if raw, err := rlp.EncodeToBytes(&qc); err != nil {
				log.Error("failed to encode prepareQC", "err", err)
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

func TestCommitFaultyQCSigBad(t *testing.T) {
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
			if ori.Code != hs.MsgTypeCommit {
				return data, true
			}
			msg := ori.Copy()

			var qc hs.QuorumCert
			if err := rlp.DecodeBytes(msg.Msg, &qc); err != nil {
				log.Error("failed to decode prepareQC", "err", err)
				return data, true
			}

			// Replace BLSSignature with faulty bytes
			qc.BLSSignature[0] += 1

			if raw, err := rlp.EncodeToBytes(&qc); err != nil {
				log.Error("failed to encode prepareQC", "err", err)
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

func TestCommitVoteFaultyHeightOk(t *testing.T) {
	H, R, fH, fN := uint64(4), uint64(0), uint64(5), int32(1)

	var locked int32
	atomic.StoreInt32(&locked, 0)

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

			var ori hs.Message
			if err := rlp.DecodeBytes(data, &ori); err != nil {
				log.Error("failed to decode message", "err", err)
				return data, true
			}
			if ori.Code != hs.MsgTypeCommitVote {
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

	if hasViewChange {
		t.Fail()
	}
}

func TestCommitVoteFaultyHeightBad(t *testing.T) {
	H, R, fH, fN := uint64(4), uint64(0), uint64(5), int32(2)

	var locked int32
	atomic.StoreInt32(&locked, 0)

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

			var ori hs.Message
			if err := rlp.DecodeBytes(data, &ori); err != nil {
				log.Error("failed to decode message", "err", err)
				return data, true
			}
			if ori.Code != hs.MsgTypeCommitVote {
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

func TestCommitVoteFaultyRoundOk(t *testing.T) {
	H, R, fR, fN := uint64(4), uint64(0), uint64(1), int32(1)

	var locked int32
	atomic.StoreInt32(&locked, 0)

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

			var ori hs.Message
			if err := rlp.DecodeBytes(data, &ori); err != nil {
				log.Error("failed to decode message", "err", err)
				return data, true
			}
			if ori.Code != hs.MsgTypeCommitVote {
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

	if hasViewChange {
		t.Fail()
	}
}

func TestCommitVoteFaultyRoundBad(t *testing.T) {
	H, R, fR, fN := uint64(4), uint64(0), uint64(1), int32(2)

	var locked int32
	atomic.StoreInt32(&locked, 0)

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

			var ori hs.Message
			if err := rlp.DecodeBytes(data, &ori); err != nil {
				log.Error("failed to decode message", "err", err)
				return data, true
			}
			if ori.Code != hs.MsgTypeCommitVote {
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

func TestCommitVoteFaultyPayloadOk(t *testing.T) {
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
			if node.IsProposer() {
				return data, true
			}

			var ori hs.Message
			if err := rlp.DecodeBytes(data, &ori); err != nil {
				log.Error("failed to decode message", "err", err)
				return data, true
			}
			if ori.Code != hs.MsgTypeCommitVote {
				return data, true
			}
			msg := ori.Copy()
			msg.Msg = common.HexToHash("0x12346").Bytes()

			payload, err := node.resignMsg(msg)
			if err != nil {
				log.Error("failed to resign message", "err", err)
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

	if hasViewChange {
		t.Fail()
	}
}

func TestCommitVoteFaultyPayloadBad(t *testing.T) {
	H, R, fN := uint64(4), uint64(0), int32(2)

	var locked int32
	atomic.StoreInt32(&locked, 0)

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

			var ori hs.Message
			if err := rlp.DecodeBytes(data, &ori); err != nil {
				log.Error("failed to decode message", "err", err)
				return data, true
			}
			if ori.Code != hs.MsgTypeCommitVote {
				return data, true
			}
			msg := ori.Copy()
			msg.Msg = common.HexToHash("0x12346").Bytes()

			payload, err := node.resignMsg(msg)
			if err != nil {
				log.Error("failed to resign message", "err", err)
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
