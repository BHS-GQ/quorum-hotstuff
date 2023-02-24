package faulty

import (
	"github.com/ethereum/go-ethereum/common"
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
)

func (c *Core) isFaultTriggered(fault hs.FaultyMode, h uint64, r uint64) bool {
	ch, cr := c.CurrentSequence()
	return c.config.FaultyMode == fault.Uint64() &&
		ch == h &&
		cr == r
}

func (c *Core) pickNValidators(valSet hs.ValidatorSet, N int) []common.Address {
	vals := make([]common.Address, N)
	i := 0
	for _, addr := range valSet.AddressList() {
		if addr == c.Address() {
			continue
		}

		if i == N {
			break
		}

		vals[i] = addr
		i += 1
	}
	return vals
}

func (c *Core) splitValSet(valSet hs.ValidatorSet, N int) (hs.ValidatorSet, hs.ValidatorSet) {
	vsA := valSet.Copy()
	vsB := valSet.Copy()

	for _, addr := range c.pickNValidators(valSet, N) {
		vsA.RemoveValidator(addr)
	}

	for _, addr := range vsA.AddressList() {
		vsB.RemoveValidator(addr)
	}

	return vsA, vsB
}
