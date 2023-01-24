package mock

import (
	"math/big"
	"testing"
	"time"

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
