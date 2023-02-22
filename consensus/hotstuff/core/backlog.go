package core

import (
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/prque"
	hs "github.com/ethereum/go-ethereum/consensus/hotstuff"
)

func (c *Core) storeBacklog(msg *hs.Message) {
	logger := c.newLogger()

	src := msg.Address
	if src == c.Address() {
		logger.Trace("Backlog from self")
		return
	}
	if _, v := c.valSet.GetByAddress(src); v == nil {
		logger.Trace("Backlog from unknown validator", "address", src)
		return
	}

	logger.Trace("Retrieving backlog queue", "msg", msg.Code, "src", src, "backlogs_size", c.backlogs.Size(src))

	c.backlogs.Push(msg)
}

func (c *Core) processBacklog() {
	logger := c.newLogger()

	c.backlogs.mu.Lock()
	defer c.backlogs.mu.Unlock()

	for addr, queue := range c.backlogs.queue {
		if queue == nil {
			continue
		}
		_, src := c.valSet.GetByAddress(addr)
		if src == nil {
			logger.Trace("Skip the backlog", "unknown validator", addr)
			continue
		}

		isFuture := false
		for !(queue.Empty() || isFuture) {
			data, priority := queue.Pop()
			msg, ok := data.(*hs.Message)
			if !ok {
				logger.Trace("Skip the backlog, invalid Message")
				continue
			}
			if err := c.checkView(msg.View); err != nil {
				if err == hs.ErrFutureMessage {
					queue.Push(data, priority)
					isFuture = true
					break
				}
				logger.Trace("Skip the backlog", "msg view", msg.View, "err", err)
				continue
			}

			logger.Trace("Replay the backlog", "msg", msg)
			go c.sendEvent(backlogEvent{src: src, msg: msg})
		}
	}
}

type backlog struct {
	mu    *sync.RWMutex
	queue map[common.Address]*prque.Prque
}

func newBackLog() *backlog {
	return &backlog{
		mu:    new(sync.RWMutex),
		queue: make(map[common.Address]*prque.Prque),
	}
}

func (b *backlog) Push(msg *hs.Message) {
	if msg == nil || msg.Address == hs.EmptyAddress ||
		msg.View == nil || msg.Code > hs.MsgTypeDecide {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	addr := msg.Address
	if _, ok := b.queue[addr]; !ok {
		b.queue[addr] = prque.New(nil)
	}
	priority := b.toPriority(msg.Code, msg.View)
	b.queue[addr].Push(msg, priority)
}

func (b *backlog) Pop(addr common.Address) (data *hs.Message, priority int64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.queue[addr]; !ok {
		return
	} else {
		item, p := b.queue[addr].Pop()
		data = item.(*hs.Message)
		priority = p
		return
	}
}

func (b *backlog) Size(addr common.Address) int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if que, ok := b.queue[addr]; !ok {
		return 0
	} else {
		return que.Size()
	}
}

var messagePriorityTable = map[hs.MsgType]int64{
	hs.MsgTypeNewView:       1,
	hs.MsgTypePrepare:       2,
	hs.MsgTypePrepareVote:   3,
	hs.MsgTypePreCommit:     4,
	hs.MsgTypePreCommitVote: 5,
	hs.MsgTypeCommit:        6,
	hs.MsgTypeCommitVote:    7,
	hs.MsgTypeDecide:        8,
}

func (b *backlog) toPriority(msgCode hs.MsgType, view *hs.View) int64 {
	priority := -(view.Height.Int64()*100 + view.Round.Int64()*10 + messagePriorityTable[msgCode])
	return priority
}
