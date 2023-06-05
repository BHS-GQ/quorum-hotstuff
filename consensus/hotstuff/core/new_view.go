package core

import hs "github.com/ethereum/go-ethereum/consensus/hotstuff"

// sendNewView
//	- Replicas get latest prepareQC and send it in new-view message
//  - BHS Spec: implements nextView interrupt (line 36)
func (c *Core) sendNewView() {
	logger := c.newLogger()
	code := hs.MsgTypeNewView

	prepareQC := c.current.PrepareQC()
	payload, err := hs.Encode(prepareQC)
	if err != nil {
		logger.Trace("Failed to encode", "msgCode", code, "err", err)
		return
	}

	c.broadcast(code, payload)
	logger.Trace("sendNewView", "msgCode", code)
}

// handleNewView
// 	- Leader waits for new-view messages
//  - Quorum: pick `highQC`, the max `prepareQC` by view sequence
// 	- `hs.stateHighQC` denotes that this node is ready to send block and prepare message
//  - BHS Spec: implements leader portion of PREPARE phase (lines 2-6)
func (c *Core) handleNewView(data *hs.Message) error {
	var (
		logger    = c.newLogger()
		prepareQC *hs.QuorumCert
		code      = data.Code
		src       = data.Address
	)

	// check message
	if err := data.Decode(&prepareQC); err != nil {
		logger.Trace("Failed to decode", "msgCode", code, "src", src, "err", err)
		return hs.ErrFailedDecodeNewView
	}
	if err := c.checkView(data.View); err != nil {
		logger.Trace("Failed to check view", "msgCode", code, "src", src, "err", err)
		return err
	}
	if err := c.checkMsgDest(); err != nil {
		logger.Trace("Failed to check proposer", "msgCode", code, "src", src, "err", err)
		return err
	}

	// ensure remote `prepareQC` is legal.
	if err := c.verifyQC(data, prepareQC); err != nil {
		logger.Trace("Failed to verify prepareQC", "msgCode", code, "src", src, "err", err)
		return err
	}
	// messages queued in messageSet to ensure there will be at least 2/3 validators on the same step
	if err := c.current.AddNewViews(data); err != nil {
		logger.Trace("Failed to add new view", "msgCode", code, "src", src, "err", err)
		return hs.ErrAddNewViews
	}

	logger.Trace("handleNewView", "msgCode", code, "src", src, "prepareQC", prepareQC.ProposedBlock)

	if size := c.current.NewViewSize(); size >= c.valSet.Q() && c.currentState() < hs.StateHighQC {
		highQC, err := c.getHighQC()
		if err != nil {
			logger.Trace("Failed to get highQC", "msgCode", code, "err", err)
			return err
		}
		c.current.SetHighQC(highQC)
		c.setCurrentState(hs.StateHighQC)

		logger.Trace("acceptHighQC", "msgCode", code, "prepareQC", prepareQC.ProposedBlock, "msgSize", size)
		c.sendPrepare()
	}

	return nil
}

// getHighQC leader find the highest `prepareQC` as highQC by `view` sequence.
func (c *Core) getHighQC() (*hs.QuorumCert, error) {
	var highQC *hs.QuorumCert
	for _, data := range c.current.NewViews() {
		var qc *hs.QuorumCert
		if err := data.Decode(&qc); err != nil {
			return nil, err
		}
		if highQC == nil || highQC.View.Cmp(qc.View) < 0 {
			highQC = qc
		}
	}
	if highQC == nil {
		return nil, hs.ErrNilHighQC
	}
	return highQC, nil
}
