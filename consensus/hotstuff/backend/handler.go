package backend

import (
	"bytes"
	"io/ioutil"
	"math/big"
	"reflect"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/p2p"
	lru "github.com/hashicorp/golang-lru"
)

const (
	NewBlockMsg = 0x07
	hotstuffMsg = 0x11
)

func (s *Backend) decode(msg p2p.Msg) ([]byte, common.Hash, error) {
	var data []byte
	if err := msg.Decode(&data); err != nil {
		return nil, common.Hash{}, hs.ErrDecodeFailed
	}

	return data, hs.RLPHash(data), nil
}

// for quorum consensus framework
func (sb *Backend) Protocol() consensus.Protocol {
	return consensus.HotstuffProtocol
}

// HandleMsg implements consensus.Handler.HandleMsg
func (s *Backend) HandleMsg(addr common.Address, msg p2p.Msg) (bool, error) {
	s.coreMu.Lock()
	defer s.coreMu.Unlock()
	if msg.Code == hotstuffMsg {
		if !s.coreStarted {
			return true, ErrStoppedEngine
		}

		data, hash, err := s.decode(msg)
		if err != nil {
			return true, hs.ErrDecodeFailed
		}
		// Mark peer's message
		ms, ok := s.recentMessages.Get(addr)
		var m *lru.ARCCache
		if ok {
			m, _ = ms.(*lru.ARCCache)
		} else {
			m, _ = lru.NewARC(inmemoryMessages)
			s.recentMessages.Add(addr, m)
		}
		m.Add(hash, true)

		// Mark self known message
		if _, ok := s.knownMessages.Get(hash); ok {
			return true, nil
		}
		s.knownMessages.Add(hash, true)

		go s.eventMux.Post(hs.MessageEvent{
			Src:     addr,
			Payload: data,
		})
		return true, nil
	}
	if msg.Code == NewBlockMsg && s.core.IsProposer() { // eth.NewBlockMsg: import cycle
		// this case is to safeguard the race of similar block which gets propagated from other node while this node is proposing
		// as p2p.Msg can only be decoded once (get EOF for any subsequence read), we need to make sure the payload is restored after we decode it
		s.logger.Debug("proposer received NewBlockMsg", "size", msg.Size, "payload.type", reflect.TypeOf(msg.Payload), "sender", addr)
		if reader, ok := msg.Payload.(*bytes.Reader); ok {
			payload, err := ioutil.ReadAll(reader)
			if err != nil {
				return true, err
			}
			reader.Reset(payload)       // ready to be decoded
			defer reader.Reset(payload) // restore so main eth/handler can decode
			var request struct {        // this has to be same as eth/protocol.go#newBlockData as we are reading NewBlockMsg
				Block *types.Block
				TD    *big.Int
			}
			if err := msg.Decode(&request); err != nil {
				s.logger.Debug("Proposer was unable to decode the NewBlockMsg", "error", err)
				return false, nil
			}
			newRequestedBlock := request.Block
			if newRequestedBlock.Header().MixDigest == types.HotstuffDigest && s.core.IsCurrentProposal(newRequestedBlock.Hash()) {
				s.logger.Debug("Proposer already proposed this block", "hash", newRequestedBlock.Hash(), "sender", addr)
				return true, nil
			}
		}
	}
	return false, nil
}

// SetBroadcaster implements consensus.Handler.SetBroadcaster
func (s *Backend) SetBroadcaster(broadcaster consensus.Broadcaster) {
	s.broadcaster = broadcaster
}

func (s *Backend) GetBroadcaster() consensus.Broadcaster {
	return s.broadcaster
}

func (s *Backend) NewChainHead(header *types.Header) error {
	s.coreMu.RLock()
	defer s.coreMu.RUnlock()
	if !s.coreStarted {
		return ErrStoppedEngine
	}
	go s.eventMux.Post(hs.FinalCommittedEvent{Header: header})
	return nil
}

func (s *Backend) SubscribeBlock(ch chan<- consensus.ExecutedBlock) event.Subscription {
	return s.executeFeed.Subscribe(ch)
}
