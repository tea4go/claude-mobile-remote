package session

import (
	"context"

	projectclaude "github.com/tea4go/claude-mobile-remote/internal/claude"
	"github.com/tea4go/claude-mobile-remote/protocol"
)

// Host is the narrow Claude lifecycle owned exclusively by hostWorker, plus the
// two capabilities the Controller must reach outside the serialized run loop:
// the pending action side channel and Interrupt (B2 design §3.4).
type Host interface {
	Connect(context.Context) error
	RunTurn(context.Context, string, func(projectclaude.Event)) error
	Actions() <-chan *projectclaude.ActionRequest
	Interrupt(context.Context) error
	Close() error
}

// PermissionModeHost is optional so existing embedding hosts can adopt the
// read/write permission contract without changing their turn lifecycle.
type PermissionModeHost interface {
	SetPermissionMode(context.Context, protocol.PermissionMode) error
}

type workerRequest struct {
	RequestID string
	Prompt    string
	Context   context.Context
}

type workerEvent struct {
	RequestID string
	Adapter   projectclaude.Event
}

type workerResult struct {
	RequestID string
	Err       error
}

// hostWorker serially owns the Host lifecycle and reports all work to its
// future controller through channels.
type hostWorker struct {
	host      Host
	requests  chan workerRequest
	events    chan workerEvent
	results   chan workerResult
	connected chan error
	stop      chan struct{}
	closed    chan error
}

func newHostWorker(host Host) *hostWorker {
	return &hostWorker{
		host:      host,
		requests:  make(chan workerRequest),
		events:    make(chan workerEvent),
		results:   make(chan workerResult),
		connected: make(chan error, 1),
		stop:      make(chan struct{}),
		closed:    make(chan error, 1),
	}
}

func (w *hostWorker) start(ctx context.Context) {
	go w.run(ctx)
}

// actions exposes the Host side channel unchanged. Pending actions must bypass
// the run loop: the permission callback runs on the SDK read goroutine, so
// routing it through requests/events would deadlock against the Controller.
func (w *hostWorker) actions() <-chan *projectclaude.ActionRequest {
	return w.host.Actions()
}

// interrupt reaches the Host directly. It is only safe without a pending
// action, because the SDK control response travels on the blocked read loop.
func (w *hostWorker) interrupt(ctx context.Context) error {
	return w.host.Interrupt(ctx)
}

func (w *hostWorker) run(ctx context.Context) {
	if err := w.host.Connect(ctx); err != nil {
		w.connected <- err
		w.closed <- w.host.Close()
		return
	}
	w.connected <- nil

	for {
		select {
		case request := <-w.requests:
			err := w.host.RunTurn(request.Context, request.Prompt, func(event projectclaude.Event) {
				w.events <- workerEvent{RequestID: request.RequestID, Adapter: event}
			})
			w.results <- workerResult{RequestID: request.RequestID, Err: err}
		case <-w.stop:
			w.closed <- w.host.Close()
			return
		}
	}
}
