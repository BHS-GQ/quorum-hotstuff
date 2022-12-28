package core

import (
	"math"
	"time"
)

// we use timeout in every view to ensure consensus liveness.  and the view timeout
// calculating format as follow:
// *	t = requestTimeout + 2^round
// the round started from 0.
//
// the waiting time in every round is greater than the last one, so that all nodes can catch up
// the same round.

func (c *core) newRoundChangeTimer() {
	c.stopTimer()

	// set timeout based on the round number
	timeout := time.Duration(c.config.RequestTimeout) * time.Millisecond
	round := c.current.Round().Uint64()
	if round > 0 {
		timeout += time.Duration(math.Pow(2, float64(round))) * time.Second
	}
	c.roundChangeTimer = time.AfterFunc(timeout, func() {
		c.sendEvent(timeoutEvent{})
	})
}

func (c *core) stopTimer() {
	if c.roundChangeTimer != nil {
		c.roundChangeTimer.Stop()
	}
}
