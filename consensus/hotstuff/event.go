package hotstuff

import "github.com/ethereum/go-ethereum/core/types"

// RequestEvent is posted to propose a proposal (posting the incoming block to
// the main hotstuff engine anyway regardless of being the speaker or delegators)
type RequestEvent struct {
	Proposal Proposal
}

// MessageEvent is posted for HotStuff engine communication (posting the incoming
// communication messages to the main hotstuff engine anyway)
type MessageEvent struct {
	Payload []byte
}

// FinalCommittedEvent is posted when a proposal is committed
type FinalCommittedEvent struct {
	Header *types.Header
}
