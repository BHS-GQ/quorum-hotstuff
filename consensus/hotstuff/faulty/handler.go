package faulty

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
)

// Start implements core.Engine.Start
func (c *Core) Start() error {
	c.isRunning = true
	c.current = nil

	c.subscribeEvents()
	go c.handleEvents()

	// Start a new round from last sequence + 1
	c.startNewRound(common.Big0)

	return nil
}

// Stop implements core.Engine.Stop
func (c *Core) Stop() error {
	c.stopTimer()
	c.unsubscribeEvents()
	c.isRunning = false

	return nil
}

func (c *Core) Address() common.Address {
	return c.signer.Address()
}

func (c *Core) IsProposer() bool {
	return c.valSet.IsProposer(c.backend.Address())
}

func (c *Core) IsCurrentProposal(blockHash common.Hash) bool {
	if c.current == nil {
		return false
	}
	if proposal := c.current.TreeNode(); proposal != nil && proposal.Hash() == blockHash {
		return true
	}
	if req := c.current.PendingRequest(); req != nil && req.Block != nil && req.Block.Hash() == blockHash {
		return true
	}
	return false
}

func (c *Core) CurrentSequence() (uint64, uint64) {
	view := c.currentView()
	return view.HeightU64(), view.RoundU64()
}

// ----------------------------------------------------------------------------

// Subscribe both internal and external events
func (c *Core) subscribeEvents() {
	c.events = c.backend.EventMux().Subscribe(
		// external events
		hs.RequestEvent{},
		// internal events
		hs.MessageEvent{},
		backlogEvent{},
	)
	c.timeoutSub = c.backend.EventMux().Subscribe(
		timeoutEvent{},
	)
	c.finalCommittedSub = c.backend.EventMux().Subscribe(
		hs.FinalCommittedEvent{},
	)
}

func (c *Core) handleEvents() {
	logger := c.logger.New("handleEvents")

	for {
		select {
		case event, ok := <-c.events.Chan():
			if !ok {
				logger.Error("Failed to receive msg Event", "err", "subscribe event chan out empty")
				return
			}
			// A real Event arrived, process interesting content
			switch ev := event.Data.(type) {
			case hs.RequestEvent:
				c.handleRequest(&hs.Request{Block: ev.Block})

			case hs.MessageEvent:
				c.handleMsg(ev.Src, ev.Payload)

			case backlogEvent:
				c.handleCheckedMsg(ev.msg)
			}

		case _, ok := <-c.timeoutSub.Chan():
			if !ok {
				logger.Error("Failed to receive timeout Event")
				return
			}
			c.handleTimeoutMsg()

		case evt, ok := <-c.finalCommittedSub.Chan():
			if !ok {
				logger.Error("Failed to receive finalCommitted Event")
				return
			}
			switch ev := evt.Data.(type) {
			case hs.FinalCommittedEvent:
				c.handleFinalCommitted(ev.Header)
			}
		}
	}
}

// sendEvent sends events to mux
func (c *Core) sendEvent(ev interface{}) {
	c.backend.EventMux().Post(ev)
}

func (c *Core) handleMsg(val common.Address, payload []byte) error {
	logger := c.logger.New()

	// Decode Message and check its signature
	msg := new(hs.Message)
	if err := msg.FromPayload(val, payload, c.validateFn); err != nil {
		logger.Error("Failed to decode Message from payload", "err", err)
		return hs.ErrFailedDecodeMessage
	}

	// Only accept message if the src is consensus participant
	index, src := c.valSet.GetByAddress(val)
	if index < 0 || src == nil {
		logger.Error("Invalid address in Message", "msg", msg)
		return hs.ErrInvalidSigner
	}

	// handle checked Message
	return c.handleCheckedMsg(msg)
}

func (c *Core) handleCheckedMsg(msg *hs.Message) (err error) {
	if c.current == nil {
		c.logger.Error("engine state not prepared...")
		return
	}

	switch msg.Code {
	case hs.MsgTypeNewView:
		err = c.handleNewView(msg)
	case hs.MsgTypePrepare:
		err = c.handlePrepare(msg)
	case hs.MsgTypePrepareVote:
		err = c.handlePrepareVote(msg)
	case hs.MsgTypePreCommit:
		err = c.handlePreCommit(msg)
	case hs.MsgTypePreCommitVote:
		err = c.handlePreCommitVote(msg)
	case hs.MsgTypeCommit:
		err = c.handleCommit(msg)
	case hs.MsgTypeCommitVote:
		err = c.handleCommitVote(msg)
	case hs.MsgTypeDecide:
		err = c.handleDecide(msg)
	default:
		err = hs.ErrInvalidMessage
		c.logger.Error("msg type invalid", "unknown type", msg.Code)
	}

	if err == hs.ErrFutureMessage {
		c.storeBacklog(msg)
	}
	return
}

func (c *Core) handleTimeoutMsg() {
	c.logger.Trace("handleTimeout", "state", c.currentState(), "view", c.currentView())
	round := new(big.Int).Add(c.current.Round(), common.Big1)
	c.startNewRound(round)
}

// Unsubscribe all events
func (c *Core) unsubscribeEvents() {
	c.events.Unsubscribe()
	c.timeoutSub.Unsubscribe()
	c.finalCommittedSub.Unsubscribe()
}

func (c *Core) broadcast(code hs.MsgType, payload []byte) {
	logger := c.logger.New("state", c.currentState())

	// Forbid non-validator nodest to send message to leader
	if index, _ := c.valSet.GetByAddress(c.Address()); index < 0 {
		return
	}

	msg := hs.NewCleanMessage(c.currentView(), code, payload)
	payload, err := c.finalizeMessage(msg)
	if err != nil {
		logger.Error("Failed to finalize Message", "msg", msg, "err", err)
		return
	}

	switch msg.Code {
	case hs.MsgTypeNewView, hs.MsgTypePrepareVote, hs.MsgTypePreCommitVote, hs.MsgTypeCommitVote:
		// Send a vote-type message to leader

		if err = c.backend.Unicast(c.valSet, payload); err != nil {
			logger.Error("Failed to unicast Message", "msg", msg, "err", err)
		}

	case hs.MsgTypePrepare, hs.MsgTypePreCommit, hs.MsgTypeCommit, hs.MsgTypeDecide:
		// Leader broadcasts decision to replicas

		if err = c.backend.Broadcast(c.valSet, payload); err != nil {
			logger.Error("Failed to broadcast Message", "msg", msg, "err", err)
		}
	default:
		logger.Error("invalid msg type", "msg", msg)
	}
}

func (c *Core) finalizeMessage(msg *hs.Message) ([]byte, error) {
	var (
		sig     []byte
		msgHash common.Hash
		err     error
	)

	// Sign Message (ECDSA)
	if _, err = msg.PayloadNoSig(); err != nil {
		return nil, err
	}
	msgHash, err = msg.Hash()
	if sig, err = c.signer.Sign(msgHash); err != nil {
		return nil, err
	} else {
		msg.Signature = sig
	}

	// Convert to payload
	return msg.Payload()
}
