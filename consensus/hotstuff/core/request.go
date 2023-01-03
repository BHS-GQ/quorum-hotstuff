package core

import (
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
)

func (c *core) handleRequest(request *hs.Request) error {
	logger := c.newLogger()
	if err := c.checkRequestMsg(request); err != nil {
		if err == errInvalidMessage {
			logger.Warn("invalid request")
		} else if err == errFutureMessage {
			c.storeRequestMsg(request)
		} else {
			logger.Warn("unexpected request", "err", err, "number", request.Block.Number(), "hash", request.Block.Hash())
		}
		return err
	}
	logger.Trace("handleRequest", "number", request.Block.Number(), "hash", request.Block.Hash())

	switch c.currentState() {
	case hs.StateAcceptRequest:
		// store request and prepare to use it after highQC
		c.storeRequestMsg(request)

	case hs.StateHighQC:
		// consensus step is blocked for proposal is not ready
		if c.current.PendingRequest() == nil ||
			c.current.PendingRequest().Block.NumberU64() < c.current.HeightU64() {
			c.current.SetPendingRequest(request)
			c.sendPrepare()
		} else {
			logger.Trace("PendingRequest exist")
		}

	default:
		// store request for `changeView` if node is not the proposer at current round.
		if c.current.PendingRequest() == nil {
			c.current.SetPendingRequest(request)
		}
	}

	return nil
}

// check request state
// return errInvalidMessage if the message is invalid
// return errFutureMessage if the sequence of proposal is larger than current sequence
// return errOldMessage if the sequence of proposal is smaller than current sequence
func (c *core) checkRequestMsg(request *hs.Request) error {
	if request == nil || request.Block == nil {
		return errInvalidMessage
	}

	if c := c.current.Height().Cmp(request.Block.Number()); c > 0 {
		return errOldMessage
	} else if c < 0 {
		return errFutureMessage
	} else {
		return nil
	}
}

func (c *core) storeRequestMsg(request *hs.Request) {
	logger := c.newLogger()

	logger.Trace("Store future request", "number", request.Block.Number(), "hash", request.Block.Hash())

	c.pendingRequestsMu.Lock()
	defer c.pendingRequestsMu.Unlock()

	c.pendingRequests.Push(request, -request.Block.Number().Int64())
}

func (c *core) processPendingRequests() {
	c.pendingRequestsMu.Lock()
	defer c.pendingRequestsMu.Unlock()

	for !(c.pendingRequests.Empty()) {
		m, prio := c.pendingRequests.Pop()
		r, ok := m.(*hs.Request)
		if !ok {
			c.logger.Warn("Malformed request, skip", "msg", m)
			continue
		}
		// Push back if it's a future message
		if err := c.checkRequestMsg(r); err != nil {
			if err == errFutureMessage {
				c.logger.Trace("Stop processing request", "number", r.Block.Number(), "hash", r.Block.Hash())
				c.pendingRequests.Push(m, prio)
				break
			}
			c.logger.Trace("Skip the pending request", "number", r.Block.Number(), "hash", r.Block.Hash(), "err", err)
			continue
		} else {
			c.logger.Trace("Post pending request", "number", r.Block.Number(), "hash", r.Block.Hash())
			go c.sendEvent(hs.RequestEvent{
				Block: r.Block,
			})
		}
	}
}
