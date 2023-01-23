package backend

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
)

// API is a user facing RPC API to allow controlling the address and voting
// mechanisms of the HotStuff scheme.
type API struct {
	chain    consensus.ChainHeaderReader
	hotstuff *backend
}

// Proposals returns the current proposals the node tries to uphold and vote on.
func (api *API) Proposals() map[common.Address]bool {
	api.hotstuff.sigMu.RLock()
	defer api.hotstuff.sigMu.RUnlock()

	proposals := make(map[common.Address]bool)
	for address, auth := range api.hotstuff.proposals {
		proposals[address] = auth
	}
	return proposals
}

// todo: add/del candidate validators approach console or api
// Propose injects a new authorization candidate that the validator will attempt to
// push through.
func (api *API) Propose(address common.Address, auth bool) {
	api.hotstuff.sigMu.Lock()
	defer api.hotstuff.sigMu.Unlock()

	api.hotstuff.proposals[address] = auth
}

// Discard drops a currently running candidate, stopping the validator from casting
// further votes (either for or against).
func (api *API) Discard(address common.Address) {
	api.hotstuff.sigMu.Lock()
	defer api.hotstuff.sigMu.Unlock()

	delete(api.hotstuff.proposals, address)
}

// CurrentView retrieve current proposal height and round number
func (api *API) CurrentSequence() (uint64, uint64) {
	return api.hotstuff.core.CurrentSequence()
}

func (api *API) IsProposer() bool {
	return api.hotstuff.core.IsProposer()
}
