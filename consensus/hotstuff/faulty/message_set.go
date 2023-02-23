package faulty

import (
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
)

// Construct a new message set to accumulate messages for given height/view number.
func NewMessageSet(valSet hs.ValidatorSet) *MessageSet {
	return &MessageSet{
		view: &hs.View{
			Round:  new(big.Int),
			Height: new(big.Int),
		},
		mu:   new(sync.RWMutex),
		msgs: make(map[common.Address]*hs.Message),
		vs:   valSet,
	}
}

type MessageSet struct {
	view *hs.View
	vs   hs.ValidatorSet
	mu   *sync.RWMutex
	msgs map[common.Address]*hs.Message
}

func (s *MessageSet) View() *hs.View {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.view
}

func (s *MessageSet) Add(msg *hs.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if index, v := s.vs.GetByAddress(msg.Address); index < 0 || v == nil {
		return fmt.Errorf("unauthorized address")
	}

	s.msgs[msg.Address] = msg
	return nil
}

func (s *MessageSet) Values() (result []*hs.Message) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, v := range s.msgs {
		result = append(result, v)
	}
	return
}

func (s *MessageSet) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.msgs)
}

func (s *MessageSet) Get(addr common.Address) *hs.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.msgs[addr]
}

func (s *MessageSet) String() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	addresses := make([]string, 0, len(s.msgs))
	for _, v := range s.msgs {
		addresses = append(addresses, v.Address.Hex())
	}
	return fmt.Sprintf("[%v]", strings.Join(addresses, ", "))
}
