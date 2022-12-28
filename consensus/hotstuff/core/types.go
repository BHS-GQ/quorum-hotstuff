package core

import hs "github.com/ethereum/go-ethereum/consensus/hotstuff"

type timeoutEvent struct{}
type backlogEvent struct {
	src hs.Validator
	msg *hs.Message
}
