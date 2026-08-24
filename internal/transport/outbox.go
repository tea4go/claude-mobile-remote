package transport

import (
	"errors"
	"sync"
)

var (
	errOutboxOverflow      = errors.New("outbox overflow")
	errOutboxClosed        = errors.New("outbox closed")
	errEarlyBufferOverflow = errors.New("early event buffer overflow")
	errCommandInFlight     = errors.New("command already in flight")
)

type outbox struct {
	mu       sync.Mutex
	cond     *sync.Cond
	items    [][]byte
	bytes    int
	maxItems int
	maxBytes int
	closed   bool
}

func newOutbox(maxItems, maxBytes int) *outbox {
	queue := &outbox{maxItems: maxItems, maxBytes: maxBytes}
	queue.cond = sync.NewCond(&queue.mu)
	return queue
}

func (q *outbox) Enqueue(data []byte) error {
	copyOfData := append([]byte(nil), data...)
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return errOutboxClosed
	}
	if len(q.items) >= q.maxItems || q.bytes+len(copyOfData) > q.maxBytes {
		return errOutboxOverflow
	}
	q.items = append(q.items, copyOfData)
	q.bytes += len(copyOfData)
	q.cond.Signal()
	return nil
}

func (q *outbox) Next() ([]byte, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.items) == 0 && !q.closed {
		q.cond.Wait()
	}
	if len(q.items) == 0 {
		return nil, false
	}
	item := q.items[0]
	q.items[0] = nil
	q.items = q.items[1:]
	q.bytes -= len(item)
	return item, true
}

func (q *outbox) Close() {
	q.mu.Lock()
	q.closed = true
	q.cond.Broadcast()
	q.mu.Unlock()
}

type commandGate struct {
	mu            sync.Mutex
	outbox        *outbox
	active        bool
	commandID     string
	early         [][]byte
	earlyBytes    int
	maxEarlyItems int
	maxEarlyBytes int
}

func newCommandGate(outbox *outbox, maxEarlyItems, maxEarlyBytes int) *commandGate {
	return &commandGate{outbox: outbox, maxEarlyItems: maxEarlyItems, maxEarlyBytes: maxEarlyBytes}
}

func (g *commandGate) Begin(commandID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.active {
		return errCommandInFlight
	}
	g.active = true
	g.commandID = commandID
	return nil
}

func (g *commandGate) EnqueueEvent(data []byte) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.active {
		return g.outbox.Enqueue(data)
	}
	if len(g.early) >= g.maxEarlyItems || g.earlyBytes+len(data) > g.maxEarlyBytes {
		return errEarlyBufferOverflow
	}
	copyOfData := append([]byte(nil), data...)
	g.early = append(g.early, copyOfData)
	g.earlyBytes += len(copyOfData)
	return nil
}

func (g *commandGate) Finish(result []byte) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.active {
		return errCommandInFlight
	}
	if err := g.outbox.Enqueue(result); err != nil {
		g.resetLocked()
		return err
	}
	for _, event := range g.early {
		if err := g.outbox.Enqueue(event); err != nil {
			g.resetLocked()
			return err
		}
	}
	g.resetLocked()
	return nil
}

func (g *commandGate) Abort(result []byte) error {
	return g.Finish(result)
}

func (g *commandGate) resetLocked() {
	for index := range g.early {
		g.early[index] = nil
	}
	g.active = false
	g.commandID = ""
	g.early = nil
	g.earlyBytes = 0
}
