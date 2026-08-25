package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tea4go/claude-mobile-remote/internal/channel"
	projectclaude "github.com/tea4go/claude-mobile-remote/internal/claude"
	"github.com/tea4go/claude-mobile-remote/internal/pairing"
	projectsession "github.com/tea4go/claude-mobile-remote/internal/session"
)

type submitResult struct {
	requestID string
	err       error
}

type fakeController struct {
	mu sync.Mutex

	startErr                 error
	subscribeErr             error
	stopErr                  error
	waitIdleErr              error
	snapshot                 projectsession.Snapshot
	waitSnapshot             projectsession.Snapshot
	events                   chan projectsession.Event
	results                  []submitResult
	startHook                func()
	subscribeHook            func()
	waitIdleHook             func()
	stopWaitsForSubscription bool
	stopRelease              chan struct{}

	inputs            []projectsession.Input
	calls             []string
	startCalls        int
	subscribeCalls    int
	subscribeCapacity int
	stopCalls         int
	stopContextErr    error
	waitIdleCalls     int
	subscriptionDone  <-chan struct{}

	started    chan struct{}
	subscribed chan struct{}
	stopped    chan struct{}
	submitted  chan struct{}
	startOnce  sync.Once
	subOnce    sync.Once
	stopOnce   sync.Once
}

type fakeRemoteService struct {
	mu sync.Mutex

	controller        *fakeController
	pairings          chan pairing.Payload
	activatePayload   pairing.Payload
	activateErr       error
	activateCalls     int
	calls             []string
	activated         chan struct{}
	activateOnce      sync.Once
	beginSawStopCalls int
	closeSawStopCalls int
}

type fakeTransportServer struct {
	mu            sync.Mutex
	startCalls    int
	shutdownCalls int
	closeCalls    int
}

func (f *fakeTransportServer) Start() error {
	f.mu.Lock()
	f.startCalls++
	f.mu.Unlock()
	return nil
}

func (f *fakeTransportServer) BeginShutdown() {
	f.mu.Lock()
	f.shutdownCalls++
	f.mu.Unlock()
}

func (f *fakeTransportServer) Close(context.Context) error {
	f.mu.Lock()
	f.closeCalls++
	f.mu.Unlock()
	return nil
}

func newFakeRemoteService(controller *fakeController) *fakeRemoteService {
	return &fakeRemoteService{
		controller: controller,
		pairings:   make(chan pairing.Payload, 4),
		activatePayload: pairing.Payload{
			Version: 1, WSURL: "ws://pc/ws", SessionID: "session-1",
			PairToken: "fixture-credential", ExpiresAt: time.Now().Add(time.Hour),
		},
		activated: make(chan struct{}),
	}
}

func (f *fakeRemoteService) Start() error {
	f.mu.Lock()
	f.calls = append(f.calls, "start")
	f.mu.Unlock()
	return nil
}

func (f *fakeRemoteService) Activate(sessionID string) (pairing.Payload, error) {
	f.mu.Lock()
	f.calls = append(f.calls, "activate:"+sessionID)
	f.activateCalls++
	err := f.activateErr
	f.mu.Unlock()
	f.activateOnce.Do(func() { close(f.activated) })
	if err != nil {
		return pairing.Payload{}, err
	}
	return f.activatePayload, nil
}

func (f *fakeRemoteService) activateCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.activateCalls
}

func (f *fakeRemoteService) Pairings() <-chan pairing.Payload { return f.pairings }

func (f *fakeRemoteService) BeginShutdown() {
	_, _, _, _, _, stopCalls, _ := f.controller.state()
	f.mu.Lock()
	f.calls = append(f.calls, "begin_shutdown")
	f.beginSawStopCalls = stopCalls
	f.mu.Unlock()
}

func (f *fakeRemoteService) Close(context.Context) error {
	_, _, _, _, _, stopCalls, _ := f.controller.state()
	f.mu.Lock()
	f.calls = append(f.calls, "close")
	f.closeSawStopCalls = stopCalls
	f.mu.Unlock()
	return nil
}

func (f *fakeRemoteService) state() ([]string, int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...), f.beginSawStopCalls, f.closeSawStopCalls
}

func newFakeController(results ...submitResult) *fakeController {
	return &fakeController{
		events:       make(chan projectsession.Event, 128),
		results:      results,
		waitSnapshot: projectsession.Snapshot{SessionStatus: projectsession.SessionReady},
		started:      make(chan struct{}),
		subscribed:   make(chan struct{}),
		stopped:      make(chan struct{}),
		submitted:    make(chan struct{}, 128),
		stopRelease:  make(chan struct{}),
	}
}

func TestRunWithRemoteActivatesAndStopsInOrder(t *testing.T) {
	controller := newFakeController()
	remote := newFakeRemoteService(controller)
	reader, writer := io.Pipe()
	defer reader.Close()
	var output bytes.Buffer
	var errorOutput bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- runWithRemote(context.Background(), reader, &output, &errorOutput, controller, remote)
	}()
	waitForSignal(t, controller.subscribed, "Controller subscription")
	controller.events <- projectsession.Event{Kind: projectsession.EventSessionStarted, SessionID: "session-1"}
	waitForSignal(t, remote.activated, "remote activation")
	if _, err := io.WriteString(writer, ":quit\n"); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	if exitCode := waitForRun(t, done); exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", exitCode, errorOutput.String())
	}
	_ = writer.Close()

	calls, beginSawStopCalls, closeSawStopCalls := remote.state()
	wantCalls := []string{"start", "activate:session-1", "begin_shutdown", "close"}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("remote calls = %v, want %v", calls, wantCalls)
	}
	if beginSawStopCalls != 0 || closeSawStopCalls != 1 {
		t.Fatalf("stop ordering: begin saw %d, close saw %d", beginSawStopCalls, closeSawStopCalls)
	}
	if !strings.Contains(output.String(), "pairing:") || !strings.Contains(output.String(), "session-1") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestRemoteRuntimeRotatesAndRevokesCredentials(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	random := append(bytes.Repeat([]byte{0x61}, 32), bytes.Repeat([]byte{0x62}, 32)...)
	manager, err := pairing.NewManager("ws://pc/ws", 10*time.Minute,
		pairing.WithClock(func() time.Time { return now }),
		pairing.WithRandom(bytes.NewReader(random)),
	)
	if err != nil {
		t.Fatal(err)
	}
	registry := channel.NewRegistry()
	server := &fakeTransportServer{}
	controller := newFakeController()
	ticks := make(chan time.Time, 1)
	runtime := newRemoteRuntime(manager, registry, server, controller)
	runtime.ticks = ticks
	if err := runtime.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	first, err := runtime.Activate("session-1")
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	if _, ok := registry.Get("session-1"); !ok {
		t.Fatal("Activate() did not register Session Channel")
	}
	lease, err := manager.Authenticate("session-1", first.PairToken)
	if err != nil {
		t.Fatalf("Authenticate(first) error = %v", err)
	}

	now = now.Add(11 * time.Minute)
	ticks <- now
	var second pairing.Payload
	select {
	case second = <-runtime.Pairings():
	case <-time.After(time.Second):
		t.Fatal("runtime did not publish rotated pairing payload")
	}
	if second.PairToken == first.PairToken {
		t.Fatal("runtime published the original credential after expiry")
	}
	runtime.BeginShutdown()
	if _, err := manager.Authenticate("session-1", second.PairToken); !errors.Is(err, pairing.ErrUnauthorized) {
		t.Fatalf("Authenticate(after BeginShutdown) error = %v", err)
	}
	select {
	case <-lease.Done():
		t.Fatal("BeginShutdown closed established lease before final flush")
	default:
	}
	if err := runtime.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	select {
	case <-lease.Done():
	case <-time.After(time.Second):
		t.Fatal("Close() did not revoke established lease")
	}
}

func TestRemoteRuntimeCloseForcesCleanupAfterContextCancellation(t *testing.T) {
	manager, err := pairing.NewManager(
		"ws://pc/ws",
		time.Hour,
		pairing.WithRandom(bytes.NewReader(bytes.Repeat([]byte{0x63}, 32))),
	)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := manager.Issue("session-1")
	if err != nil {
		t.Fatal(err)
	}
	lease, err := manager.Authenticate("session-1", payload.PairToken)
	if err != nil {
		t.Fatal(err)
	}
	server := &fakeTransportServer{}
	runtime := newRemoteRuntime(manager, channel.NewRegistry(), server, newFakeController())
	runtime.started = true // Simulate a started runtime whose rotation loop has not settled.

	closeContext, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runtime.Close(closeContext); !errors.Is(err, context.Canceled) {
		t.Fatalf("Close() error = %v, want context.Canceled", err)
	}
	server.mu.Lock()
	closeCalls := server.closeCalls
	server.mu.Unlock()
	if closeCalls != 1 {
		t.Fatalf("transport Close() calls = %d, want 1", closeCalls)
	}
	select {
	case <-lease.Done():
	default:
		t.Fatal("Close() did not revoke the established lease after cancellation")
	}
}

func (f *fakeController) Start(context.Context) error {
	f.mu.Lock()
	f.startCalls++
	f.calls = append(f.calls, "start")
	err := f.startErr
	hook := f.startHook
	f.mu.Unlock()
	f.startOnce.Do(func() { close(f.started) })
	if hook != nil {
		hook()
	}
	return err
}

func (f *fakeController) ReservedSessionID() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.snapshot.SessionID
}

func (f *fakeController) Submit(_ context.Context, input projectsession.Input) (string, error) {
	f.mu.Lock()
	call := len(f.inputs)
	f.inputs = append(f.inputs, input)
	f.calls = append(f.calls, "submit")
	result := submitResult{requestID: fmt.Sprintf("request-%d", call+1)}
	if call < len(f.results) {
		result = f.results[call]
	}
	f.mu.Unlock()
	f.submitted <- struct{}{}
	return result.requestID, result.err
}

func (f *fakeController) Subscribe(ctx context.Context, capacity int) (projectsession.Snapshot, <-chan projectsession.Event, error) {
	f.mu.Lock()
	f.subscribeCalls++
	f.calls = append(f.calls, "subscribe")
	f.subscribeCapacity = capacity
	f.subscriptionDone = ctx.Done()
	snapshot := f.snapshot
	events := f.events
	err := f.subscribeErr
	hook := f.subscribeHook
	f.mu.Unlock()
	f.subOnce.Do(func() { close(f.subscribed) })
	if hook != nil {
		hook()
	}
	if err != nil {
		return projectsession.Snapshot{}, nil, err
	}
	return snapshot, events, nil
}

func (f *fakeController) WaitIdle(context.Context) (projectsession.Snapshot, error) {
	f.mu.Lock()
	f.waitIdleCalls++
	f.calls = append(f.calls, "wait_idle")
	snapshot := f.waitSnapshot
	err := f.waitIdleErr
	hook := f.waitIdleHook
	f.mu.Unlock()
	if hook != nil {
		hook()
	}
	return snapshot, err
}

func (f *fakeController) Stop(ctx context.Context) error {
	f.mu.Lock()
	f.stopCalls++
	f.calls = append(f.calls, "stop")
	f.stopContextErr = ctx.Err()
	err := f.stopErr
	waitForSubscription := f.stopWaitsForSubscription
	subscriptionDone := f.subscriptionDone
	stopRelease := f.stopRelease
	f.mu.Unlock()
	f.stopOnce.Do(func() { close(f.stopped) })
	if waitForSubscription {
		select {
		case <-subscriptionDone:
		case <-stopRelease:
		}
	}
	return err
}

func (f *fakeController) state() (inputs []projectsession.Input, calls []string, startCalls, subscribeCalls, subscribeCapacity, stopCalls int, stopContextErr error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]projectsession.Input(nil), f.inputs...), append([]string(nil), f.calls...), f.startCalls, f.subscribeCalls, f.subscribeCapacity, f.stopCalls, f.stopContextErr
}

func (f *fakeController) waitIdleCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.waitIdleCalls
}

func waitForSignal(t *testing.T, signal <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}

func waitForRun(t *testing.T, done <-chan int) int {
	t.Helper()
	select {
	case exitCode := <-done:
		return exitCode
	case <-time.After(time.Second):
		t.Fatal("run did not return")
		return -1
	}
}

func waitForControllerQueueDepth(t *testing.T, controller *projectsession.Controller, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		snapshot, err := controller.Snapshot(context.Background())
		if err == nil && snapshot.QueueDepth == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for Controller queue depth %d", want)
}

func assertRunStillActive(t *testing.T, done <-chan int, duration time.Duration) {
	t.Helper()
	select {
	case exitCode := <-done:
		t.Fatalf("run returned early with exit code %d", exitCode)
	case <-time.After(duration):
	}
}

type runControlledTurn struct {
	emit   func(projectclaude.Event)
	result chan error
}

func (turn *runControlledTurn) emitEvent(event projectclaude.Event) {
	turn.emit(event)
}

func (turn *runControlledTurn) finish(err error) {
	turn.result <- err
}

type runControlledHost struct {
	started chan *runControlledTurn

	mu         sync.Mutex
	closeCalls int
}

func newRunControlledHost() *runControlledHost {
	return &runControlledHost{started: make(chan *runControlledTurn)}
}

func (host *runControlledHost) Connect(context.Context) error { return nil }

func (host *runControlledHost) RunTurn(ctx context.Context, _ string, emit func(projectclaude.Event)) error {
	turn := &runControlledTurn{emit: emit, result: make(chan error, 1)}
	select {
	case host.started <- turn:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-turn.result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (host *runControlledHost) Close() error {
	host.mu.Lock()
	host.closeCalls++
	host.mu.Unlock()
	return nil
}

func receiveRunControlledTurn(t *testing.T, host *runControlledHost) *runControlledTurn {
	t.Helper()
	select {
	case turn := <-host.started:
		return turn
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for real Controller Host turn")
		return nil
	}
}

type runScopedError struct {
	scope projectclaude.ErrorScope
	err   error
}

func (err runScopedError) Error() string                   { return err.err.Error() }
func (err runScopedError) Unwrap() error                   { return err.err }
func (err runScopedError) Scope() projectclaude.ErrorScope { return err.scope }

func TestRunSubmitsLinesWithoutWaitingForTurn(t *testing.T) {
	controller := newFakeController(
		submitResult{requestID: "request-1"},
		submitResult{requestID: "request-2"},
	)
	reader, writer := io.Pipe()
	defer reader.Close()
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- run(context.Background(), reader, &stdout, &stderr, controller)
	}()

	if _, err := io.WriteString(writer, "  first  \nsecond\n"); err != nil {
		t.Fatal(err)
	}
	waitForSignal(t, controller.submitted, "first Submit")
	waitForSignal(t, controller.submitted, "second Submit before a terminal event")
	inputs, calls, startCalls, subscribeCalls, subscribeCapacity, _, _ := controller.state()
	if !reflect.DeepEqual(inputs, []projectsession.Input{
		{Source: projectsession.InputTerminal, Text: "first"},
		{Source: projectsession.InputTerminal, Text: "second"},
	}) {
		t.Fatalf("inputs = %#v", inputs)
	}
	if startCalls != 1 || subscribeCalls != 1 || subscribeCapacity != 64 || !reflect.DeepEqual(calls, []string{"start", "subscribe", "submit", "submit"}) {
		t.Fatalf("calls = %#v, start/subscribe/capacity = %d/%d/%d", calls, startCalls, subscribeCalls, subscribeCapacity)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	controller.events <- projectsession.Event{Kind: projectsession.EventSessionStarted, SessionID: "session-1"}
	controller.events <- projectsession.Event{Kind: projectsession.EventTextDelta, RequestID: "request-1", Text: "one"}
	controller.events <- projectsession.Event{Kind: projectsession.EventTurnCompleted, RequestID: "request-1"}
	assertRunStillActive(t, done, 20*time.Millisecond)
	controller.events <- projectsession.Event{Kind: projectsession.EventTextDelta, RequestID: "request-2", Text: "two"}
	controller.events <- projectsession.Event{Kind: projectsession.EventTurnCompleted, RequestID: "request-2"}

	if exitCode := waitForRun(t, done); exitCode != 0 || stderr.Len() != 0 {
		t.Fatalf("exitCode = %d, stderr = %q", exitCode, stderr.String())
	}
	if got := stdout.String(); got != "session: session-1\none\ntwo\n" {
		t.Fatalf("stdout = %q", got)
	}
	_, _, _, _, _, stopCalls, stopContextErr := controller.state()
	if stopCalls != 1 || stopContextErr != nil {
		t.Fatalf("stopCalls = %d, stop context error = %v", stopCalls, stopContextErr)
	}
}

func TestRunEOFWaitsForAcceptedRequests(t *testing.T) {
	controller := newFakeController(submitResult{requestID: "request-1"})
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- run(context.Background(), strings.NewReader("first\n"), &stdout, &stderr, controller)
	}()

	waitForSignal(t, controller.submitted, "accepted request")
	assertRunStillActive(t, done, 50*time.Millisecond)
	controller.events <- projectsession.Event{Kind: projectsession.EventTurnCompleted, RequestID: "request-1"}

	if exitCode := waitForRun(t, done); exitCode != 0 || stderr.Len() != 0 {
		t.Fatalf("exitCode = %d, stderr = %q", exitCode, stderr.String())
	}
	if stdout.String() != "\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	_, _, _, _, _, stopCalls, _ := controller.state()
	if stopCalls != 1 {
		t.Fatalf("stopCalls = %d", stopCalls)
	}
	if waitIdleCalls := controller.waitIdleCallCount(); waitIdleCalls != 1 {
		t.Fatalf("WaitIdle calls = %d, want 1", waitIdleCalls)
	}
}

func TestRunEOFWaitsForControllerSettlement(t *testing.T) {
	tests := []struct {
		name         string
		resultErr    error
		wantExitCode int
		wantStderr   string
	}{
		{
			name:         "Session result reports only Session root cause",
			resultErr:    runScopedError{scope: projectclaude.ScopeSession, err: errors.New("session transport failed")},
			wantExitCode: 1,
			wantStderr:   "Session 错误: session transport failed\n",
		},
		{
			name:         "recoverable Turn result reports Turn failure",
			resultErr:    runScopedError{scope: projectclaude.ScopeTurn, err: errors.New("recoverable Worker failure")},
			wantExitCode: 0,
			wantStderr:   "Turn 错误: recoverable Adapter failure\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			host := newRunControlledHost()
			controller := projectsession.NewController(host)
			var stdout, stderr bytes.Buffer
			done := make(chan int, 1)
			go func() {
				done <- run(context.Background(), strings.NewReader("first\n"), &stdout, &stderr, controller)
			}()

			turn := receiveRunControlledTurn(t, host)
			turn.emitEvent(projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-1"})
			turn.emitEvent(projectclaude.Event{Kind: projectclaude.EventTurnFailed, Text: "recoverable Adapter failure"})
			assertRunStillActive(t, done, 50*time.Millisecond)
			turn.finish(test.resultErr)

			if exitCode := waitForRun(t, done); exitCode != test.wantExitCode {
				t.Fatalf("exitCode = %d, stderr = %q", exitCode, stderr.String())
			}
			if got := stderr.String(); got != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", got, test.wantStderr)
			}
			if got := stdout.String(); got != "session: session-1\n" {
				t.Fatalf("stdout = %q, want Session announcement only", got)
			}
		})
	}
}

func TestRunEOFSessionFailureTerminatesPartialLine(t *testing.T) {
	host := newRunControlledHost()
	controller := projectsession.NewController(host)
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- run(context.Background(), strings.NewReader("first\n"), &stdout, &stderr, controller)
	}()

	turn := receiveRunControlledTurn(t, host)
	turn.emitEvent(projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-1"})
	turn.emitEvent(projectclaude.Event{Kind: projectclaude.EventText, Text: "partial"})
	turn.emitEvent(projectclaude.Event{Kind: projectclaude.EventTurnFailed, Text: "Adapter failure"})
	assertRunStillActive(t, done, 50*time.Millisecond)
	turn.finish(runScopedError{scope: projectclaude.ScopeSession, err: errors.New("session transport failed")})

	if exitCode := waitForRun(t, done); exitCode != 1 {
		t.Fatalf("exitCode = %d, stderr = %q", exitCode, stderr.String())
	}
	if got, want := stdout.String(), "session: session-1\npartial\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if got, want := stderr.String(), "Session 错误: session transport failed\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestRunEOFSessionFailureSuppressesTurnErrorWithQueuedRequest(t *testing.T) {
	host := newRunControlledHost()
	controller := projectsession.NewController(host)
	var stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- run(context.Background(), strings.NewReader("first\nsecond\n"), io.Discard, &stderr, controller)
	}()

	turn := receiveRunControlledTurn(t, host)
	waitForControllerQueueDepth(t, controller, 1)
	turn.emitEvent(projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-1"})
	turn.emitEvent(projectclaude.Event{Kind: projectclaude.EventTurnFailed, Text: "Adapter failure"})
	assertRunStillActive(t, done, 50*time.Millisecond)
	turn.finish(runScopedError{scope: projectclaude.ScopeSession, err: errors.New("session transport failed")})

	if exitCode := waitForRun(t, done); exitCode != 1 {
		t.Fatalf("exitCode = %d, stderr = %q", exitCode, stderr.String())
	}
	if got, want := stderr.String(), "Session 错误: session transport failed\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	select {
	case <-host.started:
		t.Fatal("queued Turn unexpectedly started")
	default:
	}
}

func TestRunSessionFailureSuppressesTurnErrorBeforeEOFIsObserved(t *testing.T) {
	host := newRunControlledHost()
	controller := projectsession.NewController(host)
	reader, writer := io.Pipe()
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})
	var stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- run(context.Background(), reader, io.Discard, &stderr, controller)
	}()

	if _, err := io.WriteString(writer, "first\nsecond\n"); err != nil {
		t.Fatal(err)
	}
	turn := receiveRunControlledTurn(t, host)
	waitForControllerQueueDepth(t, controller, 1)
	turn.emitEvent(projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-1"})
	turn.emitEvent(projectclaude.Event{Kind: projectclaude.EventTurnFailed, Text: "Adapter failure"})
	turn.finish(runScopedError{scope: projectclaude.ScopeSession, err: errors.New("session transport failed")})

	if exitCode := waitForRun(t, done); exitCode != 1 {
		t.Fatalf("exitCode = %d, stderr = %q", exitCode, stderr.String())
	}
	if got, want := stderr.String(), "Session 错误: session transport failed\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestRunDirectSessionFailureTerminatesPartialLine(t *testing.T) {
	controller := newFakeController(submitResult{requestID: "request-1"})
	reader, writer := io.Pipe()
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- run(context.Background(), reader, &stdout, &stderr, controller)
	}()

	if _, err := io.WriteString(writer, "first\n"); err != nil {
		t.Fatal(err)
	}
	waitForSignal(t, controller.submitted, "accepted request")
	controller.events <- projectsession.Event{Kind: projectsession.EventTextDelta, RequestID: "request-1", Text: "partial"}
	controller.events <- projectsession.Event{
		Kind:  projectsession.EventSessionFailed,
		Error: &projectsession.ErrorInfo{Message: "session failed"},
	}

	if exitCode := waitForRun(t, done); exitCode != 1 {
		t.Fatalf("exitCode = %d, stderr = %q", exitCode, stderr.String())
	}
	if got, want := stdout.String(), "partial\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRunQuitStopsImmediately(t *testing.T) {
	controller := newFakeController()
	var stdout, stderr bytes.Buffer
	exitCode := run(context.Background(), strings.NewReader(":quit\n"), &stdout, &stderr, controller)

	inputs, _, _, _, _, stopCalls, _ := controller.state()
	if exitCode != 0 || stderr.Len() != 0 || len(inputs) != 0 || stopCalls != 1 {
		t.Fatalf("exitCode = %d, inputs = %#v, stopCalls = %d, stderr = %q", exitCode, inputs, stopCalls, stderr.String())
	}
}

func TestRunCancelsSubscriptionBeforeStop(t *testing.T) {
	tests := []struct {
		name         string
		input        io.Reader
		wantExitCode int
	}{
		{name: "quit", input: strings.NewReader(":quit\n"), wantExitCode: 0},
		{name: "scanner error", input: failingReader{err: errors.New("read failed")}, wantExitCode: 1},
		{name: "EOF", input: strings.NewReader(""), wantExitCode: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			controller := newFakeController()
			controller.stopWaitsForSubscription = true
			t.Cleanup(func() { close(controller.stopRelease) })
			done := make(chan int, 1)
			go func() {
				done <- run(context.Background(), test.input, io.Discard, io.Discard, controller)
			}()

			if exitCode := waitForRun(t, done); exitCode != test.wantExitCode {
				t.Fatalf("exitCode = %d, want %d", exitCode, test.wantExitCode)
			}
			_, _, _, _, _, stopCalls, _ := controller.state()
			if stopCalls != 1 {
				t.Fatalf("stopCalls = %d, want 1", stopCalls)
			}
		})
	}
}

func TestRunReportsSubmitRejectionAndContinues(t *testing.T) {
	controller := newFakeController(
		submitResult{err: fmt.Errorf("%w: queue at capacity", projectsession.ErrQueueFull)},
		submitResult{requestID: "request-2"},
	)
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- run(context.Background(), strings.NewReader("first\nsecond\n"), &stdout, &stderr, controller)
	}()

	waitForSignal(t, controller.submitted, "rejected Submit")
	waitForSignal(t, controller.submitted, "Submit after rejection")
	controller.events <- projectsession.Event{Kind: projectsession.EventTurnCompleted, RequestID: "request-2"}

	if exitCode := waitForRun(t, done); exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %q", exitCode, stderr.String())
	}
	if got := stderr.String(); !strings.Contains(got, "输入拒绝:") || !strings.Contains(got, "queue at capacity") {
		t.Fatalf("stderr = %q", got)
	}
	inputs, _, _, _, _, _, _ := controller.state()
	if !reflect.DeepEqual(inputs, []projectsession.Input{
		{Source: projectsession.InputTerminal, Text: "first"},
		{Source: projectsession.InputTerminal, Text: "second"},
	}) {
		t.Fatalf("inputs = %#v", inputs)
	}
}

func TestRunSessionUnavailableSubmitExitsOne(t *testing.T) {
	controller := newFakeController(
		submitResult{err: fmt.Errorf("%w: controller stopped", projectsession.ErrSessionUnavailable)},
		submitResult{requestID: "request-2"},
	)
	var stderr bytes.Buffer
	exitCode := run(context.Background(), strings.NewReader("first\nsecond\n"), io.Discard, &stderr, controller)

	if exitCode != 1 {
		t.Fatalf("exitCode = %d, stderr = %q", exitCode, stderr.String())
	}
	if got := stderr.String(); !strings.Contains(got, "输入拒绝:") || !strings.Contains(got, "controller stopped") {
		t.Fatalf("stderr = %q", got)
	}
	inputs, _, _, _, _, stopCalls, _ := controller.state()
	if len(inputs) != 1 || inputs[0].Text != "first" || stopCalls != 1 {
		t.Fatalf("inputs = %#v, stopCalls = %d", inputs, stopCalls)
	}
}

func TestRunTurnFailureContinues(t *testing.T) {
	controller := newFakeController(
		submitResult{requestID: "request-1"},
		submitResult{requestID: "request-2"},
	)
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- run(context.Background(), strings.NewReader("first\nsecond\n"), &stdout, &stderr, controller)
	}()

	waitForSignal(t, controller.submitted, "first Submit")
	waitForSignal(t, controller.submitted, "second Submit")
	controller.events <- projectsession.Event{Kind: projectsession.EventTextDelta, RequestID: "request-1", Text: "partial"}
	controller.events <- projectsession.Event{
		Kind:      projectsession.EventTurnFailed,
		RequestID: "request-1",
		Error:     &projectsession.ErrorInfo{Code: "turn_failed", Message: "recoverable"},
	}
	assertRunStillActive(t, done, 20*time.Millisecond)
	controller.events <- projectsession.Event{Kind: projectsession.EventTextDelta, RequestID: "request-2", Text: "ok"}
	controller.events <- projectsession.Event{Kind: projectsession.EventTurnCompleted, RequestID: "request-2"}

	if exitCode := waitForRun(t, done); exitCode != 0 {
		t.Fatalf("exitCode = %d, stderr = %q", exitCode, stderr.String())
	}
	if stdout.String() != "partial\nok\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if got := stderr.String(); !strings.Contains(got, "Turn 错误:") || !strings.Contains(got, "recoverable") {
		t.Fatalf("stderr = %q", got)
	}
}

func TestRunSessionFailureExitsOne(t *testing.T) {
	controller := newFakeController(submitResult{requestID: "request-1"})
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- run(context.Background(), strings.NewReader("first\n"), &stdout, &stderr, controller)
	}()

	waitForSignal(t, controller.submitted, "accepted request")
	controller.events <- projectsession.Event{
		Kind:  projectsession.EventSessionFailed,
		Error: &projectsession.ErrorInfo{Code: "session_failed", Message: "stream unavailable"},
	}

	if exitCode := waitForRun(t, done); exitCode != 1 {
		t.Fatalf("exitCode = %d, stderr = %q", exitCode, stderr.String())
	}
	if got := stderr.String(); !strings.Contains(got, "Session 错误:") || !strings.Contains(got, "stream unavailable") {
		t.Fatalf("stderr = %q", got)
	}
}

type blockingReadCloser struct {
	closed       chan struct{}
	readStarted  chan struct{}
	readReturned chan struct{}
	closeOnce    sync.Once
	startOnce    sync.Once
	readOnce     sync.Once
}

func newBlockingReadCloser() *blockingReadCloser {
	return &blockingReadCloser{
		closed:       make(chan struct{}),
		readStarted:  make(chan struct{}),
		readReturned: make(chan struct{}),
	}
}

func (r *blockingReadCloser) Read([]byte) (int, error) {
	r.startOnce.Do(func() { close(r.readStarted) })
	<-r.closed
	r.readOnce.Do(func() { close(r.readReturned) })
	return 0, io.EOF
}

func (r *blockingReadCloser) Close() error {
	r.closeOnce.Do(func() { close(r.closed) })
	return nil
}

func TestRunCancellationClosesBlockingReader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reader := newBlockingReadCloser()
	controller := newFakeController()
	done := make(chan int, 1)
	go func() {
		done <- run(ctx, reader, io.Discard, io.Discard, controller)
	}()

	waitForSignal(t, controller.subscribed, "subscription")
	waitForSignal(t, reader.readStarted, "blocking scanner start")
	cancel()
	if exitCode := waitForRun(t, done); exitCode != 0 {
		t.Fatalf("exitCode = %d", exitCode)
	}
	waitForSignal(t, reader.closed, "reader close")
	waitForSignal(t, reader.readReturned, "blocking scanner return")
	_, _, _, _, _, stopCalls, stopContextErr := controller.state()
	if stopCalls != 1 || stopContextErr != nil {
		t.Fatalf("stopCalls = %d, stop context error = %v", stopCalls, stopContextErr)
	}
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

type cancelingErrorReader struct {
	cancel context.CancelFunc
	err    error
}

func (r cancelingErrorReader) Read([]byte) (int, error) {
	r.cancel()
	return 0, r.err
}

func TestRunCancellationWinsErrorRaces(t *testing.T) {
	t.Run("Start error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		controller := newFakeController()
		controller.startErr = errors.New("start failed during cancellation")
		controller.startHook = cancel
		var stderr bytes.Buffer
		if exitCode := run(ctx, strings.NewReader(""), io.Discard, &stderr, controller); exitCode != 0 || stderr.Len() != 0 {
			t.Fatalf("exitCode = %d, stderr = %q", exitCode, stderr.String())
		}
	})

	t.Run("Subscribe error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		controller := newFakeController()
		controller.subscribeErr = errors.New("subscribe failed during cancellation")
		controller.subscribeHook = cancel
		var stderr bytes.Buffer
		if exitCode := run(ctx, strings.NewReader(""), io.Discard, &stderr, controller); exitCode != 0 || stderr.Len() != 0 {
			t.Fatalf("exitCode = %d, stderr = %q", exitCode, stderr.String())
		}
	})

	t.Run("scanner error", func(t *testing.T) {
		for attempt := 0; attempt < 25; attempt++ {
			ctx, cancel := context.WithCancel(context.Background())
			controller := newFakeController()
			var stderr bytes.Buffer
			exitCode := run(ctx, cancelingErrorReader{cancel: cancel, err: errors.New("read failed during cancellation")}, io.Discard, &stderr, controller)
			if exitCode != 0 || stderr.Len() != 0 {
				t.Fatalf("attempt %d: exitCode = %d, stderr = %q", attempt, exitCode, stderr.String())
			}
		}
	})

	t.Run("closed event stream with pending request", func(t *testing.T) {
		for attempt := 0; attempt < 25; attempt++ {
			ctx, cancel := context.WithCancel(context.Background())
			controller := newFakeController(submitResult{requestID: "request-1"})
			var stderr bytes.Buffer
			done := make(chan int, 1)
			go func() {
				done <- run(ctx, strings.NewReader("first\n"), io.Discard, &stderr, controller)
			}()
			waitForSignal(t, controller.submitted, "pending request before cancellation")
			cancel()
			close(controller.events)
			if exitCode := waitForRun(t, done); exitCode != 0 || stderr.Len() != 0 {
				t.Fatalf("attempt %d: exitCode = %d, stderr = %q", attempt, exitCode, stderr.String())
			}
		}
	})

	t.Run("Session failure event", func(t *testing.T) {
		for attempt := 0; attempt < 25; attempt++ {
			ctx, cancel := context.WithCancel(context.Background())
			controller := newFakeController(submitResult{requestID: "request-1"})
			var stderr bytes.Buffer
			done := make(chan int, 1)
			go func() {
				done <- run(ctx, strings.NewReader("first\n"), io.Discard, &stderr, controller)
			}()
			waitForSignal(t, controller.submitted, "pending request before Session failure")
			cancel()
			controller.events <- projectsession.Event{
				Kind:  projectsession.EventSessionFailed,
				Error: &projectsession.ErrorInfo{Message: "failure during cancellation"},
			}
			if exitCode := waitForRun(t, done); exitCode != 0 || stderr.Len() != 0 {
				t.Fatalf("attempt %d: exitCode = %d, stderr = %q", attempt, exitCode, stderr.String())
			}
		}
	})

	t.Run("WaitIdle error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		controller := newFakeController()
		controller.waitIdleErr = errors.New("WaitIdle failed during cancellation")
		controller.waitIdleHook = cancel
		var stderr bytes.Buffer
		if exitCode := run(ctx, strings.NewReader(""), io.Discard, &stderr, controller); exitCode != 0 || stderr.Len() != 0 {
			t.Fatalf("exitCode = %d, stderr = %q", exitCode, stderr.String())
		}
	})
}

func TestRunReportsStartSubscribeScannerAndStopErrors(t *testing.T) {
	tests := []struct {
		name           string
		input          io.Reader
		configure      func(*fakeController)
		wantPrefixes   []string
		wantStartCalls int
		wantSubscribe  int
		wantStopCalls  int
	}{
		{
			name:  "start error",
			input: strings.NewReader(""),
			configure: func(controller *fakeController) {
				controller.startErr = errors.New("start failed")
			},
			wantPrefixes:   []string{"启动错误: start failed"},
			wantStartCalls: 1,
			wantStopCalls:  1,
		},
		{
			name:  "subscribe error",
			input: strings.NewReader(""),
			configure: func(controller *fakeController) {
				controller.subscribeErr = errors.New("subscribe failed")
			},
			wantPrefixes:   []string{"订阅错误: subscribe failed"},
			wantStartCalls: 1,
			wantSubscribe:  1,
			wantStopCalls:  1,
		},
		{
			name:           "scanner error",
			input:          failingReader{err: errors.New("read failed")},
			configure:      func(*fakeController) {},
			wantPrefixes:   []string{"读取输入错误: read failed"},
			wantStartCalls: 1,
			wantSubscribe:  1,
			wantStopCalls:  1,
		},
		{
			name:  "WaitIdle error",
			input: strings.NewReader(""),
			configure: func(controller *fakeController) {
				controller.waitIdleErr = errors.New("settlement failed")
			},
			wantPrefixes:   []string{"结算错误: settlement failed"},
			wantStartCalls: 1,
			wantSubscribe:  1,
			wantStopCalls:  1,
		},
		{
			name:  "stop error after success",
			input: strings.NewReader(""),
			configure: func(controller *fakeController) {
				controller.stopErr = errors.New("stop failed")
			},
			wantPrefixes:   []string{"关闭错误: stop failed"},
			wantStartCalls: 1,
			wantSubscribe:  1,
			wantStopCalls:  1,
		},
		{
			name:  "stop error after primary failure",
			input: strings.NewReader(""),
			configure: func(controller *fakeController) {
				controller.startErr = errors.New("primary failure")
				controller.stopErr = errors.New("stop failed")
			},
			wantPrefixes:   []string{"启动错误: primary failure", "关闭错误: stop failed"},
			wantStartCalls: 1,
			wantStopCalls:  1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			controller := newFakeController()
			test.configure(controller)
			var stdout, stderr bytes.Buffer
			exitCode := run(context.Background(), test.input, &stdout, &stderr, controller)

			if exitCode != 1 {
				t.Fatalf("exitCode = %d, stderr = %q", exitCode, stderr.String())
			}
			for _, want := range test.wantPrefixes {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("stderr = %q, want %q", stderr.String(), want)
				}
			}
			_, _, startCalls, subscribeCalls, _, stopCalls, stopContextErr := controller.state()
			if startCalls != test.wantStartCalls || subscribeCalls != test.wantSubscribe || stopCalls != test.wantStopCalls {
				t.Fatalf("start/subscribe/stop calls = %d/%d/%d", startCalls, subscribeCalls, stopCalls)
			}
			if stopContextErr != nil {
				t.Fatalf("stop context error = %v", stopContextErr)
			}
		})
	}
}

func TestRunAcceptsPromptLargerThanScannerDefault(t *testing.T) {
	t.Run("70 KiB is submitted", func(t *testing.T) {
		prompt := strings.Repeat("x", 70*1024)
		controller := newFakeController(submitResult{requestID: "request-1"})
		done := make(chan int, 1)
		go func() {
			done <- run(context.Background(), strings.NewReader(prompt+"\n"), io.Discard, io.Discard, controller)
		}()

		waitForSignal(t, controller.submitted, "large prompt Submit")
		inputs, _, _, _, _, _, _ := controller.state()
		if len(inputs) != 1 || inputs[0].Text != prompt {
			t.Fatalf("submitted prompt count/length = %d/%d", len(inputs), len(inputs[0].Text))
		}
		controller.events <- projectsession.Event{Kind: projectsession.EventTurnCompleted, RequestID: "request-1"}
		if exitCode := waitForRun(t, done); exitCode != 0 {
			t.Fatalf("exitCode = %d", exitCode)
		}
	})

	for _, test := range []struct {
		name       string
		terminator string
	}{
		{name: "exactly 1 MiB with LF is submitted", terminator: "\n"},
		{name: "exactly 1 MiB with CRLF is submitted", terminator: "\r\n"},
		{name: "exactly 1 MiB without newline is submitted", terminator: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			prompt := strings.Repeat("x", projectsession.MaxPromptBytes)
			controller := newFakeController(submitResult{requestID: "request-1"})
			var stderr bytes.Buffer
			done := make(chan int, 1)
			go func() {
				done <- run(context.Background(), strings.NewReader(prompt+test.terminator), io.Discard, &stderr, controller)
			}()

			waitForSignal(t, controller.submitted, "1 MiB prompt Submit")
			inputs, _, _, _, _, _, _ := controller.state()
			if len(inputs) != 1 || inputs[0].Text != prompt {
				t.Fatalf("submitted prompt count/length = %d/%d", len(inputs), len(inputs[0].Text))
			}
			controller.events <- projectsession.Event{Kind: projectsession.EventTurnCompleted, RequestID: "request-1"}
			if exitCode := waitForRun(t, done); exitCode != 0 || stderr.Len() != 0 {
				t.Fatalf("exitCode = %d, stderr = %q", exitCode, stderr.String())
			}
		})
	}

	t.Run("over 1 MiB reports scanner error", func(t *testing.T) {
		prompt := strings.Repeat("x", projectsession.MaxPromptBytes+1)
		controller := newFakeController()
		var stderr bytes.Buffer
		exitCode := run(context.Background(), strings.NewReader(prompt+"\n"), io.Discard, &stderr, controller)

		inputs, _, _, _, _, stopCalls, _ := controller.state()
		if exitCode != 1 || len(inputs) != 0 || stopCalls != 1 {
			t.Fatalf("exitCode = %d, inputs = %d, stopCalls = %d", exitCode, len(inputs), stopCalls)
		}
		if got := stderr.String(); !strings.Contains(got, "读取输入错误:") || len(got) > 1024 {
			t.Fatalf("stderr length/content = %d/%q", len(got), got)
		}
	})
}

func TestRunReportsUnexpectedEventClosureWithPendingRequest(t *testing.T) {
	controller := newFakeController(submitResult{requestID: "request-1"})
	var stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- run(context.Background(), strings.NewReader("first\n"), io.Discard, &stderr, controller)
	}()

	waitForSignal(t, controller.submitted, "accepted request")
	close(controller.events)
	if exitCode := waitForRun(t, done); exitCode != 1 {
		t.Fatalf("exitCode = %d, stderr = %q", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "事件流") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

// ---- A5.2 首轮前配对（设计 §8）----

// syncBuffer 是并发安全的输出缓冲。首轮前配对的断言需要在 run goroutine 仍在
// 写入时读取输出，普通 bytes.Buffer 会被 race 检测判定为竞争。
type syncBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (s *syncBuffer) Write(data []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buffer.Write(data)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buffer.String()
}

// waitForOutput 轮询等待 output 出现 marker，超时则失败。
func waitForOutput(t *testing.T, output *syncBuffer, marker string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(output.String(), marker) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("output = %q, want %q", output.String(), marker)
}

func TestRunWithRemoteActivatesBeforeFirstInput(t *testing.T) {
	controller := newFakeController()
	controller.snapshot = projectsession.Snapshot{
		SessionID:     "session-reserved",
		SessionStatus: projectsession.SessionStarting,
		Turn:          projectsession.TurnSnapshot{Status: projectsession.TurnIdle},
	}
	remote := newFakeRemoteService(controller)
	reader, writer := io.Pipe()
	defer reader.Close()
	output := &syncBuffer{}
	errorOutput := &syncBuffer{}
	done := make(chan int, 1)
	go func() {
		done <- runWithRemote(context.Background(), reader, output, errorOutput, controller, remote)
	}()

	// 无任何终端输入：Subscribe 后立即以预留 ID 激活并输出配对（设计 §8）。
	waitForOutput(t, output, "pairing:")
	if activateCalls := remote.activateCallCount(); activateCalls != 1 {
		t.Fatalf("Activate calls before first input = %d, want 1", activateCalls)
	}

	// session_started 只更新可见状态，不再触发第二次激活。
	controller.events <- projectsession.Event{Kind: projectsession.EventSessionStarted, SessionID: "session-reserved"}
	waitForOutput(t, output, "session: session-reserved")
	if activateCalls := remote.activateCallCount(); activateCalls != 1 {
		t.Fatalf("Activate calls after session_started = %d, want exactly 1 (pre-turn)", activateCalls)
	}

	if _, err := io.WriteString(writer, ":quit\n"); err != nil {
		t.Fatal(err)
	}
	if exitCode := waitForRun(t, done); exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", exitCode, errorOutput.String())
	}
	_ = writer.Close()
}

func TestRunWithRemoteActivationFailureCleansUp(t *testing.T) {
	controller := newFakeController()
	controller.snapshot = projectsession.Snapshot{
		SessionID:     "session-reserved",
		SessionStatus: projectsession.SessionStarting,
		Turn:          projectsession.TurnSnapshot{Status: projectsession.TurnIdle},
	}
	remote := newFakeRemoteService(controller)
	remote.activateErr = errors.New("channel register failed")
	reader, writer := io.Pipe()
	defer reader.Close()
	var output bytes.Buffer
	var errorOutput bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- runWithRemote(context.Background(), reader, &output, &errorOutput, controller, remote)
	}()

	_ = writer
	if exitCode := waitForRun(t, done); exitCode != 1 {
		t.Fatalf("exit code = %d, want 1 on activation failure", exitCode)
	}
	if !strings.Contains(errorOutput.String(), "配对激活错误") {
		t.Fatalf("stderr = %q, want activation failure", errorOutput.String())
	}
	_, _, _, _, _, stopCalls, _ := controller.state()
	if stopCalls != 1 {
		t.Fatalf("Stop calls = %d, want 1 (cleanup on activation failure)", stopCalls)
	}
}

func TestParseConfigResumeSessionID(t *testing.T) {
	config, err := parseConfig([]string{"--resume", "0f0d7a5a-33b5-4f0a-9d3e-6f2b8cf1a111"})
	if err != nil {
		t.Fatalf("parseConfig(--resume) error = %v", err)
	}
	if config.resumeSessionID != "0f0d7a5a-33b5-4f0a-9d3e-6f2b8cf1a111" {
		t.Fatalf("resumeSessionID = %q", config.resumeSessionID)
	}
	if _, err := parseConfig([]string{"--resume", "not-a-uuid"}); err == nil {
		t.Fatal("parseConfig accepted invalid resume UUID")
	}
}

// TestBuildRuntimeReservesSessionIDBeforeConnect 验证启动顺序的第一步（设计
// §5）：新会话在创建 Host 前生成 UUID，resume 沿用已有 ID；两者都在
// Connect 前就把真实 ID 预留给 Controller，供首轮前配对使用。
func TestBuildRuntimeReservesSessionIDBeforeConnect(t *testing.T) {
	baseConfig, err := parseConfig(nil)
	if err != nil {
		t.Fatalf("parseConfig(nil) error = %v", err)
	}

	newSessionController, _, err := buildRuntime(baseConfig)
	if err != nil {
		t.Fatalf("buildRuntime(new session) error = %v", err)
	}
	generated := newSessionController.ReservedSessionID()
	if len(generated) != 36 || generated[14] != '4' {
		t.Fatalf("new session reserved ID = %q, want UUID v4", generated)
	}

	resumeConfig := baseConfig
	resumeConfig.resumeSessionID = "0f0d7a5a-33b5-4f0a-9d3e-6f2b8cf1a111"
	resumeController, _, err := buildRuntime(resumeConfig)
	if err != nil {
		t.Fatalf("buildRuntime(resume) error = %v", err)
	}
	if reserved := resumeController.ReservedSessionID(); reserved != resumeConfig.resumeSessionID {
		t.Fatalf("resume reserved ID = %q, want %q", reserved, resumeConfig.resumeSessionID)
	}
}
