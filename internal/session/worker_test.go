package session

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	projectclaude "github.com/tea4go/claude-mobile-remote/internal/claude"
)

const workerTestTimeout = time.Second

type runControl struct {
	events []projectclaude.Event
	err    error
}

type workerFakeHost struct {
	connectErr error
	closeErr   error

	starts   chan string
	controls chan runControl

	mu           sync.Mutex
	connectCalls int
	closeCalls   int
	active       int
	maxActive    int
}

func newWorkerFakeHost() *workerFakeHost {
	return &workerFakeHost{
		starts:   make(chan string, 2),
		controls: make(chan runControl, 2),
	}
}

func (f *workerFakeHost) Connect(context.Context) error {
	f.mu.Lock()
	f.connectCalls++
	f.mu.Unlock()
	return f.connectErr
}

func (f *workerFakeHost) RunTurn(_ context.Context, prompt string, emit func(projectclaude.Event)) error {
	f.mu.Lock()
	f.active++
	if f.active > f.maxActive {
		f.maxActive = f.active
	}
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.active--
		f.mu.Unlock()
	}()

	f.starts <- prompt
	control := <-f.controls
	for _, event := range control.events {
		emit(event)
	}
	return control.err
}

func (f *workerFakeHost) Close() error {
	f.mu.Lock()
	f.closeCalls++
	f.mu.Unlock()
	return f.closeErr
}

func (f *workerFakeHost) finish(t *testing.T, control runControl) {
	t.Helper()
	select {
	case f.controls <- control:
	case <-time.After(workerTestTimeout):
		t.Fatal("timed out releasing fake RunTurn")
	}
}

func (f *workerFakeHost) callCounts() (connectCalls, closeCalls, maxActive int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.connectCalls, f.closeCalls, f.maxActive
}

func receiveWorkerValue[T any](t *testing.T, channel <-chan T, description string) T {
	t.Helper()
	select {
	case value := <-channel:
		return value
	case <-time.After(workerTestTimeout):
		t.Fatalf("timed out waiting for %s", description)
		var zero T
		return zero
	}
}

func sendWorkerRequest(t *testing.T, worker *hostWorker, request workerRequest) {
	t.Helper()
	select {
	case worker.requests <- request:
	case <-time.After(workerTestTimeout):
		t.Fatal("timed out sending worker request")
	}
}

func stopWorker(t *testing.T, worker *hostWorker) error {
	t.Helper()
	close(worker.stop)
	return receiveWorkerValue(t, worker.closed, "worker close")
}

func TestWorkerOwnsHostLifecycleAndSerializesTurns(t *testing.T) {
	host := newWorkerFakeHost()
	worker := newHostWorker(host)
	worker.start(context.Background())

	if err := receiveWorkerValue(t, worker.connected, "worker connection"); err != nil {
		t.Fatalf("Connect error = %v, want nil", err)
	}

	sendWorkerRequest(t, worker, workerRequest{
		RequestID: "request-1",
		Prompt:    "first",
		Context:   context.Background(),
	})
	if prompt := receiveWorkerValue(t, host.starts, "first RunTurn"); prompt != "first" {
		t.Fatalf("first RunTurn prompt = %q, want %q", prompt, "first")
	}

	select {
	case worker.requests <- workerRequest{RequestID: "request-2", Prompt: "second", Context: context.Background()}:
		t.Fatal("second request was accepted while first RunTurn was active")
	default:
	}

	host.finish(t, runControl{})
	result := receiveWorkerValue(t, worker.results, "first worker result")
	if result.RequestID != "request-1" || result.Err != nil {
		t.Fatalf("first result = %#v, want request-1 with nil error", result)
	}

	sendWorkerRequest(t, worker, workerRequest{
		RequestID: "request-2",
		Prompt:    "second",
		Context:   context.Background(),
	})
	if prompt := receiveWorkerValue(t, host.starts, "second RunTurn"); prompt != "second" {
		t.Fatalf("second RunTurn prompt = %q, want %q", prompt, "second")
	}
	host.finish(t, runControl{})
	result = receiveWorkerValue(t, worker.results, "second worker result")
	if result.RequestID != "request-2" || result.Err != nil {
		t.Fatalf("second result = %#v, want request-2 with nil error", result)
	}

	if err := stopWorker(t, worker); err != nil {
		t.Fatalf("Close error = %v, want nil", err)
	}
	connectCalls, closeCalls, maxActive := host.callCounts()
	if connectCalls != 1 || closeCalls != 1 || maxActive != 1 {
		t.Fatalf("host calls = Connect:%d Close:%d max RunTurn:%d, want 1, 1, 1", connectCalls, closeCalls, maxActive)
	}
}

func TestWorkerForwardsAdapterEventsWithRequestID(t *testing.T) {
	host := newWorkerFakeHost()
	worker := newHostWorker(host)
	worker.start(context.Background())
	if err := receiveWorkerValue(t, worker.connected, "worker connection"); err != nil {
		t.Fatalf("Connect error = %v, want nil", err)
	}

	sendWorkerRequest(t, worker, workerRequest{
		RequestID: "request-1",
		Prompt:    "first",
		Context:   context.Background(),
	})
	receiveWorkerValue(t, host.starts, "RunTurn")

	wantAdapter := projectclaude.Event{Kind: projectclaude.EventText, SessionID: "session-1", Text: "chunk", Source: projectclaude.SessionIDFromInit}
	host.finish(t, runControl{events: []projectclaude.Event{wantAdapter}})
	gotEvent := receiveWorkerValue(t, worker.events, "forwarded adapter event")
	if gotEvent.RequestID != "request-1" {
		t.Fatalf("event request ID = %q, want %q", gotEvent.RequestID, "request-1")
	}
	if gotEvent.Adapter != wantAdapter {
		t.Fatalf("adapter event = %#v, want %#v", gotEvent.Adapter, wantAdapter)
	}

	result := receiveWorkerValue(t, worker.results, "worker result")
	if result.RequestID != "request-1" || result.Err != nil {
		t.Fatalf("result = %#v, want request-1 with nil error", result)
	}
	if err := stopWorker(t, worker); err != nil {
		t.Fatalf("Close error = %v, want nil", err)
	}
}

func TestWorkerReportsConnectErrorAndClosesHost(t *testing.T) {
	connectErr := errors.New("connect failed")
	host := newWorkerFakeHost()
	host.connectErr = connectErr
	worker := newHostWorker(host)
	worker.start(context.Background())

	if err := receiveWorkerValue(t, worker.connected, "connection error"); !errors.Is(err, connectErr) {
		t.Fatalf("Connect error = %v, want %v", err, connectErr)
	}
	if err := receiveWorkerValue(t, worker.closed, "worker close after connection error"); err != nil {
		t.Fatalf("Close error = %v, want nil", err)
	}
	connectCalls, closeCalls, maxActive := host.callCounts()
	if connectCalls != 1 || closeCalls != 1 || maxActive != 0 {
		t.Fatalf("host calls = Connect:%d Close:%d max RunTurn:%d, want 1, 1, 0", connectCalls, closeCalls, maxActive)
	}
}

func TestWorkerPreservesRunTurnErrorAndRequestID(t *testing.T) {
	runErr := errors.New("turn failed")
	host := newWorkerFakeHost()
	worker := newHostWorker(host)
	worker.start(context.Background())
	if err := receiveWorkerValue(t, worker.connected, "worker connection"); err != nil {
		t.Fatalf("Connect error = %v, want nil", err)
	}

	sendWorkerRequest(t, worker, workerRequest{
		RequestID: "request-1",
		Prompt:    "first",
		Context:   context.Background(),
	})
	receiveWorkerValue(t, host.starts, "RunTurn")
	host.finish(t, runControl{err: runErr})
	result := receiveWorkerValue(t, worker.results, "worker result")
	if result.RequestID != "request-1" || !errors.Is(result.Err, runErr) {
		t.Fatalf("result = %#v, want request-1 with %v", result, runErr)
	}
	if err := stopWorker(t, worker); err != nil {
		t.Fatalf("Close error = %v, want nil", err)
	}
}
