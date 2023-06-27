package mock

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
)

func TestDecideFaultyHeightBad(t *testing.T) {
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
			if ori.Code != hs.MsgTypeDecide {
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
	sys.Close(20)

	if !hasViewChange {
		t.Fail()
	}
}

func TestDecideFaultyRoundBad(t *testing.T) {
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
			if ori.Code != hs.MsgTypeDecide {
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

func TestDecideFaultyDiplomaBlockBad(t *testing.T) {
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
			if ori.Code != hs.MsgTypeDecide {
				return data, true
			}
			msg := ori.Copy()

			var diploma hs.Diploma
			if err := rlp.DecodeBytes(msg.Msg, &diploma); err != nil {
				log.Error("failed to decode diploma", "err", err)
				return data, true
			}
			diploma.BlockHash = common.HexToHash("0x123")
			if raw, err := rlp.EncodeToBytes(&diploma); err != nil {
				log.Error("failed to encode lockQC", "err", err)
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
	sys.Close(20)

	if !hasViewChange {
		t.Fail()
	}
}

func TestDecideFaultyDiplomaQCBlockBad(t *testing.T) {
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
			if ori.Code != hs.MsgTypeDecide {
				return data, true
			}
			msg := ori.Copy()

			var diploma hs.Diploma
			if err := rlp.DecodeBytes(msg.Msg, &diploma); err != nil {
				log.Error("failed to decode diploma", "err", err)
				return data, true
			}
			if raw, err := rlp.EncodeToBytes(diploma.CommitQC); err != nil {
				log.Error("failed to encode diploma.commitQC", "err", err)
				return data, true
			} else {
				var qc hs.QuorumCert
				if err = rlp.DecodeBytes(raw, &qc); err != nil {
					log.Error("failed to decode diploma.commitQC", "err", err)
					return data, true
				} else {
					qc.ProposedBlock = common.HexToHash("0x123")
				}
				var newDiploma = struct {
					CommitQC  *hs.QuorumCert
					BlockHash common.Hash
				}{
					&qc,
					diploma.BlockHash,
				}
				if raw, err := rlp.EncodeToBytes(&newDiploma); err != nil {
					log.Error("failed to encode new diploma", "err", err)
					return data, true
				} else {
					msg.Msg = raw
				}
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
	sys.Close(20)

	if !hasViewChange {
		t.Fail()
	}
}
