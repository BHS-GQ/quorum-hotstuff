package backend

import hs "github.com/ethereum/go-ethereum/consensus/hotstuff"

// todo: use snap or reconfig validators group
func (s *Backend) snap() hs.ValidatorSet {
	return s.valset.Copy()
}
