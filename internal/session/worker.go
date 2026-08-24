package session

import (
	"context"

	projectclaude "github.com/tea4go/claude-mobile-remote/internal/claude"
)

// Host is the narrow Claude lifecycle owned exclusively by hostWorker.
type Host interface {
	Connect(context.Context) error
	RunTurn(context.Context, string, func(projectclaude.Event)) error
	Close() error
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
