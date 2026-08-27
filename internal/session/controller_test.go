package session

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	projectclaude "github.com/tea4go/claude-mobile-remote/internal/claude"
)

const controllerTestTimeout = 2 * time.Second

type controlledTurn struct {
	prompt        string
	events        chan projectclaude.Event
	result        chan error
	cancelled     chan struct{}
	delivered     chan struct{}
	cancelledOnce sync.Once
}

func newControlledTurn(prompt string) *controlledTurn {
	return &controlledTurn{
		prompt:    prompt,
		events:    make(chan projectclaude.Event),
		result:    make(chan error, 1),
		cancelled: make(chan struct{}),
		delivered: make(chan struct{}, 64),
	}
}

func (turn *controlledTurn) observeCancellation() {
	turn.cancelledOnce.Do(func() {
		close(turn.cancelled)
	})
}

func (turn *controlledTurn) emit(t *testing.T, event projectclaude.Event) {
	t.Helper()
	select {
	case turn.events <- event:
	case <-time.After(controllerTestTimeout):
		t.Fatal("timed out emitting controlled Adapter event")
	}
}

func (turn *controlledTurn) emitAndWaitForDelivery(t *testing.T, event projectclaude.Event) {
	t.Helper()
	turn.emit(t, event)
	select {
	case <-turn.delivered:
	case <-time.After(controllerTestTimeout):
		t.Fatal("Host did not deliver controlled Adapter event to Controller")
	}
}

func (turn *controlledTurn) finish(t *testing.T, err error) {
	t.Helper()
	select {
	case turn.result <- err:
	case <-time.After(controllerTestTimeout):
		t.Fatal("timed out finishing controlled turn")
	}
}

type controlledHost struct {
	connectErr     error
	closeErr       error
	interruptErr   error
	started        chan *controlledTurn
	connectEntered chan struct{}
	connectRelease <-chan struct{}
	closeEntered   chan struct{}
	closeRelease   <-chan struct{}
	// actions is the unbuffered side channel the Adapter uses to hand pending
	// interactions to the Controller, mirroring B2 §3.4.
	actions         chan *projectclaude.ActionRequest
	interruptCalled chan struct{}

	mu                   sync.Mutex
	active               int
	maxActive            int
	closeCalls           int
	interrupts           int
	closeEnteredOnce     sync.Once
	interruptEnteredOnce sync.Once
}

func newControlledHost() *controlledHost {
	return &controlledHost{
		started:         make(chan *controlledTurn),
		actions:         make(chan *projectclaude.ActionRequest),
		interruptCalled: make(chan struct{}),
	}
}

func (host *controlledHost) Connect(ctx context.Context) error {
	if host.connectEntered != nil {
		close(host.connectEntered)
	}
	if host.connectRelease != nil {
		select {
		case <-host.connectRelease:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return host.connectErr
}

func (host *controlledHost) RunTurn(ctx context.Context, prompt string, emit func(projectclaude.Event)) error {
	host.mu.Lock()
	host.active++
	if host.active > host.maxActive {
		host.maxActive = host.active
	}
	host.mu.Unlock()
	defer func() {
		host.mu.Lock()
		host.active--
		host.mu.Unlock()
	}()

	turn := newControlledTurn(prompt)
	select {
	case host.started <- turn:
	case <-ctx.Done():
		turn.observeCancellation()
		return ctx.Err()
	}

	for {
		select {
		case event := <-turn.events:
			emit(event)
			turn.delivered <- struct{}{}
		case err := <-turn.result:
			return err
		case <-ctx.Done():
			turn.observeCancellation()
			return ctx.Err()
		}
	}
}

func (host *controlledHost) Close() error {
	host.mu.Lock()
	host.closeCalls++
	host.mu.Unlock()
	if host.closeEntered != nil {
		host.closeEnteredOnce.Do(func() {
			close(host.closeEntered)
		})
	}
	if host.closeRelease != nil {
		<-host.closeRelease
	}
	return host.closeErr
}

func (host *controlledHost) Actions() <-chan *projectclaude.ActionRequest {
	return host.actions
}

func (host *controlledHost) Interrupt(context.Context) error {
	host.mu.Lock()
	host.interrupts++
	host.mu.Unlock()
	host.interruptEnteredOnce.Do(func() {
		close(host.interruptCalled)
	})
	return host.interruptErr
}

func (host *controlledHost) counts() (maxActive, closeCalls int) {
	host.mu.Lock()
	defer host.mu.Unlock()
	return host.maxActive, host.closeCalls
}

func (host *controlledHost) interruptCount() int {
	host.mu.Lock()
	defer host.mu.Unlock()
	return host.interrupts
}

// requestAction hands one pending interaction to the Controller the same way a
// blocked permission callback would, then returns the request so the test can
// observe the decision that travels back down the side channel.
func (host *controlledHost) requestAction(t *testing.T, spec projectclaude.ActionSpec) *projectclaude.ActionRequest {
	t.Helper()
	request := projectclaude.NewActionRequest(spec)
	select {
	case host.actions <- request:
		return request
	case <-time.After(controllerTestTimeout):
		t.Fatal("timed out handing an action request to the Controller")
		return nil
	}
}

func (host *controlledHost) awaitInterrupt(t *testing.T) {
	t.Helper()
	select {
	case <-host.interruptCalled:
	case <-time.After(controllerTestTimeout):
		t.Fatal("timed out waiting for the SDK Interrupt call")
	}
}

func receiveControlledTurn(t *testing.T, host *controlledHost) *controlledTurn {
	t.Helper()
	select {
	case turn := <-host.started:
		return turn
	case <-time.After(controllerTestTimeout):
		t.Fatal("timed out waiting for Host turn")
		return nil
	}
}

func assertNoControlledTurn(t *testing.T, host *controlledHost) {
	t.Helper()
	select {
	case turn := <-host.started:
		t.Fatalf("unexpected Host turn for prompt %q", turn.prompt)
	case <-time.After(25 * time.Millisecond):
	}
}

func receiveDomainEvent(t *testing.T, events <-chan Event) Event {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(controllerTestTimeout):
		t.Fatal("timed out waiting for Controller domain event")
		return Event{}
	}
}

func receiveClosedDomainEvents(t *testing.T, events <-chan Event) []Event {
	t.Helper()
	var received []Event
	for {
		select {
		case event, open := <-events:
			if !open {
				return received
			}
			received = append(received, event)
		case <-time.After(controllerTestTimeout):
			t.Fatal("timed out waiting for Controller subscription to close")
			return nil
		}
	}
}

func waitForTurnCancellation(t *testing.T, turn *controlledTurn) {
	t.Helper()
	select {
	case <-turn.cancelled:
	case <-time.After(controllerTestTimeout):
		t.Fatal("timed out waiting for Host RunTurn context cancellation")
	}
}

func assertNoDomainEvent(t *testing.T, events <-chan Event) {
	t.Helper()
	select {
	case event := <-events:
		t.Fatalf("unexpected Controller domain event: %#v", event)
	case <-time.After(25 * time.Millisecond):
	}
}

func waitForSnapshot(t *testing.T, controller *Controller, condition func(Snapshot) bool) Snapshot {
	t.Helper()
	deadline := time.Now().Add(controllerTestTimeout)
	for {
		snapshot := snapshotController(t, controller)
		if condition(snapshot) {
			return snapshot
		}
		if time.Now().After(deadline) {
			t.Fatalf("Controller snapshot did not reach expected state: %#v", snapshot)
		}
		time.Sleep(time.Millisecond)
	}
}

func controllerContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), controllerTestTimeout)
}

func startController(t *testing.T, host *controlledHost, options ...Option) *Controller {
	t.Helper()
	controller := NewController(host, options...)
	serviceContext, cancelService := context.WithCancel(context.Background())
	started := make(chan error, 1)
	go func() {
		started <- controller.Start(serviceContext)
	}()
	select {
	case err := <-started:
		if err != nil {
			cancelService()
			t.Fatalf("Start() error = %v, want nil", err)
		}
	case <-time.After(controllerTestTimeout):
		cancelService()
		t.Fatal("timed out starting Controller")
	}
	t.Cleanup(func() {
		ctx, cancel := controllerContext(t)
		defer cancel()
		if err := controller.Stop(ctx); err != nil {
			t.Errorf("cleanup Stop() error = %v", err)
		}
		cancelService()
	})
	return controller
}

func startControllerWithServiceContext(t *testing.T, host *controlledHost, options ...Option) (*Controller, context.CancelFunc) {
	t.Helper()
	controller := NewController(host, options...)
	serviceContext, cancelService := context.WithCancel(context.Background())
	if err := controller.Start(serviceContext); err != nil {
		cancelService()
		t.Fatalf("Start() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		cancelService()
		ctx, cancel := controllerContext(t)
		defer cancel()
		_ = controller.Stop(ctx)
	})
	return controller, cancelService
}

func snapshotController(t *testing.T, controller *Controller) Snapshot {
	t.Helper()
	ctx, cancel := controllerContext(t)
	defer cancel()
	snapshot, err := controller.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() error = %v, want nil", err)
	}
	return snapshot
}

func submitTerminal(t *testing.T, controller *Controller, prompt string) (string, error) {
	t.Helper()
	ctx, cancel := controllerContext(t)
	defer cancel()
	return controller.Submit(ctx, Input{Source: InputTerminal, Text: prompt})
}

func sequentialRequestIDs(ids ...string) (requestIDGenerator, *int) {
	next := 0
	return func() (string, error) {
		if next >= len(ids) {
			return "", fmt.Errorf("%w: exhausted test request IDs", ErrInternal)
		}
		id := ids[next]
		next++
		return id, nil
	}, &next
}

func monotonicRequestIDs() requestIDGenerator {
	next := 0
	return func() (string, error) {
		next++
		return fmt.Sprintf("request-%02d", next), nil
	}
}

type scopedTestError struct {
	scope projectclaude.ErrorScope
	err   error
}

func (e scopedTestError) Error() string                   { return e.err.Error() }
func (e scopedTestError) Unwrap() error                   { return e.err }
func (e scopedTestError) Scope() projectclaude.ErrorScope { return e.scope }

func TestControllerFirstTurnEventOrderAndSnapshot(t *testing.T) {
	host := newControlledHost()
	generator, _ := sequentialRequestIDs("request-01", "request-02")
	controller := startController(t, host, withRequestIDGenerator(generator))

	subscriptionContext, cancelSubscription := context.WithCancel(context.Background())
	t.Cleanup(cancelSubscription)
	initial, events, err := controller.Subscribe(subscriptionContext, 16)
	if err != nil {
		t.Fatalf("Subscribe() error = %v, want nil", err)
	}
	if initial.Seq != 0 || initial.SessionStatus != SessionStarting || initial.Turn.Status != TurnIdle {
		t.Fatalf("initial subscription Snapshot = %#v, want starting/idle seq 0", initial)
	}

	requestID, err := submitTerminal(t, controller, "first")
	if err != nil || requestID != "request-01" {
		t.Fatalf("Submit(first) = %q, %v, want request-01, nil", requestID, err)
	}
	first := receiveControlledTurn(t, host)
	first.emit(t, projectclaude.Event{Kind: projectclaude.EventText, Text: "pre"})
	assertNoDomainEvent(t, events)

	first.emit(t, projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-1"})
	first.emit(t, projectclaude.Event{Kind: projectclaude.EventText, Text: "post"})
	first.emit(t, projectclaude.Event{Kind: projectclaude.EventTurnCompleted})
	first.finish(t, nil)

	wantKinds := []EventKind{
		EventSessionStarted,
		EventTurnStarted,
		EventTextDelta,
		EventTextDelta,
		EventTurnCompleted,
	}
	wantTexts := []string{"", "first", "pre", "post", ""}
	for index, wantKind := range wantKinds {
		event := receiveDomainEvent(t, events)
		if event.Kind != wantKind || event.Seq != uint64(index+1) || event.SessionID != "session-1" {
			t.Fatalf("domain event %d = %#v, want kind %q seq %d session-1", index+1, event, wantKind, index+1)
		}
		if event.Text != wantTexts[index] {
			t.Fatalf("domain event %d text = %q, want %q", index+1, event.Text, wantTexts[index])
		}
		if event.Kind == EventSessionStarted {
			if event.RequestID != "" || event.Source != "" {
				t.Fatalf("session_started correlation = request %q source %q, want empty", event.RequestID, event.Source)
			}
		} else if event.RequestID != "request-01" || event.Source != InputTerminal {
			t.Fatalf("Turn event %d correlation = request %q source %q, want request-01/terminal", index+1, event.RequestID, event.Source)
		}
	}
	assertNoDomainEvent(t, events)

	snapshot := waitForSnapshot(t, controller, func(snapshot Snapshot) bool {
		return snapshot.Turn.Status == TurnCompleted
	})
	if snapshot.SessionID != "session-1" || snapshot.SessionStatus != SessionReady || snapshot.Turn.Message != "prepost" || snapshot.QueueDepth != 0 || snapshot.Seq != 5 {
		t.Fatalf("completed Snapshot = %#v, want ready/completed message prepost queue 0 seq 5", snapshot)
	}
	if snapshot.Turn.Error != nil || snapshot.Turn.PendingAction != nil {
		t.Fatalf("completed Turn pointers = error %#v pending %#v, want nil", snapshot.Turn.Error, snapshot.Turn.PendingAction)
	}

	requestID, err = submitTerminal(t, controller, "second")
	if err != nil || requestID != "request-02" {
		t.Fatalf("Submit(second) = %q, %v, want request-02, nil", requestID, err)
	}
	started := receiveDomainEvent(t, events)
	if started.Kind != EventTurnStarted || started.RequestID != "request-02" ||
		started.Source != InputTerminal || started.Text != "second" {
		t.Fatalf("second turn_started = %#v, want terminal request-02 text second", started)
	}
	snapshot = snapshotController(t, controller)
	if snapshot.SessionStatus != SessionReady || snapshot.Turn.Status != TurnRunning || snapshot.Turn.RequestID != "request-02" {
		t.Fatalf("second Turn Snapshot = %#v, want ready/running request-02", snapshot)
	}
	if snapshot.Turn.Message != "" || snapshot.Turn.Error != nil || snapshot.Turn.PendingAction != nil {
		t.Fatalf("second Turn stale state = message %q error %#v pending %#v, want cleared", snapshot.Turn.Message, snapshot.Turn.Error, snapshot.Turn.PendingAction)
	}

	second := receiveControlledTurn(t, host)
	second.emit(t, projectclaude.Event{Kind: projectclaude.EventTurnCompleted})
	second.finish(t, nil)
}

func TestControllerTurnStartedPublishesNormalizedInputOnlyWhenRunning(t *testing.T) {
	host := newControlledHost()
	generator, _ := sequentialRequestIDs("request-01", "request-02")
	controller := startController(t, host, withRequestIDGenerator(generator))

	subscriptionContext, cancelSubscription := context.WithCancel(context.Background())
	t.Cleanup(cancelSubscription)
	_, events, err := controller.Subscribe(subscriptionContext, 16)
	if err != nil {
		t.Fatalf("Subscribe() error = %v, want nil", err)
	}

	if _, submitErr := submitTerminal(t, controller, "  first  "); submitErr != nil {
		t.Fatalf("Submit(  first  ) error = %v", submitErr)
	}
	if _, submitErr := submitTerminal(t, controller, "  second  "); submitErr != nil {
		t.Fatalf("Submit(  second  ) error = %v", submitErr)
	}

	first := receiveControlledTurn(t, host)
	if first.prompt != "first" {
		t.Fatalf("first Host prompt = %q, want normalized first", first.prompt)
	}
	first.emit(t, projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-1"})

	if sessionStarted := receiveDomainEvent(t, events); sessionStarted.Kind != EventSessionStarted {
		t.Fatalf("first event = %#v, want session_started", sessionStarted)
	}
	started := receiveDomainEvent(t, events)
	if started.Kind != EventTurnStarted || started.RequestID != "request-01" || started.Text != "first" {
		t.Fatalf("first turn_started = %#v, want request-01 text first", started)
	}
	// 第二条输入仍在队列中：结算前不发布其正文。
	assertNoDomainEvent(t, events)

	first.emit(t, projectclaude.Event{Kind: projectclaude.EventTurnCompleted})
	first.finish(t, nil)
	if completed := receiveDomainEvent(t, events); completed.Kind != EventTurnCompleted {
		t.Fatalf("first turn_completed = %#v, want turn_completed", completed)
	}

	second := receiveControlledTurn(t, host)
	if second.prompt != "second" {
		t.Fatalf("second Host prompt = %q, want normalized second", second.prompt)
	}
	secondStarted := receiveDomainEvent(t, events)
	if secondStarted.Kind != EventTurnStarted || secondStarted.RequestID != "request-02" || secondStarted.Text != "second" {
		t.Fatalf("second turn_started = %#v, want request-02 text second", secondStarted)
	}
	second.emit(t, projectclaude.Event{Kind: projectclaude.EventTurnCompleted})
	second.finish(t, nil)
}

func TestControllerTurnFailureContinuesQueue(t *testing.T) {
	host := newControlledHost()
	generator, _ := sequentialRequestIDs("request-01", "request-02")
	controller := startController(t, host, withRequestIDGenerator(generator))

	subscriptionContext, cancelSubscription := context.WithCancel(context.Background())
	t.Cleanup(cancelSubscription)
	_, events, err := controller.Subscribe(subscriptionContext, 16)
	if err != nil {
		t.Fatalf("Subscribe() error = %v, want nil", err)
	}

	if requestID, submitErr := submitTerminal(t, controller, "first"); submitErr != nil || requestID != "request-01" {
		t.Fatalf("Submit(first) = %q, %v, want request-01, nil", requestID, submitErr)
	}
	if requestID, submitErr := submitTerminal(t, controller, "second"); submitErr != nil || requestID != "request-02" {
		t.Fatalf("Submit(second) = %q, %v, want request-02, nil", requestID, submitErr)
	}

	first := receiveControlledTurn(t, host)
	first.emit(t, projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-1"})
	first.emit(t, projectclaude.Event{Kind: projectclaude.EventTurnFailed, Text: "adapter failure"})
	first.finish(t, scopedTestError{scope: projectclaude.ScopeTurn, err: errors.New("turn result failure")})

	second := receiveControlledTurn(t, host)
	if second.prompt != "second" {
		t.Fatalf("second Host prompt = %q, want second", second.prompt)
	}

	wantKinds := []EventKind{EventSessionStarted, EventTurnStarted, EventTurnFailed, EventTurnStarted}
	turnFailedCount := 0
	for index, wantKind := range wantKinds {
		event := receiveDomainEvent(t, events)
		if event.Kind != wantKind {
			t.Fatalf("domain event %d kind = %q, want %q: %#v", index+1, event.Kind, wantKind, event)
		}
		if event.Kind == EventTurnFailed {
			turnFailedCount++
			if event.RequestID != "request-01" || event.Error == nil || event.Error.Code != "turn_failed" || event.Error.Message != "adapter failure" {
				t.Fatalf("turn_failed event = %#v, want Adapter diagnostic for request-01", event)
			}
		}
	}
	if turnFailedCount != 1 {
		t.Fatalf("turn_failed event count = %d, want 1", turnFailedCount)
	}
	assertNoDomainEvent(t, events)

	snapshot := snapshotController(t, controller)
	if snapshot.SessionStatus != SessionReady || snapshot.Turn.Status != TurnRunning || snapshot.Turn.RequestID != "request-02" || snapshot.QueueDepth != 0 {
		t.Fatalf("Snapshot after Turn failure = %#v, want ready with second Turn running", snapshot)
	}

	second.emit(t, projectclaude.Event{Kind: projectclaude.EventTurnCompleted})
	second.finish(t, nil)
}

func TestControllerSessionFailureClearsQueue(t *testing.T) {
	host := newControlledHost()
	generator, _ := sequentialRequestIDs("request-01", "request-02")
	controller := startController(t, host, withRequestIDGenerator(generator))

	subscriptionContext, cancelSubscription := context.WithCancel(context.Background())
	t.Cleanup(cancelSubscription)
	_, events, err := controller.Subscribe(subscriptionContext, 16)
	if err != nil {
		t.Fatalf("Subscribe() error = %v, want nil", err)
	}

	if _, err := submitTerminal(t, controller, "first"); err != nil {
		t.Fatalf("Submit(first) error = %v, want nil", err)
	}
	if _, err := submitTerminal(t, controller, "queued"); err != nil {
		t.Fatalf("Submit(queued) error = %v, want nil", err)
	}
	first := receiveControlledTurn(t, host)
	first.emit(t, projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-1"})
	first.finish(t, scopedTestError{scope: projectclaude.ScopeSession, err: errors.New("session transport failed")})

	snapshot := waitForSnapshot(t, controller, func(snapshot Snapshot) bool {
		return snapshot.SessionStatus == SessionFailed
	})
	if snapshot.SessionError == nil || snapshot.SessionError.Code != "session_failed" || snapshot.SessionError.Message != "session transport failed" {
		t.Fatalf("Session failure Snapshot error = %#v, want session_failed diagnostic", snapshot.SessionError)
	}
	if snapshot.Turn.Status != TurnFailed || snapshot.Turn.Error == nil || snapshot.Turn.Error.Code != "turn_failed" || snapshot.QueueDepth != 0 {
		t.Fatalf("Session failure Snapshot = %#v, want failed Turn and cleared queue", snapshot)
	}
	assertNoControlledTurn(t, host)
	if requestID, submitErr := submitTerminal(t, controller, "rejected"); requestID != "" || !errors.Is(submitErr, ErrSessionUnavailable) {
		t.Fatalf("Submit after Session failure = %q, %v, want empty ID and ErrSessionUnavailable", requestID, submitErr)
	}

	wantKinds := []EventKind{EventSessionStarted, EventTurnStarted, EventTurnFailed, EventSessionFailed}
	for index, wantKind := range wantKinds {
		event := receiveDomainEvent(t, events)
		if event.Kind != wantKind {
			t.Fatalf("domain event %d kind = %q, want %q: %#v", index+1, event.Kind, wantKind, event)
		}
	}
	assertNoDomainEvent(t, events)
}

func TestControllerSynthesizesMissingTerminal(t *testing.T) {
	t.Run("nil result fails Session", func(t *testing.T) {
		host := newControlledHost()
		generator, _ := sequentialRequestIDs("request-01")
		controller := startController(t, host, withRequestIDGenerator(generator))

		subscriptionContext, cancelSubscription := context.WithCancel(context.Background())
		t.Cleanup(cancelSubscription)
		_, events, err := controller.Subscribe(subscriptionContext, 16)
		if err != nil {
			t.Fatalf("Subscribe() error = %v, want nil", err)
		}

		if _, err := submitTerminal(t, controller, "missing terminal"); err != nil {
			t.Fatalf("Submit() error = %v, want nil", err)
		}
		turn := receiveControlledTurn(t, host)
		turn.emit(t, projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-1"})
		turn.finish(t, nil)

		snapshot := waitForSnapshot(t, controller, func(snapshot Snapshot) bool {
			return snapshot.SessionStatus == SessionFailed
		})
		if snapshot.Turn.Status != TurnFailed || snapshot.SessionError == nil || !strings.Contains(snapshot.SessionError.Message, "Host returned without a terminal event") {
			t.Fatalf("missing terminal Snapshot = %#v, want failed Session and Turn with diagnostic", snapshot)
		}

		wantKinds := []EventKind{EventSessionStarted, EventTurnStarted, EventTurnFailed, EventSessionFailed}
		for index, wantKind := range wantKinds {
			event := receiveDomainEvent(t, events)
			if event.Kind != wantKind {
				t.Fatalf("domain event %d kind = %q, want %q: %#v", index+1, event.Kind, wantKind, event)
			}
		}
		assertNoDomainEvent(t, events)
	})

	t.Run("Turn scope result fails only Turn", func(t *testing.T) {
		host := newControlledHost()
		generator, _ := sequentialRequestIDs("request-01")
		controller := startController(t, host, withRequestIDGenerator(generator))

		subscriptionContext, cancelSubscription := context.WithCancel(context.Background())
		t.Cleanup(cancelSubscription)
		_, events, err := controller.Subscribe(subscriptionContext, 16)
		if err != nil {
			t.Fatalf("Subscribe() error = %v, want nil", err)
		}

		if _, err := submitTerminal(t, controller, "turn failure"); err != nil {
			t.Fatalf("Submit() error = %v, want nil", err)
		}
		turn := receiveControlledTurn(t, host)
		turn.emit(t, projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-1"})
		turn.finish(t, scopedTestError{scope: projectclaude.ScopeTurn, err: errors.New("turn failed without Adapter terminal")})

		snapshot := waitForSnapshot(t, controller, func(snapshot Snapshot) bool {
			return snapshot.Turn.Status == TurnFailed
		})
		if snapshot.SessionStatus != SessionReady || snapshot.SessionError != nil || snapshot.Turn.Error == nil || snapshot.Turn.Error.Message != "turn failed without Adapter terminal" {
			t.Fatalf("Turn-scope missing terminal Snapshot = %#v, want ready Session and failed Turn", snapshot)
		}

		wantKinds := []EventKind{EventSessionStarted, EventTurnStarted, EventTurnFailed}
		for index, wantKind := range wantKinds {
			event := receiveDomainEvent(t, events)
			if event.Kind != wantKind {
				t.Fatalf("domain event %d kind = %q, want %q: %#v", index+1, event.Kind, wantKind, event)
			}
		}
		assertNoDomainEvent(t, events)
	})
}

func TestControllerPreInitFailureIsLocalOnly(t *testing.T) {
	host := newControlledHost()
	generator, _ := sequentialRequestIDs("request-01", "request-02")
	controller := startController(t, host, withRequestIDGenerator(generator))

	subscriptionContext, cancelSubscription := context.WithCancel(context.Background())
	t.Cleanup(cancelSubscription)
	_, events, err := controller.Subscribe(subscriptionContext, 16)
	if err != nil {
		t.Fatalf("Subscribe() error = %v, want nil", err)
	}

	if _, err := submitTerminal(t, controller, "first"); err != nil {
		t.Fatalf("Submit(first) error = %v, want nil", err)
	}
	if _, err := submitTerminal(t, controller, "second"); err != nil {
		t.Fatalf("Submit(second) error = %v, want nil", err)
	}
	first := receiveControlledTurn(t, host)
	first.emit(t, projectclaude.Event{Kind: projectclaude.EventText, Text: "discarded"})
	first.finish(t, scopedTestError{scope: projectclaude.ScopeTurn, err: errors.New("pre-init turn failed")})

	failed := receiveDomainEvent(t, events)
	if failed.Kind != EventTurnFailed || failed.SessionID != "" || failed.RequestID != "request-01" || failed.Source != InputTerminal {
		t.Fatalf("pre-init local event = %#v, want turn_failed with empty SessionID and first correlation", failed)
	}
	second := receiveControlledTurn(t, host)
	second.emit(t, projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-1"})
	second.emit(t, projectclaude.Event{Kind: projectclaude.EventTurnCompleted})
	second.finish(t, nil)

	wantKinds := []EventKind{EventSessionStarted, EventTurnStarted, EventTurnCompleted}
	for index, wantKind := range wantKinds {
		event := receiveDomainEvent(t, events)
		if event.Kind != wantKind {
			t.Fatalf("second Turn domain event %d kind = %q, want %q: %#v", index+1, event.Kind, wantKind, event)
		}
		if event.RequestID != "" && event.RequestID != "request-02" {
			t.Fatalf("second Turn domain event %d request ID = %q, want empty or request-02", index+1, event.RequestID)
		}
	}
	assertNoDomainEvent(t, events)

	snapshot := waitForSnapshot(t, controller, func(snapshot Snapshot) bool {
		return snapshot.Turn.Status == TurnCompleted
	})
	if snapshot.SessionStatus != SessionReady || snapshot.Turn.Message != "" || snapshot.Seq != 4 {
		t.Fatalf("post-init Snapshot = %#v, want ready/completed with discarded pre-init text and seq 4", snapshot)
	}
}

func TestControllerSnapshotReturnsDeepCopy(t *testing.T) {
	host := newControlledHost()
	controller := NewController(host)
	controller.initialSnapshot.SessionError = &ErrorInfo{Code: "session-original", Message: "session original"}
	controller.initialSnapshot.Turn.Error = &ErrorInfo{Code: "turn-original", Message: "turn original"}
	controller.initialSnapshot.Turn.PendingAction = &PendingAction{}

	serviceContext, cancelService := context.WithCancel(context.Background())
	if err := controller.Start(serviceContext); err != nil {
		cancelService()
		t.Fatalf("Start() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		ctx, cancel := controllerContext(t)
		defer cancel()
		if err := controller.Stop(ctx); err != nil {
			t.Errorf("cleanup Stop() error = %v", err)
		}
		cancelService()
	})

	first := snapshotController(t, controller)
	subscriptionContext, cancelSubscription := context.WithCancel(context.Background())
	t.Cleanup(cancelSubscription)
	subscribed, _, err := controller.Subscribe(subscriptionContext, 1)
	if err != nil {
		t.Fatalf("Subscribe() error = %v, want nil", err)
	}
	if first.SessionError == subscribed.SessionError || first.Turn.Error == subscribed.Turn.Error {
		t.Fatalf("Snapshot and Subscribe returned shared pointers: Snapshot %#v Subscribe %#v", first, subscribed)
	}

	first.SessionError.Code = "mutated-first"
	first.Turn.Error.Message = "mutated-first"
	first.Turn.PendingAction = nil
	subscribed.SessionError.Message = "mutated-subscribe"
	subscribed.Turn.Error.Code = "mutated-subscribe"
	subscribed.Turn.PendingAction = nil

	current := snapshotController(t, controller)
	if current.SessionError == nil || current.SessionError.Code != "session-original" || current.SessionError.Message != "session original" {
		t.Fatalf("actor SessionError mutated through returned Snapshot: %#v", current.SessionError)
	}
	if current.Turn.Error == nil || current.Turn.Error.Code != "turn-original" || current.Turn.Error.Message != "turn original" {
		t.Fatalf("actor Turn.Error mutated through returned Snapshot: %#v", current.Turn.Error)
	}
	if current.Turn.PendingAction == nil {
		t.Fatal("actor PendingAction mutated through a returned Snapshot, want non-nil")
	}
}

func TestControllerStartAndInitialSnapshot(t *testing.T) {
	t.Run("connected", func(t *testing.T) {
		host := newControlledHost()
		controller := startController(t, host)

		snapshot := snapshotController(t, controller)
		if snapshot.SessionStatus != SessionStarting || snapshot.Turn.Status != TurnIdle || snapshot.Seq != 0 || snapshot.QueueDepth != 0 {
			t.Fatalf("initial Snapshot() = %#v, want starting/idle seq 0 queue depth 0", snapshot)
		}
		if snapshot.Turn.PendingAction != nil {
			t.Fatalf("initial pending action = %#v, want nil", snapshot.Turn.PendingAction)
		}
	})

	t.Run("connect failure", func(t *testing.T) {
		connectErr := errors.New("connect failed")
		host := newControlledHost()
		host.connectErr = connectErr
		controller := NewController(host)

		ctx, cancel := controllerContext(t)
		err := controller.Start(ctx)
		cancel()
		if !errors.Is(err, connectErr) {
			t.Fatalf("Start() error = %v, want errors.Is connect error", err)
		}

		snapshot := snapshotController(t, controller)
		if snapshot.SessionStatus != SessionFailed {
			t.Fatalf("failed Snapshot session status = %q, want %q", snapshot.SessionStatus, SessionFailed)
		}
		if snapshot.SessionError == nil || snapshot.SessionError.Code != "session_failed" {
			t.Fatalf("failed Snapshot session error = %#v, want session_failed", snapshot.SessionError)
		}
		if snapshot.Turn.PendingAction != nil {
			t.Fatalf("failed Snapshot pending action = %#v, want nil", snapshot.Turn.PendingAction)
		}
		_, closeCalls := host.counts()
		if closeCalls != 1 {
			t.Fatalf("Host Close calls = %d, want 1", closeCalls)
		}

		ctx, cancel = controllerContext(t)
		defer cancel()
		if err := controller.Stop(ctx); err != nil {
			t.Fatalf("Stop() after failed Start error = %v, want nil", err)
		}
	})
}

func TestControllerStartIsResolvedWhenStopFinishes(t *testing.T) {
	controller := NewController(newControlledHost())
	controller.actorLive.Store(true)
	state := actorState{
		snapshot:     cloneSnapshot(controller.initialSnapshot),
		subscribers:  make(map[uint64]actorSubscriber),
		workerClosed: true,
		stopping:     true,
	}

	controller.finishStop(&state)
	select {
	case err := <-controller.startResult:
		if !errors.Is(err, ErrSessionUnavailable) {
			t.Fatalf("pending Start result = %v, want ErrSessionUnavailable", err)
		}
	case <-time.After(controllerTestTimeout):
		t.Fatal("finishStop did not resolve pending Start")
	}
}

func TestControllerStopDuringConnectResolvesStart(t *testing.T) {
	closeErr := errors.New("close during explicit Stop")
	host := newControlledHost()
	host.closeErr = closeErr
	host.connectEntered = make(chan struct{})
	connectRelease := make(chan struct{})
	host.connectRelease = connectRelease
	controller := NewController(host)
	startResult := make(chan error, 1)
	go func() {
		startResult <- controller.Start(context.Background())
	}()

	var releaseConnectOnce sync.Once
	releaseConnect := func() { releaseConnectOnce.Do(func() { close(connectRelease) }) }
	t.Cleanup(releaseConnect)
	select {
	case <-host.connectEntered:
	case <-time.After(controllerTestTimeout):
		t.Fatal("timed out waiting for Host Connect to start")
	}

	stopResult := make(chan error, 1)
	go func() {
		stopResult <- controller.Stop(context.Background())
	}()
	select {
	case err := <-startResult:
		if !errors.Is(err, ErrSessionUnavailable) {
			t.Fatalf("Start() interrupted by explicit Stop error = %v, want ErrSessionUnavailable", err)
		}
	case <-time.After(controllerTestTimeout):
		t.Fatal("explicit Stop left Start blocked during Connect")
	}
	select {
	case err := <-stopResult:
		if !errors.Is(err, closeErr) {
			t.Fatalf("Stop() during Connect error = %v, want cached %v", err, closeErr)
		}
	case <-time.After(controllerTestTimeout):
		t.Fatal("timed out waiting for Stop() during Connect")
	}

	final := snapshotController(t, controller)
	if final.SessionStatus != SessionStopped || final.SessionError != nil || final.Turn.Status != TurnIdle || final.QueueDepth != 0 {
		t.Fatalf("final Snapshot after explicit Stop during Connect = %#v, want stopped/idle", final)
	}
	_, closeCalls := host.counts()
	if closeCalls != 1 {
		t.Fatalf("Host Close calls = %d, want 1", closeCalls)
	}
}

func TestControllerConnectFailurePublishesSessionFailed(t *testing.T) {
	connectErr := errors.New("known connect failure")
	closeErr := errors.New("close after connect failure")
	host := newControlledHost()
	host.connectErr = connectErr
	host.closeErr = closeErr
	host.connectEntered = make(chan struct{})
	connectRelease := make(chan struct{})
	host.connectRelease = connectRelease
	controller := NewController(host)
	serviceContext, cancelService := context.WithCancel(context.Background())
	startResult := make(chan error, 1)
	go func() {
		startResult <- controller.Start(serviceContext)
	}()

	var releaseConnectOnce sync.Once
	releaseConnect := func() { releaseConnectOnce.Do(func() { close(connectRelease) }) }
	t.Cleanup(func() {
		releaseConnect()
		cancelService()
		ctx, cancel := controllerContext(t)
		defer cancel()
		_ = controller.Stop(ctx)
	})
	select {
	case <-host.connectEntered:
	case <-time.After(controllerTestTimeout):
		t.Fatal("timed out waiting for Host Connect to start")
	}

	subscriptionContext, cancelSubscription := context.WithCancel(context.Background())
	t.Cleanup(cancelSubscription)
	initial, events, err := controller.Subscribe(subscriptionContext, 4)
	if err != nil {
		t.Fatalf("Subscribe() during Connect error = %v, want nil", err)
	}
	if initial.SessionStatus != SessionStarting || initial.Seq != 0 {
		t.Fatalf("initial Connect Snapshot = %#v, want starting seq 0", initial)
	}
	releaseConnect()
	select {
	case err := <-startResult:
		if !errors.Is(err, connectErr) {
			t.Fatalf("Start() Connect error = %v, want %v", err, connectErr)
		}
	case <-time.After(controllerTestTimeout):
		t.Fatal("timed out waiting for Start() Connect failure")
	}

	failedEvent := receiveDomainEvent(t, events)
	if failedEvent.Kind != EventSessionFailed || failedEvent.SessionID != "" || failedEvent.SessionStatus != SessionFailed || failedEvent.Seq != 1 {
		t.Fatalf("Connect failure event = %#v, want local session_failed seq 1", failedEvent)
	}
	if failedEvent.Error == nil || failedEvent.Error.Code != "session_failed" || failedEvent.Error.Message != connectErr.Error() {
		t.Fatalf("Connect failure event error = %#v, want original diagnostic", failedEvent.Error)
	}
	failed := snapshotController(t, controller)
	if failed.SessionStatus != SessionFailed || failed.SessionError == nil || failed.SessionError.Code != "session_failed" || failed.SessionError.Message != connectErr.Error() || failed.Seq != 1 {
		t.Fatalf("Connect failure Snapshot = %#v, want failed with original root cause and seq 1", failed)
	}

	ctx, cancel := controllerContext(t)
	stopErr := controller.Stop(ctx)
	cancel()
	if !errors.Is(stopErr, closeErr) {
		t.Fatalf("Stop() after Connect failure error = %v, want %v", stopErr, closeErr)
	}
	if remaining := receiveClosedDomainEvents(t, events); len(remaining) != 0 {
		t.Fatalf("Stop() after Connect failure published unexpected events: %#v", remaining)
	}
	_, closeCalls := host.counts()
	if closeCalls != 1 {
		t.Fatalf("Host Close calls = %d, want 1", closeCalls)
	}
}

func TestControllerSubmitSerialFIFO(t *testing.T) {
	host := newControlledHost()
	generator, _ := sequentialRequestIDs("request-01", "request-02", "request-03")
	controller := startController(t, host, withRequestIDGenerator(generator))

	for index, prompt := range []string{"first", "second", "third"} {
		requestID, err := submitTerminal(t, controller, prompt)
		if err != nil {
			t.Fatalf("Submit(%q) error = %v, want nil", prompt, err)
		}
		wantID := fmt.Sprintf("request-%02d", index+1)
		if requestID != wantID {
			t.Fatalf("Submit(%q) request ID = %q, want %q", prompt, requestID, wantID)
		}
		if snapshot := snapshotController(t, controller); snapshot.Turn.PendingAction != nil {
			t.Fatalf("Snapshot pending action after Submit(%q) = %#v, want nil", prompt, snapshot.Turn.PendingAction)
		}
	}

	first := receiveControlledTurn(t, host)
	if first.prompt != "first" {
		t.Fatalf("first Host prompt = %q, want first", first.prompt)
	}
	assertNoControlledTurn(t, host)

	first.emit(t, projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-1"})
	first.emit(t, projectclaude.Event{Kind: projectclaude.EventTurnCompleted})
	first.finish(t, nil)
	second := receiveControlledTurn(t, host)
	if second.prompt != "second" {
		t.Fatalf("second Host prompt = %q, want second", second.prompt)
	}
	assertNoControlledTurn(t, host)

	second.emit(t, projectclaude.Event{Kind: projectclaude.EventTurnCompleted})
	second.finish(t, nil)
	third := receiveControlledTurn(t, host)
	if third.prompt != "third" {
		t.Fatalf("third Host prompt = %q, want third", third.prompt)
	}
	assertNoControlledTurn(t, host)
	third.emit(t, projectclaude.Event{Kind: projectclaude.EventTurnCompleted})
	third.finish(t, nil)

	deadline := time.Now().Add(controllerTestTimeout)
	for {
		snapshot := snapshotController(t, controller)
		if snapshot.QueueDepth == 0 && snapshot.Turn.Status != TurnRunning {
			if snapshot.Turn.PendingAction != nil {
				t.Fatalf("final pending action = %#v, want nil", snapshot.Turn.PendingAction)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("controller did not release final Turn: %#v", snapshot)
		}
		time.Sleep(time.Millisecond)
	}

	maxActive, _ := host.counts()
	if maxActive != 1 {
		t.Fatalf("maximum concurrent Host turns = %d, want 1", maxActive)
	}
}

func TestControllerSubmitWhileConnectPendingIsUnavailable(t *testing.T) {
	host := newControlledHost()
	host.connectEntered = make(chan struct{})
	connectRelease := make(chan struct{})
	host.connectRelease = connectRelease
	generator, generated := sequentialRequestIDs("request-01")
	controller := NewController(host, withRequestIDGenerator(generator))

	serviceContext, cancelService := context.WithCancel(context.Background())
	startFinished := make(chan struct{})
	var startErr error
	go func() {
		startErr = controller.Start(serviceContext)
		close(startFinished)
	}()

	var releaseOnce sync.Once
	releaseConnect := func() { releaseOnce.Do(func() { close(connectRelease) }) }
	t.Cleanup(func() {
		releaseConnect()
		select {
		case <-startFinished:
		case <-time.After(controllerTestTimeout):
			t.Error("cleanup timed out waiting for Start")
		}
		ctx, cancel := controllerContext(t)
		defer cancel()
		if err := controller.Stop(ctx); err != nil && !errors.Is(err, ErrSessionUnavailable) {
			t.Errorf("cleanup Stop() error = %v", err)
		}
		cancelService()
	})

	select {
	case <-host.connectEntered:
	case <-time.After(controllerTestTimeout):
		t.Fatal("timed out waiting for Host Connect to start")
	}

	requestID, err := submitTerminal(t, controller, "before connect")
	if requestID != "" || !errors.Is(err, ErrSessionUnavailable) {
		t.Fatalf("Submit() while Connect pending = %q, %v, want empty ID and ErrSessionUnavailable", requestID, err)
	}
	if *generated != 0 {
		t.Fatalf("request IDs generated before Connect = %d, want 0", *generated)
	}
	assertNoControlledTurn(t, host)

	releaseConnect()
	select {
	case <-startFinished:
		if startErr != nil {
			t.Fatalf("Start() error after releasing Connect = %v, want nil", startErr)
		}
	case <-time.After(controllerTestTimeout):
		t.Fatal("timed out waiting for Start after releasing Connect")
	}

	requestID, err = submitTerminal(t, controller, "after connect")
	if requestID != "request-01" || err != nil {
		t.Fatalf("Submit() after Connect = %q, %v, want request-01, nil", requestID, err)
	}
	turn := receiveControlledTurn(t, host)
	if turn.prompt != "after connect" {
		t.Fatalf("Host prompt = %q, want after connect", turn.prompt)
	}
	turn.emit(t, projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-1"})
	turn.emit(t, projectclaude.Event{Kind: projectclaude.EventTurnCompleted})
	turn.finish(t, nil)
}

func TestControllerQueueCapacity(t *testing.T) {
	host := newControlledHost()
	ids := make([]string, 0, QueueCapacity+1)
	for index := 1; index <= QueueCapacity+1; index++ {
		ids = append(ids, fmt.Sprintf("request-%02d", index))
	}
	generator, generated := sequentialRequestIDs(ids...)
	controller := startController(t, host, withRequestIDGenerator(generator))

	if requestID, err := submitTerminal(t, controller, "running"); err != nil || requestID != "request-01" {
		t.Fatalf("first Submit() = %q, %v, want request-01, nil", requestID, err)
	}
	running := receiveControlledTurn(t, host)

	for index := 0; index < QueueCapacity; index++ {
		prompt := fmt.Sprintf("waiting-%02d", index+1)
		requestID, err := submitTerminal(t, controller, prompt)
		if err != nil {
			t.Fatalf("Submit(%q) error = %v, want nil", prompt, err)
		}
		wantID := fmt.Sprintf("request-%02d", index+2)
		if requestID != wantID {
			t.Fatalf("Submit(%q) request ID = %q, want %q", prompt, requestID, wantID)
		}
	}

	snapshot := snapshotController(t, controller)
	if snapshot.QueueDepth != QueueCapacity {
		t.Fatalf("queue depth = %d, want %d", snapshot.QueueDepth, QueueCapacity)
	}
	if snapshot.Turn.PendingAction != nil {
		t.Fatalf("queue-full Snapshot pending action = %#v, want nil", snapshot.Turn.PendingAction)
	}

	requestID, err := submitTerminal(t, controller, "rejected")
	if requestID != "" || !errors.Is(err, ErrQueueFull) {
		t.Fatalf("rejected Submit() = %q, %v, want empty ID and ErrQueueFull", requestID, err)
	}
	if *generated != QueueCapacity+1 {
		t.Fatalf("request IDs generated = %d, want %d; rejection consumed an ID", *generated, QueueCapacity+1)
	}
	assertNoControlledTurn(t, host)

	running.emit(t, projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-1"})
	running.emit(t, projectclaude.Event{Kind: projectclaude.EventTurnCompleted})
	running.finish(t, nil)
}

func TestControllerRequestIDFailureDoesNotEnqueue(t *testing.T) {
	host := newControlledHost()
	generator := func() (string, error) {
		return "", fmt.Errorf("%w: entropy", ErrInternal)
	}
	controller := startController(t, host, withRequestIDGenerator(generator))

	requestID, err := submitTerminal(t, controller, "not accepted")
	if requestID != "" || !errors.Is(err, ErrInternal) {
		t.Fatalf("Submit() = %q, %v, want empty ID and ErrInternal", requestID, err)
	}
	snapshot := snapshotController(t, controller)
	if snapshot.QueueDepth != 0 {
		t.Fatalf("queue depth = %d, want 0", snapshot.QueueDepth)
	}
	if snapshot.Turn.PendingAction != nil {
		t.Fatalf("request-ID failure pending action = %#v, want nil", snapshot.Turn.PendingAction)
	}
	assertNoControlledTurn(t, host)
}

func TestControllerConcurrentSubmitIsSerialized(t *testing.T) {
	const submitCount = 16
	host := newControlledHost()
	controller := startController(t, host, withRequestIDGenerator(monotonicRequestIDs()))

	type accepted struct {
		prompt    string
		requestID string
		err       error
	}
	acceptedResults := make(chan accepted, submitCount)
	start := make(chan struct{})
	var submitters sync.WaitGroup
	for index := 1; index <= submitCount; index++ {
		prompt := fmt.Sprintf("prompt-%02d", index)
		submitters.Add(1)
		go func() {
			defer submitters.Done()
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), controllerTestTimeout)
			defer cancel()
			requestID, err := controller.Submit(ctx, Input{Source: InputTerminal, Text: prompt})
			acceptedResults <- accepted{prompt: prompt, requestID: requestID, err: err}
		}()
	}
	close(start)
	submitters.Wait()
	close(acceptedResults)

	acceptedByID := make([]accepted, 0, submitCount)
	seen := make(map[string]bool, submitCount)
	for result := range acceptedResults {
		if result.err != nil {
			t.Fatalf("Submit(%q) error = %v, want nil", result.prompt, result.err)
		}
		if seen[result.requestID] {
			t.Fatalf("duplicate request ID %q", result.requestID)
		}
		seen[result.requestID] = true
		acceptedByID = append(acceptedByID, result)
	}
	sort.Slice(acceptedByID, func(left, right int) bool {
		leftNumber, leftErr := strconv.Atoi(strings.TrimPrefix(acceptedByID[left].requestID, "request-"))
		rightNumber, rightErr := strconv.Atoi(strings.TrimPrefix(acceptedByID[right].requestID, "request-"))
		if leftErr != nil || rightErr != nil {
			t.Fatalf("invalid generated request IDs %q and %q", acceptedByID[left].requestID, acceptedByID[right].requestID)
		}
		return leftNumber < rightNumber
	})

	for index, acceptedInput := range acceptedByID {
		turn := receiveControlledTurn(t, host)
		if turn.prompt != acceptedInput.prompt {
			t.Fatalf("Host prompt %d = %q, want %q for %s", index+1, turn.prompt, acceptedInput.prompt, acceptedInput.requestID)
		}
		assertNoControlledTurn(t, host)
		if index == 0 {
			turn.emit(t, projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-1"})
		}
		turn.emit(t, projectclaude.Event{Kind: projectclaude.EventTurnCompleted})
		turn.finish(t, nil)
	}

	maxActive, _ := host.counts()
	if maxActive != 1 {
		t.Fatalf("maximum concurrent Host turns = %d, want 1", maxActive)
	}
}

func TestControllerSubscribeSnapshotHasNoGap(t *testing.T) {
	host := newControlledHost()
	generator, _ := sequentialRequestIDs("request-01", "request-02")
	controller := startController(t, host, withRequestIDGenerator(generator))

	contextA, cancelA := context.WithCancel(context.Background())
	t.Cleanup(cancelA)
	snapshotA, eventsA, err := controller.Subscribe(contextA, 16)
	if err != nil {
		t.Fatalf("Subscribe(A) error = %v, want nil", err)
	}
	if snapshotA.Seq != 0 {
		t.Fatalf("Subscribe(A) Snapshot seq = %d, want 0", snapshotA.Seq)
	}

	if requestID, submitErr := submitTerminal(t, controller, "first"); submitErr != nil || requestID != "request-01" {
		t.Fatalf("Submit(first) = %q, %v, want request-01, nil", requestID, submitErr)
	}
	first := receiveControlledTurn(t, host)
	first.emit(t, projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-1"})
	first.emit(t, projectclaude.Event{Kind: projectclaude.EventText, Text: "before"})

	for index, wantKind := range []EventKind{EventSessionStarted, EventTurnStarted, EventTextDelta} {
		event := receiveDomainEvent(t, eventsA)
		if event.Kind != wantKind || event.Seq != uint64(index+1) {
			t.Fatalf("pre-boundary event A %d = %#v, want kind %q seq %d", index+1, event, wantKind, index+1)
		}
	}

	contextB, cancelB := context.WithCancel(context.Background())
	snapshotB, eventsB, err := controller.Subscribe(contextB, 16)
	if err != nil {
		cancelB()
		t.Fatalf("Subscribe(B) error = %v, want nil", err)
	}
	if snapshotB.Seq != 3 || snapshotB.Seq != snapshotController(t, controller).Seq {
		cancelB()
		t.Fatalf("Subscribe(B) Snapshot seq = %d, want atomic Actor boundary 3", snapshotB.Seq)
	}
	if snapshotB.Turn.Message != "before" {
		cancelB()
		t.Fatalf("Subscribe(B) Snapshot message = %q, want before", snapshotB.Turn.Message)
	}

	first.emit(t, projectclaude.Event{Kind: projectclaude.EventText, Text: "shared"})
	first.emit(t, projectclaude.Event{Kind: projectclaude.EventTurnCompleted})
	for index, wantKind := range []EventKind{EventTextDelta, EventTurnCompleted} {
		eventA := receiveDomainEvent(t, eventsA)
		eventB := receiveDomainEvent(t, eventsB)
		wantSeq := snapshotB.Seq + uint64(index+1)
		if eventA != eventB {
			t.Fatalf("post-boundary event %d differs: A %#v B %#v", index+1, eventA, eventB)
		}
		if eventA.Kind != wantKind || eventA.Seq != wantSeq || eventA.Seq <= snapshotB.Seq {
			t.Fatalf("post-boundary event %d = %#v, want kind %q seq %d > %d", index+1, eventA, wantKind, wantSeq, snapshotB.Seq)
		}
	}
	first.finish(t, nil)

	cancelB()
	if remaining := receiveClosedDomainEvents(t, eventsB); len(remaining) != 0 {
		t.Fatalf("Subscribe(B) received events while cancelling: %#v", remaining)
	}

	if requestID, submitErr := submitTerminal(t, controller, "second"); submitErr != nil || requestID != "request-02" {
		t.Fatalf("Submit(second) = %q, %v, want request-02, nil", requestID, submitErr)
	}
	second := receiveControlledTurn(t, host)
	if event := receiveDomainEvent(t, eventsA); event.Kind != EventTurnStarted || event.Seq != 6 {
		t.Fatalf("A event after B cancellation = %#v, want turn_started seq 6", event)
	}
	second.emit(t, projectclaude.Event{Kind: projectclaude.EventText, Text: "after-cancel"})
	if event := receiveDomainEvent(t, eventsA); event.Kind != EventTextDelta || event.Seq != 7 || event.Text != "after-cancel" {
		t.Fatalf("A text after B cancellation = %#v, want text_delta seq 7", event)
	}
	second.emit(t, projectclaude.Event{Kind: projectclaude.EventTurnCompleted})
	if event := receiveDomainEvent(t, eventsA); event.Kind != EventTurnCompleted || event.Seq != 8 {
		t.Fatalf("A terminal event after B cancellation = %#v, want turn_completed seq 8", event)
	}
	second.finish(t, nil)
}

func TestControllerSubscribeCancellationIsOwnedByActor(t *testing.T) {
	host := newControlledHost()
	controller := startController(t, host)

	subscriptionContext, cancelSubscription := context.WithCancel(context.Background())
	cancelSubscription()
	response := make(chan subscribeResponse, 1)
	select {
	case controller.commands <- subscribeCommand{
		ctx:      subscriptionContext,
		capacity: 1,
		response: response,
	}:
	case <-time.After(controllerTestTimeout):
		t.Fatal("timed out delivering subscribe command to Actor")
	}

	var subscribed subscribeResponse
	select {
	case subscribed = <-response:
	case <-time.After(controllerTestTimeout):
		t.Fatal("timed out waiting for Actor subscribe response")
	}
	select {
	case _, open := <-subscribed.events:
		if open {
			t.Fatal("cancelled subscription channel remained open")
		}
	case <-time.After(controllerTestTimeout):
		t.Fatal("Actor did not close subscription after its context was cancelled")
	}

	activeContext, cancelActive := context.WithCancel(context.Background())
	t.Cleanup(cancelActive)
	_, activeEvents, err := controller.Subscribe(activeContext, 4)
	if err != nil {
		t.Fatalf("Subscribe(active) error = %v, want nil", err)
	}
	if _, submitErr := submitTerminal(t, controller, "still active"); submitErr != nil {
		t.Fatalf("Submit() after cancellation error = %v, want nil", submitErr)
	}
	turn := receiveControlledTurn(t, host)
	turn.emit(t, projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-1"})
	for _, wantKind := range []EventKind{EventSessionStarted, EventTurnStarted} {
		if event := receiveDomainEvent(t, activeEvents); event.Kind != wantKind {
			t.Fatalf("active subscription event = %#v, want kind %q", event, wantKind)
		}
	}
	turn.emit(t, projectclaude.Event{Kind: projectclaude.EventTurnCompleted})
	turn.finish(t, nil)
}

func TestControllerSubscribeCancellationUnblocksFullBuffer(t *testing.T) {
	host := newControlledHost()
	controller := startController(t, host, withRequestIDGenerator(monotonicRequestIDs()))

	activeContext, cancelActive := context.WithCancel(context.Background())
	t.Cleanup(cancelActive)
	_, activeEvents, err := controller.Subscribe(activeContext, 16)
	if err != nil {
		t.Fatalf("Subscribe(active) error = %v, want nil", err)
	}
	if _, submitErr := submitTerminal(t, controller, "initialize"); submitErr != nil {
		t.Fatalf("Submit(initialize) error = %v, want nil", submitErr)
	}
	initialize := receiveControlledTurn(t, host)
	initialize.emit(t, projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-1"})
	initialize.emit(t, projectclaude.Event{Kind: projectclaude.EventTurnCompleted})
	for _, wantKind := range []EventKind{EventSessionStarted, EventTurnStarted, EventTurnCompleted} {
		if event := receiveDomainEvent(t, activeEvents); event.Kind != wantKind {
			t.Fatalf("initialization event = %#v, want kind %q", event, wantKind)
		}
	}
	initialize.finish(t, nil)

	slowContext, cancelSlow := context.WithCancel(context.Background())
	_, slowEvents, err := controller.Subscribe(slowContext, 1)
	if err != nil {
		cancelSlow()
		t.Fatalf("Subscribe(slow) error = %v, want nil", err)
	}
	t.Cleanup(func() {
		cancelSlow()
		deadline := time.After(controllerTestTimeout)
		for {
			select {
			case _, open := <-slowEvents:
				if !open {
					return
				}
			case <-deadline:
				return
			}
		}
	})
	if _, submitErr := submitTerminal(t, controller, "after init"); submitErr != nil {
		t.Fatalf("Submit(after init) error = %v, want nil", submitErr)
	}
	second := receiveControlledTurn(t, host)
	started := receiveDomainEvent(t, activeEvents)
	if started.Kind != EventTurnStarted || started.Seq != 4 {
		t.Fatalf("second Turn start = %#v, want turn_started seq 4", started)
	}

	second.emitAndWaitForDelivery(t, projectclaude.Event{Kind: projectclaude.EventText, Text: "continues"})
	cancelSlow()
	ctx, cancel := controllerContext(t)
	snapshot, snapshotErr := controller.Snapshot(ctx)
	cancel()
	if snapshotErr != nil {
		t.Fatalf("Snapshot() behind cancelled full subscriber error = %v, want nil", snapshotErr)
	}
	if snapshot.Seq != 5 || snapshot.Turn.Message != "continues" {
		t.Fatalf("Snapshot behind cancelled full subscriber = %#v, want seq 5 with text", snapshot)
	}
	textEvent := receiveDomainEvent(t, activeEvents)
	if textEvent.Kind != EventTextDelta || textEvent.Seq != 5 || textEvent.Text != "continues" {
		t.Fatalf("active event behind cancelled full subscriber = %#v, want text_delta seq 5", textEvent)
	}
	if buffered := receiveDomainEvent(t, slowEvents); buffered.Kind != EventTurnStarted || buffered.Seq != 4 {
		t.Fatalf("slow buffered event = %#v, want turn_started seq 4", buffered)
	}
	select {
	case _, open := <-slowEvents:
		if open {
			t.Fatal("cancelled slow subscription received an event after cancellation")
		}
	case <-time.After(controllerTestTimeout):
		t.Fatal("cancelled slow subscription channel did not close")
	}

	second.emit(t, projectclaude.Event{Kind: projectclaude.EventTurnCompleted})
	if completed := receiveDomainEvent(t, activeEvents); completed.Kind != EventTurnCompleted || completed.Seq != 6 {
		t.Fatalf("active completion after slow cancellation = %#v, want turn_completed seq 6", completed)
	}
	second.finish(t, nil)
}

func TestControllerRejectsZeroSubscriptionCapacity(t *testing.T) {
	host := newControlledHost()
	controller := startController(t, host, withRequestIDGenerator(monotonicRequestIDs()))

	snapshot, events, err := controller.Subscribe(context.Background(), 0)
	// Snapshot carries a queuedRequests slice since B2, so the zero value has to
	// be compared structurally rather than with ==.
	if !errors.Is(err, ErrInvalidInput) || events != nil || !reflect.DeepEqual(snapshot, Snapshot{}) {
		t.Fatalf("Subscribe(capacity 0) = %#v, %#v, %v, want zero Snapshot, nil events, ErrInvalidInput", snapshot, events, err)
	}

	validContext, cancelValid := context.WithCancel(context.Background())
	t.Cleanup(cancelValid)
	_, validEvents, err := controller.Subscribe(validContext, 4)
	if err != nil {
		t.Fatalf("valid Subscribe() error = %v, want nil", err)
	}
	if _, submitErr := submitTerminal(t, controller, "first"); submitErr != nil {
		t.Fatalf("Submit() after rejected Subscribe error = %v, want nil", submitErr)
	}
	turn := receiveControlledTurn(t, host)
	turn.emit(t, projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-1"})
	for index, wantKind := range []EventKind{EventSessionStarted, EventTurnStarted} {
		if event := receiveDomainEvent(t, validEvents); event.Kind != wantKind {
			t.Fatalf("valid subscription event %d = %#v, want kind %q", index+1, event, wantKind)
		}
	}
	turn.emit(t, projectclaude.Event{Kind: projectclaude.EventTurnCompleted})
	turn.finish(t, nil)
}

func TestControllerStopCancelsRunningTurn(t *testing.T) {
	closeErr := errors.New("close failed")
	host := newControlledHost()
	host.closeErr = closeErr
	controller, _ := startControllerWithServiceContext(t, host, withRequestIDGenerator(monotonicRequestIDs()))

	subscriptionContext, cancelSubscription := context.WithCancel(context.Background())
	t.Cleanup(cancelSubscription)
	_, events, err := controller.Subscribe(subscriptionContext, 16)
	if err != nil {
		t.Fatalf("Subscribe() error = %v, want nil", err)
	}
	if _, submitErr := submitTerminal(t, controller, "running"); submitErr != nil {
		t.Fatalf("Submit(running) error = %v, want nil", submitErr)
	}
	if _, submitErr := submitTerminal(t, controller, "waiting"); submitErr != nil {
		t.Fatalf("Submit(waiting) error = %v, want nil", submitErr)
	}
	running := receiveControlledTurn(t, host)
	running.emit(t, projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-1"})
	for _, wantKind := range []EventKind{EventSessionStarted, EventTurnStarted} {
		if event := receiveDomainEvent(t, events); event.Kind != wantKind {
			t.Fatalf("pre-stop event = %#v, want kind %q", event, wantKind)
		}
	}

	stopResult := make(chan error, 1)
	go func() {
		stopResult <- controller.Stop(context.Background())
	}()
	waitForTurnCancellation(t, running)
	select {
	case stopErr := <-stopResult:
		if !errors.Is(stopErr, closeErr) {
			t.Fatalf("Stop() error = %v, want errors.Is %v", stopErr, closeErr)
		}
	case <-time.After(controllerTestTimeout):
		t.Fatal("timed out waiting for Stop()")
	}

	remaining := receiveClosedDomainEvents(t, events)
	if len(remaining) != 1 || remaining[0].Kind != EventSessionStopped || remaining[0].SessionStatus != SessionStopped || remaining[0].SessionID != "session-1" {
		t.Fatalf("events after active Stop = %#v, want one final session_stopped for session-1", remaining)
	}

	final := snapshotController(t, controller)
	if final.SessionStatus != SessionStopped || final.SessionError != nil || final.QueueDepth != 0 {
		t.Fatalf("final stopped Snapshot = %#v, want stopped with no Session error and empty queue", final)
	}
	if final.Turn.Status != TurnCancelled || final.Turn.Error != nil || final.Turn.PendingAction != nil {
		t.Fatalf("final stopped Turn = %#v, want cancelled with cleared error and pending action", final.Turn)
	}
	if final.Seq != remaining[0].Seq {
		t.Fatalf("final Snapshot seq = %d, want final event seq %d", final.Seq, remaining[0].Seq)
	}
	_, closeCalls := host.counts()
	if closeCalls != 1 {
		t.Fatalf("Host Close calls = %d, want 1", closeCalls)
	}
}

func TestControllerStopIsIdempotent(t *testing.T) {
	t.Run("sequential", func(t *testing.T) {
		closeErr := errors.New("sequential close failed")
		host := newControlledHost()
		host.closeErr = closeErr
		controller, _ := startControllerWithServiceContext(t, host)

		for call := 1; call <= 3; call++ {
			ctx, cancel := controllerContext(t)
			err := controller.Stop(ctx)
			cancel()
			if !errors.Is(err, closeErr) {
				t.Fatalf("Stop() call %d error = %v, want errors.Is %v", call, err, closeErr)
			}
		}
		_, closeCalls := host.counts()
		if closeCalls != 1 {
			t.Fatalf("Host Close calls = %d, want 1", closeCalls)
		}
	})

	t.Run("concurrent", func(t *testing.T) {
		const stopCount = 16
		closeErr := errors.New("concurrent close failed")
		host := newControlledHost()
		host.closeErr = closeErr
		host.closeEntered = make(chan struct{})
		closeRelease := make(chan struct{})
		host.closeRelease = closeRelease
		var releaseCloseOnce sync.Once
		releaseClose := func() { releaseCloseOnce.Do(func() { close(closeRelease) }) }
		t.Cleanup(releaseClose)
		controller, _ := startControllerWithServiceContext(t, host)

		start := make(chan struct{})
		ready := make(chan struct{}, stopCount)
		results := make(chan error, stopCount)
		for index := 0; index < stopCount; index++ {
			go func() {
				ready <- struct{}{}
				<-start
				ctx, cancel := controllerContext(t)
				defer cancel()
				results <- controller.Stop(ctx)
			}()
		}
		for index := 0; index < stopCount; index++ {
			<-ready
		}
		close(start)
		select {
		case <-host.closeEntered:
		case <-time.After(controllerTestTimeout):
			t.Fatal("timed out waiting for Host Close to start")
		}
		releaseClose()

		for index := 0; index < stopCount; index++ {
			select {
			case err := <-results:
				if !errors.Is(err, closeErr) {
					t.Fatalf("concurrent Stop() error = %v, want errors.Is %v", err, closeErr)
				}
			case <-time.After(controllerTestTimeout):
				t.Fatal("timed out waiting for concurrent Stop()")
			}
		}
		ctx, cancel := controllerContext(t)
		defer cancel()
		if err := controller.Stop(ctx); !errors.Is(err, closeErr) {
			t.Fatalf("Stop() after actor exit error = %v, want errors.Is %v", err, closeErr)
		}
		_, closeCalls := host.counts()
		if closeCalls != 1 {
			t.Fatalf("Host Close calls = %d, want 1", closeCalls)
		}
	})
}

func TestControllerStopAfterFailurePreservesFailed(t *testing.T) {
	closeErr := errors.New("close after failure")
	sessionErr := errors.New("session transport failed")
	host := newControlledHost()
	host.closeErr = closeErr
	controller, _ := startControllerWithServiceContext(t, host, withRequestIDGenerator(monotonicRequestIDs()))

	subscriptionContext, cancelSubscription := context.WithCancel(context.Background())
	t.Cleanup(cancelSubscription)
	_, events, err := controller.Subscribe(subscriptionContext, 16)
	if err != nil {
		t.Fatalf("Subscribe() error = %v, want nil", err)
	}
	if _, submitErr := submitTerminal(t, controller, "failing"); submitErr != nil {
		t.Fatalf("Submit(failing) error = %v, want nil", submitErr)
	}
	if _, submitErr := submitTerminal(t, controller, "discarded"); submitErr != nil {
		t.Fatalf("Submit(discarded) error = %v, want nil", submitErr)
	}
	turn := receiveControlledTurn(t, host)
	turn.emit(t, projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-1"})
	turn.finish(t, scopedTestError{scope: projectclaude.ScopeSession, err: sessionErr})

	for index, wantKind := range []EventKind{EventSessionStarted, EventTurnStarted, EventTurnFailed, EventSessionFailed} {
		if event := receiveDomainEvent(t, events); event.Kind != wantKind {
			t.Fatalf("failure event %d = %#v, want kind %q", index+1, event, wantKind)
		}
	}
	failed := snapshotController(t, controller)
	if failed.SessionStatus != SessionFailed || failed.SessionError == nil || failed.SessionError.Message != sessionErr.Error() || failed.QueueDepth != 0 {
		t.Fatalf("failed Snapshot before Stop = %#v, want preserved root cause and empty queue", failed)
	}

	ctx, cancel := controllerContext(t)
	stopErr := controller.Stop(ctx)
	cancel()
	if !errors.Is(stopErr, closeErr) {
		t.Fatalf("Stop() after failure error = %v, want errors.Is %v", stopErr, closeErr)
	}
	if remaining := receiveClosedDomainEvents(t, events); len(remaining) != 0 {
		t.Fatalf("Stop() after failure published unexpected events: %#v", remaining)
	}

	final := snapshotController(t, controller)
	if final.SessionStatus != SessionFailed || final.SessionError == nil || final.SessionError.Message != sessionErr.Error() || final.QueueDepth != 0 {
		t.Fatalf("final failed Snapshot = %#v, want original Session failure and empty queue", final)
	}
	final.SessionError.Message = "mutated"
	again := snapshotController(t, controller)
	if again.SessionError == nil || again.SessionError.Message != sessionErr.Error() {
		t.Fatalf("cached final Snapshot shares SessionError pointer: %#v", again.SessionError)
	}
	_, closeCalls := host.counts()
	if closeCalls != 1 {
		t.Fatalf("Host Close calls = %d, want 1", closeCalls)
	}
}

func TestControllerServiceContextCancellationUsesStopPath(t *testing.T) {
	host := newControlledHost()
	controller, cancelService := startControllerWithServiceContext(t, host, withRequestIDGenerator(monotonicRequestIDs()))

	subscriptionContext, cancelSubscription := context.WithCancel(context.Background())
	t.Cleanup(cancelSubscription)
	_, events, err := controller.Subscribe(subscriptionContext, 16)
	if err != nil {
		t.Fatalf("Subscribe() error = %v, want nil", err)
	}
	if _, submitErr := submitTerminal(t, controller, "running"); submitErr != nil {
		t.Fatalf("Submit(running) error = %v, want nil", submitErr)
	}
	if _, submitErr := submitTerminal(t, controller, "waiting"); submitErr != nil {
		t.Fatalf("Submit(waiting) error = %v, want nil", submitErr)
	}
	running := receiveControlledTurn(t, host)
	running.emit(t, projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-1"})
	for _, wantKind := range []EventKind{EventSessionStarted, EventTurnStarted} {
		if event := receiveDomainEvent(t, events); event.Kind != wantKind {
			t.Fatalf("pre-cancel event = %#v, want kind %q", event, wantKind)
		}
	}

	cancelService()
	waitForTurnCancellation(t, running)
	remaining := receiveClosedDomainEvents(t, events)
	if len(remaining) != 1 || remaining[0].Kind != EventSessionStopped || remaining[0].SessionStatus != SessionStopped {
		t.Fatalf("events after service cancellation = %#v, want one final session_stopped", remaining)
	}
	final := snapshotController(t, controller)
	if final.SessionStatus != SessionStopped || final.Turn.Status != TurnCancelled || final.QueueDepth != 0 || final.Turn.Error != nil || final.Turn.PendingAction != nil {
		t.Fatalf("final Snapshot after service cancellation = %#v, want stopped/cancelled with cleared queue", final)
	}
	ctx, cancel := controllerContext(t)
	defer cancel()
	if err := controller.Stop(ctx); err != nil {
		t.Fatalf("Stop() after service cancellation error = %v, want cached nil", err)
	}
	_, closeCalls := host.counts()
	if closeCalls != 1 {
		t.Fatalf("Host Close calls = %d, want 1", closeCalls)
	}
}

func TestControllerServiceContextCancellationDuringConnectUsesStopPath(t *testing.T) {
	closeErr := errors.New("close during connect cancellation")
	host := newControlledHost()
	host.closeErr = closeErr
	host.connectEntered = make(chan struct{})
	connectRelease := make(chan struct{})
	host.connectRelease = connectRelease
	controller := NewController(host)
	serviceContext, cancelService := context.WithCancel(context.Background())
	startResult := make(chan error, 1)
	go func() {
		startResult <- controller.Start(serviceContext)
	}()

	var releaseConnectOnce sync.Once
	releaseConnect := func() { releaseConnectOnce.Do(func() { close(connectRelease) }) }
	t.Cleanup(func() {
		releaseConnect()
		cancelService()
		ctx, cancel := controllerContext(t)
		defer cancel()
		_ = controller.Stop(ctx)
	})
	select {
	case <-host.connectEntered:
	case <-time.After(controllerTestTimeout):
		t.Fatal("timed out waiting for Host Connect to start")
	}

	subscriptionContext, cancelSubscription := context.WithCancel(context.Background())
	t.Cleanup(cancelSubscription)
	initial, events, err := controller.Subscribe(subscriptionContext, 4)
	if err != nil {
		t.Fatalf("Subscribe() during Connect error = %v, want nil", err)
	}
	if initial.SessionStatus != SessionStarting || initial.Turn.Status != TurnIdle || initial.Seq != 0 {
		t.Fatalf("Snapshot during Connect = %#v, want starting/idle seq 0", initial)
	}

	cancelService()
	select {
	case err := <-startResult:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Start() after service cancellation error = %v, want context.Canceled", err)
		}
	case <-time.After(controllerTestTimeout):
		t.Fatal("timed out waiting for Start() after service cancellation")
	}

	remaining := receiveClosedDomainEvents(t, events)
	if len(remaining) != 1 || remaining[0].Kind != EventSessionStopped || remaining[0].SessionStatus != SessionStopped || remaining[0].SessionID != "" || remaining[0].Seq != 1 {
		t.Fatalf("events after service cancellation during Connect = %#v, want one local session_stopped seq 1", remaining)
	}
	final := snapshotController(t, controller)
	if final.SessionStatus != SessionStopped || final.SessionError != nil || final.Turn.Status != TurnIdle || final.QueueDepth != 0 || final.Seq != 1 {
		t.Fatalf("final Snapshot after Connect cancellation = %#v, want stopped/idle seq 1", final)
	}
	ctx, cancel := controllerContext(t)
	defer cancel()
	if err := controller.Stop(ctx); !errors.Is(err, closeErr) {
		t.Fatalf("Stop() after Connect cancellation error = %v, want cached %v", err, closeErr)
	}
	_, closeCalls := host.counts()
	if closeCalls != 1 {
		t.Fatalf("Host Close calls = %d, want 1", closeCalls)
	}
}

func TestControllerStopCallerCancellationDoesNotPreventConvergence(t *testing.T) {
	host := newControlledHost()
	host.closeEntered = make(chan struct{})
	closeRelease := make(chan struct{})
	host.closeRelease = closeRelease
	var releaseCloseOnce sync.Once
	releaseClose := func() { releaseCloseOnce.Do(func() { close(closeRelease) }) }
	t.Cleanup(releaseClose)
	controller, _ := startControllerWithServiceContext(t, host)

	subscriptionContext, cancelSubscription := context.WithCancel(context.Background())
	t.Cleanup(cancelSubscription)
	_, events, err := controller.Subscribe(subscriptionContext, 2)
	if err != nil {
		t.Fatalf("Subscribe() error = %v, want nil", err)
	}
	stopContext, cancelStop := context.WithCancel(context.Background())
	stopResult := make(chan error, 1)
	go func() {
		stopResult <- controller.Stop(stopContext)
	}()
	select {
	case <-host.closeEntered:
	case <-time.After(controllerTestTimeout):
		t.Fatal("timed out waiting for Stop command to reach Host Close")
	}
	cancelStop()
	select {
	case err := <-stopResult:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Stop() after caller cancellation error = %v, want context.Canceled", err)
		}
	case <-time.After(controllerTestTimeout):
		t.Fatal("timed out waiting for canceled Stop caller")
	}

	releaseClose()
	remaining := receiveClosedDomainEvents(t, events)
	if len(remaining) != 1 || remaining[0].Kind != EventSessionStopped {
		t.Fatalf("events after canceled Stop caller = %#v, want final session_stopped", remaining)
	}
	final := snapshotController(t, controller)
	if final.SessionStatus != SessionStopped {
		t.Fatalf("final Snapshot status = %q, want stopped", final.SessionStatus)
	}
	ctx, cancel := controllerContext(t)
	defer cancel()
	if err := controller.Stop(ctx); err != nil {
		t.Fatalf("Stop() after convergence error = %v, want nil", err)
	}
}

func TestControllerWaitIdleWaitsForWorkerResult(t *testing.T) {
	host := newControlledHost()
	generator, _ := sequentialRequestIDs("request-01")
	controller := startController(t, host, withRequestIDGenerator(generator))

	requestID, err := submitTerminal(t, controller, "first")
	if err != nil || requestID != "request-01" {
		t.Fatalf("Submit() = %q, %v, want request-01, nil", requestID, err)
	}
	turn := receiveControlledTurn(t, host)
	turn.emitAndWaitForDelivery(t, projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-1"})
	turn.emitAndWaitForDelivery(t, projectclaude.Event{Kind: projectclaude.EventTurnCompleted})
	terminal := waitForSnapshot(t, controller, func(snapshot Snapshot) bool {
		return snapshot.Turn.Status == TurnCompleted
	})

	timedOutContext, cancelTimedOut := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancelTimedOut()
	if _, err := controller.WaitIdle(timedOutContext); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WaitIdle() before Worker result error = %v, want deadline exceeded", err)
	}

	waitContext, cancelWait := controllerContext(t)
	defer cancelWait()
	waitResult := make(chan snapshotResponse, 1)
	go func() {
		snapshot, err := controller.WaitIdle(waitContext)
		waitResult <- snapshotResponse{snapshot: snapshot, err: err}
	}()
	select {
	case result := <-waitResult:
		t.Fatalf("WaitIdle() returned before Worker result: %#v", result)
	case <-time.After(25 * time.Millisecond):
	}

	turn.finish(t, nil)
	select {
	case result := <-waitResult:
		if result.err != nil {
			t.Fatalf("WaitIdle() error = %v, want nil", result.err)
		}
		if result.snapshot.SessionStatus != SessionReady || result.snapshot.Turn.Status != TurnCompleted || result.snapshot.QueueDepth != 0 {
			t.Fatalf("WaitIdle() Snapshot = %#v, want ready/completed queue 0", result.snapshot)
		}
		if result.snapshot.Seq != terminal.Seq {
			t.Fatalf("WaitIdle() changed seq from %d to %d", terminal.Seq, result.snapshot.Seq)
		}
	case <-time.After(controllerTestTimeout):
		t.Fatal("timed out waiting for WaitIdle() after Worker result")
	}
}

func TestControllerWaitIdleReturnsFailedAfterSessionResult(t *testing.T) {
	host := newControlledHost()
	generator, _ := sequentialRequestIDs("request-01")
	controller := startController(t, host, withRequestIDGenerator(generator))

	if _, err := submitTerminal(t, controller, "first"); err != nil {
		t.Fatalf("Submit() error = %v, want nil", err)
	}
	turn := receiveControlledTurn(t, host)
	turn.emitAndWaitForDelivery(t, projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-1"})
	turn.emitAndWaitForDelivery(t, projectclaude.Event{Kind: projectclaude.EventTurnFailed, Text: "recoverable Adapter failure"})
	waitForSnapshot(t, controller, func(snapshot Snapshot) bool {
		return snapshot.Turn.Status == TurnFailed
	})

	waitContext, cancelWait := controllerContext(t)
	defer cancelWait()
	waitResult := make(chan snapshotResponse, 1)
	go func() {
		snapshot, err := controller.WaitIdle(waitContext)
		waitResult <- snapshotResponse{snapshot: snapshot, err: err}
	}()
	select {
	case result := <-waitResult:
		t.Fatalf("WaitIdle() returned before Session result: %#v", result)
	case <-time.After(25 * time.Millisecond):
	}

	sessionErr := errors.New("session transport failed")
	turn.finish(t, scopedTestError{scope: projectclaude.ScopeSession, err: sessionErr})
	select {
	case result := <-waitResult:
		if result.err != nil {
			t.Fatalf("WaitIdle() error = %v, want nil", result.err)
		}
		if result.snapshot.SessionStatus != SessionFailed || result.snapshot.SessionError == nil || result.snapshot.SessionError.Message != sessionErr.Error() {
			t.Fatalf("WaitIdle() Snapshot = %#v, want failed with Session root cause", result.snapshot)
		}
		if result.snapshot.Turn.Status != TurnFailed || result.snapshot.Turn.Error == nil || result.snapshot.Turn.Error.Message != "recoverable Adapter failure" {
			t.Fatalf("WaitIdle() Turn = %#v, want preserved Adapter Turn failure", result.snapshot.Turn)
		}
	case <-time.After(controllerTestTimeout):
		t.Fatal("timed out waiting for failed WaitIdle() Snapshot")
	}
}

func TestControllerWaitIdleReturnsCachedFinalSnapshot(t *testing.T) {
	host := newControlledHost()
	controller := startController(t, host)
	stopContext, cancelStop := controllerContext(t)
	if err := controller.Stop(stopContext); err != nil {
		cancelStop()
		t.Fatalf("Stop() error = %v, want nil", err)
	}
	cancelStop()

	waitContext, cancelWait := controllerContext(t)
	defer cancelWait()
	snapshot, err := controller.WaitIdle(waitContext)
	if err != nil {
		t.Fatalf("WaitIdle() after Stop error = %v, want nil", err)
	}
	if snapshot.SessionStatus != SessionStopped || snapshot.Turn.Status != TurnIdle || snapshot.QueueDepth != 0 {
		t.Fatalf("WaitIdle() cached Snapshot = %#v, want stopped/idle queue 0", snapshot)
	}
}

// ---- A5.2 预留 session ID 与 confirmed 状态（设计 §4/§6）----

func TestControllerReservedSessionIDSubmitBeforeConfirm(t *testing.T) {
	host := newControlledHost()
	generator, _ := sequentialRequestIDs("request-01")
	controller := startController(t, host,
		withRequestIDGenerator(generator),
		WithReservedSessionID("session-reserved"),
	)

	subscriptionContext, cancelSubscription := context.WithCancel(context.Background())
	t.Cleanup(cancelSubscription)
	initial, events, err := controller.Subscribe(subscriptionContext, 16)
	if err != nil {
		t.Fatalf("Subscribe() error = %v, want nil", err)
	}
	// 预留 ID 从订阅起可见：starting/idle/seq 0（设计 §9 snapshot）。
	if initial.SessionID != "session-reserved" || initial.SessionStatus != SessionStarting ||
		initial.Turn.Status != TurnIdle || initial.Seq != 0 {
		t.Fatalf("initial Snapshot = %#v, want reserved ID/starting/idle/seq 0", initial)
	}

	// 未 confirmed 时手机输入（携带匹配 sessionId）与终端输入都可 Submit。
	ctx, cancel := controllerContext(t)
	defer cancel()
	mobileRequestID, err := controller.Submit(ctx, Input{
		SessionID: "session-reserved", Source: InputMobile, Text: "from phone",
	})
	if err != nil || mobileRequestID != "request-01" {
		t.Fatalf("Submit(mobile) = %q, %v, want request-01, nil", mobileRequestID, err)
	}

	// 输入成为 current 并交给 Worker，但 turn_started 暂缓（设计 §6）。
	first := receiveControlledTurn(t, host)
	if first.prompt != "from phone" {
		t.Fatalf("first Host prompt = %q, want from phone", first.prompt)
	}
	assertNoDomainEvent(t, events)

	// init 前的 Claude 文本继续缓冲。
	first.emit(t, projectclaude.Event{Kind: projectclaude.EventText, Text: "pre"})
	assertNoDomainEvent(t, events)

	// init 确认后按 session_started → turn_started → text_delta 顺序发布。
	first.emit(t, projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-reserved"})
	sessionStarted := receiveDomainEvent(t, events)
	if sessionStarted.Kind != EventSessionStarted || sessionStarted.SessionID != "session-reserved" {
		t.Fatalf("session_started = %#v, want reserved ID", sessionStarted)
	}
	turnStarted := receiveDomainEvent(t, events)
	if turnStarted.Kind != EventTurnStarted || turnStarted.RequestID != "request-01" ||
		turnStarted.Source != InputMobile || turnStarted.Text != "from phone" {
		t.Fatalf("turn_started = %#v, want mobile request-01 from phone", turnStarted)
	}
	delta := receiveDomainEvent(t, events)
	if delta.Kind != EventTextDelta || delta.Text != "pre" {
		t.Fatalf("text_delta = %#v, want buffered pre", delta)
	}

	first.emit(t, projectclaude.Event{Kind: projectclaude.EventTurnCompleted})
	first.finish(t, nil)
	if completed := receiveDomainEvent(t, events); completed.Kind != EventTurnCompleted {
		t.Fatalf("turn_completed = %#v", completed)
	}

	snapshot := snapshotController(t, controller)
	if snapshot.SessionID != "session-reserved" || snapshot.SessionStatus != SessionReady {
		t.Fatalf("Snapshot = %#v, want reserved ID/ready", snapshot)
	}
}

func TestControllerReservedSessionIDMismatchFailsSession(t *testing.T) {
	host := newControlledHost()
	generator, _ := sequentialRequestIDs("request-01", "request-02")
	controller := startController(t, host,
		withRequestIDGenerator(generator),
		WithReservedSessionID("session-reserved"),
	)

	subscriptionContext, cancelSubscription := context.WithCancel(context.Background())
	t.Cleanup(cancelSubscription)
	_, events, err := controller.Subscribe(subscriptionContext, 16)
	if err != nil {
		t.Fatalf("Subscribe() error = %v, want nil", err)
	}
	if _, submitErr := submitTerminal(t, controller, "first"); submitErr != nil {
		t.Fatalf("Submit(first) error = %v", submitErr)
	}
	// 队列里再放一条，验证 mismatch 时清空。
	if _, submitErr := submitTerminal(t, controller, "second"); submitErr != nil {
		t.Fatalf("Submit(second) error = %v", submitErr)
	}

	turn := receiveControlledTurn(t, host)
	// init 返回不同 ID：Session 级失败，队列清空（设计 §11）。failSession
	// 既有顺序：先结算当前 turn_failed，再发布 session_failed。
	turn.emit(t, projectclaude.Event{Kind: projectclaude.EventSessionStarted, SessionID: "session-other"})
	if turnFailed := receiveDomainEvent(t, events); turnFailed.Kind != EventTurnFailed {
		t.Fatalf("turn event = %#v, want turn_failed", turnFailed)
	}
	failed := receiveDomainEvent(t, events)
	if failed.Kind != EventSessionFailed || failed.SessionID != "session-reserved" {
		t.Fatalf("session_failed = %#v, want reserved ID failure", failed)
	}
	turn.finish(t, nil)

	snapshot := snapshotController(t, controller)
	if snapshot.SessionStatus != SessionFailed || snapshot.QueueDepth != 0 {
		t.Fatalf("Snapshot = %#v, want failed and empty queue", snapshot)
	}
	if _, submitErr := submitTerminal(t, controller, "after"); !errors.Is(submitErr, ErrSessionUnavailable) {
		t.Fatalf("Submit(after failure) error = %v, want ErrSessionUnavailable", submitErr)
	}
}

func TestControllerReservedSessionIDRejectsMismatchedMobileInput(t *testing.T) {
	host := newControlledHost()
	controller := startController(t, host, WithReservedSessionID("session-reserved"))

	ctx, cancel := controllerContext(t)
	defer cancel()
	if _, err := controller.Submit(ctx, Input{
		SessionID: "session-other", Source: InputMobile, Text: "wrong target",
	}); !errors.Is(err, ErrSessionMismatch) {
		t.Fatalf("Submit(mismatched mobile) error = %v, want ErrSessionMismatch", err)
	}
}

// activeSession is a Controller that already runs one confirmed Turn, which is
// the only state in which a pending action can exist.
type activeSession struct {
	controller *Controller
	host       *controlledHost
	turn       *controlledTurn
	events     <-chan Event
	requestID  string
}

func startActiveSession(t *testing.T, options ...Option) *activeSession {
	t.Helper()
	host := newControlledHost()
	controller := startController(t, host, append([]Option{withRequestIDGenerator(monotonicRequestIDs())}, options...)...)

	// The subscription must outlive every wait below, so it cannot borrow the
	// short-lived per-call context helpers.
	subscriptionContext, cancelSubscription := context.WithCancel(context.Background())
	t.Cleanup(cancelSubscription)
	_, events, err := controller.Subscribe(subscriptionContext, 64)
	if err != nil {
		t.Fatalf("Subscribe() error = %v, want nil", err)
	}

	requestID, err := submitTerminal(t, controller, "run the tool")
	if err != nil {
		t.Fatalf("Submit() error = %v, want nil", err)
	}
	turn := receiveControlledTurn(t, host)
	turn.emitAndWaitForDelivery(t, projectclaude.Event{
		Kind: projectclaude.EventSessionStarted, SessionID: "session-1",
	})
	if event := receiveDomainEvent(t, events); event.Kind != EventSessionStarted {
		t.Fatalf("first event = %q, want session_started", event.Kind)
	}
	if event := receiveDomainEvent(t, events); event.Kind != EventTurnStarted {
		t.Fatalf("second event = %q, want turn_started", event.Kind)
	}
	return &activeSession{controller: controller, host: host, turn: turn, events: events, requestID: requestID}
}

func approvalSpec(toolName, preview string, allowSession bool) projectclaude.ActionSpec {
	return projectclaude.ActionSpec{
		Kind:                  projectclaude.ActionToolApproval,
		ToolName:              toolName,
		Preview:               preview,
		AllowSessionAvailable: allowSession,
	}
}

func questionSpec(questions ...projectclaude.Question) projectclaude.ActionSpec {
	return projectclaude.ActionSpec{
		Kind:      projectclaude.ActionAskUserQuestion,
		ToolName:  AskUserQuestionToolName,
		Questions: questions,
	}
}

func choiceQuestion(header, prompt string, multiSelect bool, labels ...string) projectclaude.Question {
	options := make([]projectclaude.QuestionOption, 0, len(labels))
	for _, label := range labels {
		options = append(options, projectclaude.QuestionOption{Label: label})
	}
	return projectclaude.Question{
		Header:      header,
		Question:    prompt,
		InputType:   projectclaude.QuestionChoice,
		MultiSelect: multiSelect,
		Options:     options,
	}
}

func awaitDecision(t *testing.T, request *projectclaude.ActionRequest) projectclaude.ActionDecision {
	t.Helper()
	select {
	case decision := <-request.Decision():
		return decision
	case <-time.After(controllerTestTimeout):
		t.Fatal("timed out waiting for the Controller action decision")
		return projectclaude.ActionDecision{}
	}
}

func assertNoDecision(t *testing.T, request *projectclaude.ActionRequest) {
	t.Helper()
	select {
	case decision := <-request.Decision():
		t.Fatalf("unexpected action decision: %+v", decision)
	case <-time.After(25 * time.Millisecond):
	}
}

func awaitActionRequired(t *testing.T, session *activeSession, spec projectclaude.ActionSpec) (*projectclaude.ActionRequest, Event) {
	t.Helper()
	request := session.host.requestAction(t, spec)
	event := receiveDomainEvent(t, session.events)
	if event.Kind != EventActionRequired || event.PendingAction == nil {
		t.Fatalf("event = %#v, want action_required carrying a pending action", event)
	}
	return request, event
}

func resolveAction(t *testing.T, controller *Controller, actionID string, decision ActionDecision, answers []QuestionAnswer) error {
	t.Helper()
	ctx, cancel := controllerContext(t)
	defer cancel()
	return controller.ResolveAction(ctx, actionID, decision, answers)
}

func cancelTurn(t *testing.T, controller *Controller, requestID string) error {
	t.Helper()
	ctx, cancel := controllerContext(t)
	defer cancel()
	return controller.CancelTurn(ctx, requestID)
}

func cancelQueued(t *testing.T, controller *Controller, requestID string) error {
	t.Helper()
	ctx, cancel := controllerContext(t)
	defer cancel()
	return controller.CancelQueued(ctx, requestID)
}

func TestControllerPublishesActionRequired(t *testing.T) {
	session := startActiveSession(t)
	_, event := awaitActionRequired(t, session, approvalSpec("Bash", "ls -la /tmp", true))

	if event.RequestID != session.requestID || event.TurnStatus != TurnWaiting {
		t.Fatalf("event = %#v, want the running request waiting", event)
	}
	pending := event.PendingAction
	if pending.ActionID == "" {
		t.Fatal("action_required must carry a Controller-assigned actionId")
	}
	if pending.RequestID != session.requestID || pending.Kind != ActionToolApproval {
		t.Fatalf("PendingAction = %#v, want a tool approval on the running Turn", pending)
	}
	if pending.ToolName != "Bash" || pending.Preview != "ls -la /tmp" || !pending.AllowSessionAvailable {
		t.Fatalf("PendingAction = %#v, want the Adapter display fields", pending)
	}

	// The waiting status and the pending action must become visible together.
	snapshot := snapshotController(t, session.controller)
	if snapshot.Turn.Status != TurnWaiting || snapshot.Turn.PendingAction == nil {
		t.Fatalf("Snapshot.Turn = %#v, want waiting with a pending action", snapshot.Turn)
	}
	if snapshot.Turn.PendingAction.ActionID != pending.ActionID {
		t.Fatalf("Snapshot action ID = %q, want the published %q", snapshot.Turn.PendingAction.ActionID, pending.ActionID)
	}
}

func TestControllerResolveActionAllowOnce(t *testing.T) {
	session := startActiveSession(t)
	request, event := awaitActionRequired(t, session, approvalSpec("Bash", "ls", true))
	actionID := event.PendingAction.ActionID

	if err := resolveAction(t, session.controller, actionID, DecisionAllowOnce, nil); err != nil {
		t.Fatalf("ResolveAction() error = %v, want nil", err)
	}
	if decision := awaitDecision(t, request); decision.Kind != projectclaude.DecisionAllowOnce {
		t.Fatalf("decision = %+v, want allow_once", decision)
	}

	resolved := receiveDomainEvent(t, session.events)
	if resolved.Kind != EventActionResolved || resolved.RequestID != session.requestID {
		t.Fatalf("event = %#v, want action_resolved for the running Turn", resolved)
	}
	if resolved.TurnStatus != TurnRunning {
		t.Fatalf("action_resolved TurnStatus = %q, want running again", resolved.TurnStatus)
	}
	if resolved.PendingAction != nil {
		t.Fatalf("action_resolved must not carry a pending action: %#v", resolved.PendingAction)
	}

	snapshot := snapshotController(t, session.controller)
	if snapshot.Turn.Status != TurnRunning || snapshot.Turn.PendingAction != nil {
		t.Fatalf("Snapshot.Turn = %#v, want running without a pending action", snapshot.Turn)
	}
	if err := resolveAction(t, session.controller, actionID, DecisionDeny, nil); !errors.Is(err, ErrActionNotPending) {
		t.Fatalf("second ResolveAction() error = %v, want ErrActionNotPending", err)
	}
}

func TestControllerResolveActionRejectsUnknownAction(t *testing.T) {
	session := startActiveSession(t)
	request, event := awaitActionRequired(t, session, approvalSpec("Bash", "ls", true))

	if err := resolveAction(t, session.controller, event.PendingAction.ActionID+"-stale", DecisionAllowOnce, nil); !errors.Is(err, ErrActionNotPending) {
		t.Fatalf("ResolveAction(stale) error = %v, want ErrActionNotPending", err)
	}
	assertNoDecision(t, request)
	assertNoDomainEvent(t, session.events)
}

func TestControllerResolveActionRejectsAllowSessionWithoutSuggestions(t *testing.T) {
	session := startActiveSession(t)
	request, event := awaitActionRequired(t, session, approvalSpec("Bash", "ls", false))

	if err := resolveAction(t, session.controller, event.PendingAction.ActionID, DecisionAllowSession, nil); !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("ResolveAction(allow_session) error = %v, want ErrInvalidAction", err)
	}
	assertNoDecision(t, request)
	if snapshot := snapshotController(t, session.controller); snapshot.Turn.PendingAction == nil {
		t.Fatal("a rejected resolution must leave the action pending")
	}
}

func TestControllerResolveActionAnswersQuestions(t *testing.T) {
	session := startActiveSession(t, WithServiceConfig(ServiceConfig{AskUserTextEnabled: false}))
	request, event := awaitActionRequired(t, session, questionSpec(
		choiceQuestion("Framework", "Which framework?", false, "React", "Vue"),
		choiceQuestion("Targets", "Which targets?", true, "iOS", "Android"),
	))

	pending := event.PendingAction
	if len(pending.Questions) != 2 {
		t.Fatalf("Questions = %#v, want two", pending.Questions)
	}
	for index, question := range pending.Questions {
		if question.QuestionID == "" {
			t.Fatalf("Questions[%d] has no Controller-assigned questionId", index)
		}
	}

	// Answers arrive keyed by questionId in any order; the Adapter decision must
	// still follow the question order the SDK asked in.
	err := resolveAction(t, session.controller, pending.ActionID, "", []QuestionAnswer{
		{QuestionID: pending.Questions[1].QuestionID, Values: []string{"iOS", "Android"}},
		{QuestionID: pending.Questions[0].QuestionID, Values: []string{"React"}},
	})
	if err != nil {
		t.Fatalf("ResolveAction(answers) error = %v, want nil", err)
	}
	decision := awaitDecision(t, request)
	if decision.Kind != projectclaude.DecisionAnswer {
		t.Fatalf("decision kind = %q, want answer", decision.Kind)
	}
	wantAnswers := [][]string{{"React"}, {"iOS", "Android"}}
	if !reflect.DeepEqual(decision.Answers, wantAnswers) {
		t.Fatalf("decision.Answers = %#v, want %#v in question order", decision.Answers, wantAnswers)
	}
}

func TestControllerResolveActionRejectsMismatchedShapes(t *testing.T) {
	approval := startActiveSession(t)
	approvalRequest, approvalEvent := awaitActionRequired(t, approval, approvalSpec("Bash", "ls", true))
	answers := []QuestionAnswer{{QuestionID: "q1", Values: []string{"React"}}}
	if err := resolveAction(t, approval.controller, approvalEvent.PendingAction.ActionID, "", answers); !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("ResolveAction(answers for approval) error = %v, want ErrInvalidAction", err)
	}
	assertNoDecision(t, approvalRequest)

	question := startActiveSession(t)
	questionRequest, questionEvent := awaitActionRequired(t, question, questionSpec(
		choiceQuestion("Framework", "Which framework?", false, "React", "Vue"),
		choiceQuestion("Targets", "Which targets?", false, "iOS", "Android"),
	))
	pending := questionEvent.PendingAction
	if err := resolveAction(t, question.controller, pending.ActionID, DecisionAllowOnce, nil); !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("ResolveAction(approval for question) error = %v, want ErrInvalidAction", err)
	}
	partial := []QuestionAnswer{{QuestionID: pending.Questions[0].QuestionID, Values: []string{"React"}}}
	if err := resolveAction(t, question.controller, pending.ActionID, "", partial); !errors.Is(err, ErrInvalidAnswer) {
		t.Fatalf("ResolveAction(partial answers) error = %v, want ErrInvalidAnswer", err)
	}
	unknown := []QuestionAnswer{
		{QuestionID: pending.Questions[0].QuestionID, Values: []string{"React"}},
		{QuestionID: "q-not-here", Values: []string{"iOS"}},
	}
	if err := resolveAction(t, question.controller, pending.ActionID, "", unknown); !errors.Is(err, ErrInvalidAnswer) {
		t.Fatalf("ResolveAction(unknown question) error = %v, want ErrInvalidAnswer", err)
	}
	assertNoDecision(t, questionRequest)
}

func TestControllerRejectsConcurrentAction(t *testing.T) {
	session := startActiveSession(t)
	first, event := awaitActionRequired(t, session, approvalSpec("Bash", "ls", true))

	second := session.host.requestAction(t, approvalSpec("Write", "/etc/hosts", true))
	decision := awaitDecision(t, second)
	if decision.Kind != projectclaude.DecisionDeny || decision.Interrupt {
		t.Fatalf("decision = %+v, want a non-interrupting deny", decision)
	}

	notice := receiveDomainEvent(t, session.events)
	if notice.Kind != EventUnsupported || notice.Code != UnsupportedConcurrentAction {
		t.Fatalf("event = %#v, want unsupported/concurrent_action", notice)
	}
	if notice.PendingAction != nil || notice.Block != nil {
		t.Fatalf("unsupported notice must stay structurally empty: %#v", notice)
	}

	snapshot := snapshotController(t, session.controller)
	if snapshot.Turn.PendingAction == nil || snapshot.Turn.PendingAction.ActionID != event.PendingAction.ActionID {
		t.Fatalf("Snapshot.Turn = %#v, want the first action still pending", snapshot.Turn)
	}
	assertNoDecision(t, first)
}

func TestControllerRejectsFreeTextQuestionWhenDisabled(t *testing.T) {
	freeText := projectclaude.Question{
		Header: "Name", Question: "What should it be called?", InputType: projectclaude.QuestionText,
	}

	disabled := startActiveSession(t)
	request := disabled.host.requestAction(t, questionSpec(freeText))
	if decision := awaitDecision(t, request); decision.Kind != projectclaude.DecisionDeny || decision.Interrupt {
		t.Fatalf("decision = %+v, want a non-interrupting deny", decision)
	}
	notice := receiveDomainEvent(t, disabled.events)
	if notice.Kind != EventUnsupported || notice.Code != UnsupportedTextDisabled {
		t.Fatalf("event = %#v, want unsupported/text_input_disabled", notice)
	}
	if snapshot := snapshotController(t, disabled.controller); snapshot.Turn.PendingAction != nil {
		t.Fatalf("Snapshot.Turn = %#v, want no pending action", snapshot.Turn)
	}

	enabled := startActiveSession(t, WithServiceConfig(ServiceConfig{AskUserTextEnabled: true}))
	allowed, event := awaitActionRequired(t, enabled, questionSpec(freeText))
	if len(event.PendingAction.Questions) != 1 || event.PendingAction.Questions[0].InputType != QuestionText {
		t.Fatalf("PendingAction = %#v, want one free-text question", event.PendingAction)
	}
	err := resolveAction(t, enabled.controller, event.PendingAction.ActionID, "", []QuestionAnswer{
		{QuestionID: event.PendingAction.Questions[0].QuestionID, Values: []string{"Aurora"}},
	})
	if err != nil {
		t.Fatalf("ResolveAction(free text) error = %v, want nil", err)
	}
	if decision := awaitDecision(t, allowed); !reflect.DeepEqual(decision.Answers, [][]string{{"Aurora"}}) {
		t.Fatalf("decision.Answers = %#v, want the free-text answer", decision.Answers)
	}
}

func TestControllerRejectsMalformedQuestionAction(t *testing.T) {
	session := startActiveSession(t)
	// The Adapter reports an empty question list when it cannot render the SDK
	// payload safely; such an action can never be answered.
	request := session.host.requestAction(t, questionSpec())
	if decision := awaitDecision(t, request); decision.Kind != projectclaude.DecisionDeny {
		t.Fatalf("decision = %+v, want deny", decision)
	}
	notice := receiveDomainEvent(t, session.events)
	if notice.Kind != EventUnsupported || notice.Code != UnsupportedUnknownBlock {
		t.Fatalf("event = %#v, want unsupported/unknown_block", notice)
	}
}

func TestControllerSubmitDuringPendingActionDoesNotDeadlock(t *testing.T) {
	session := startActiveSession(t)
	request, event := awaitActionRequired(t, session, approvalSpec("Bash", "ls", true))

	// A blocked permission callback must not stall the actor: queueing, snapshots
	// and the later resolution all have to keep working (B2 §3.4).
	queuedID, err := submitTerminal(t, session.controller, "next prompt")
	if err != nil {
		t.Fatalf("Submit(during pending action) error = %v, want nil", err)
	}
	snapshot := snapshotController(t, session.controller)
	if snapshot.QueueDepth != 1 || snapshot.Turn.Status != TurnWaiting {
		t.Fatalf("Snapshot = %#v, want one queued input while waiting", snapshot)
	}
	if len(snapshot.QueuedRequests) != 1 || snapshot.QueuedRequests[0].RequestID != queuedID {
		t.Fatalf("QueuedRequests = %#v, want the newly queued request", snapshot.QueuedRequests)
	}

	if err := resolveAction(t, session.controller, event.PendingAction.ActionID, DecisionDeny, nil); err != nil {
		t.Fatalf("ResolveAction() error = %v, want nil", err)
	}
	if decision := awaitDecision(t, request); decision.Kind != projectclaude.DecisionDeny || decision.Interrupt {
		t.Fatalf("decision = %+v, want a non-interrupting deny", decision)
	}
	if session.host.interruptCount() != 0 {
		t.Fatalf("Interrupt called %d times, want 0 for a plain deny", session.host.interruptCount())
	}
}

func TestControllerCancelTurnWithPendingActionSkipsInterrupt(t *testing.T) {
	session := startActiveSession(t)
	request, _ := awaitActionRequired(t, session, approvalSpec("Bash", "ls", true))

	if err := cancelTurn(t, session.controller, session.requestID); err != nil {
		t.Fatalf("CancelTurn() error = %v, want nil", err)
	}
	// §3.3 case 1: the pending callback is released with Deny{Interrupt:true};
	// calling SDK Interrupt first would deadlock on the blocked read loop.
	decision := awaitDecision(t, request)
	if decision.Kind != projectclaude.DecisionDeny || !decision.Interrupt {
		t.Fatalf("decision = %+v, want an interrupting deny", decision)
	}
	if session.host.interruptCount() != 0 {
		t.Fatalf("Interrupt called %d times, want 0 while an action was pending", session.host.interruptCount())
	}

	cancelled := receiveDomainEvent(t, session.events)
	if cancelled.Kind != EventTurnCompleted || cancelled.TurnStatus != TurnCancelled {
		t.Fatalf("event = %#v, want a cancelled terminal Turn event", cancelled)
	}
	if cancelled.RequestID != session.requestID {
		t.Fatalf("cancelled event request = %q, want %q", cancelled.RequestID, session.requestID)
	}
	waitForTurnCancellation(t, session.turn)

	snapshot := waitForSnapshot(t, session.controller, func(snapshot Snapshot) bool {
		return snapshot.Turn.PendingAction == nil
	})
	if snapshot.Turn.Status != TurnCancelled || snapshot.SessionStatus != SessionReady {
		t.Fatalf("Snapshot = %#v, want a cancelled Turn on a ready Session", snapshot)
	}
	if err := cancelTurn(t, session.controller, session.requestID); !errors.Is(err, ErrTurnNotCancellable) {
		t.Fatalf("second CancelTurn() error = %v, want ErrTurnNotCancellable", err)
	}
}

func TestControllerCancelTurnWithoutPendingActionInterrupts(t *testing.T) {
	session := startActiveSession(t)

	if err := cancelTurn(t, session.controller, session.requestID); err != nil {
		t.Fatalf("CancelTurn() error = %v, want nil", err)
	}
	// §3.3 case 2: without a pending action nothing blocks the read loop, so the
	// SDK Interrupt is the only way to stop Claude mid-Turn.
	session.host.awaitInterrupt(t)
	cancelled := receiveDomainEvent(t, session.events)
	if cancelled.Kind != EventTurnCompleted || cancelled.TurnStatus != TurnCancelled {
		t.Fatalf("event = %#v, want a cancelled terminal Turn event", cancelled)
	}
	waitForTurnCancellation(t, session.turn)
	if snapshot := snapshotController(t, session.controller); snapshot.SessionStatus != SessionReady {
		t.Fatalf("SessionStatus = %q, want a cancelled Turn to keep the Session ready", snapshot.SessionStatus)
	}
	if err := cancelTurn(t, session.controller, "request-unknown"); !errors.Is(err, ErrTurnNotCancellable) {
		t.Fatalf("CancelTurn(unknown) error = %v, want ErrTurnNotCancellable", err)
	}
}

func TestControllerCancelQueuedRemovesRequest(t *testing.T) {
	session := startActiveSession(t)
	firstID, err := submitTerminal(t, session.controller, "first queued")
	if err != nil {
		t.Fatalf("Submit(first) error = %v, want nil", err)
	}
	ctx, cancel := controllerContext(t)
	defer cancel()
	secondID, err := session.controller.Submit(ctx, Input{
		SessionID: "session-1", Source: InputMobile, Text: "second queued",
	})
	if err != nil {
		t.Fatalf("Submit(second) error = %v, want nil", err)
	}

	if err := cancelQueued(t, session.controller, firstID); err != nil {
		t.Fatalf("CancelQueued() error = %v, want nil", err)
	}
	snapshot := snapshotController(t, session.controller)
	if snapshot.QueueDepth != 1 || len(snapshot.QueuedRequests) != 1 {
		t.Fatalf("Snapshot = %#v, want the two FIFO views to agree on one entry", snapshot)
	}
	remaining := snapshot.QueuedRequests[0]
	if remaining.RequestID != secondID || remaining.Source != InputMobile || remaining.Preview != "second queued" {
		t.Fatalf("QueuedRequests[0] = %#v, want the surviving mobile entry", remaining)
	}

	if err := cancelQueued(t, session.controller, firstID); !errors.Is(err, ErrRequestNotQueued) {
		t.Fatalf("CancelQueued(twice) error = %v, want ErrRequestNotQueued", err)
	}
	// The running Turn is not a queue entry; it can only be stopped by cancel_turn.
	if err := cancelQueued(t, session.controller, session.requestID); !errors.Is(err, ErrRequestNotQueued) {
		t.Fatalf("CancelQueued(running) error = %v, want ErrRequestNotQueued", err)
	}
}

func TestControllerStopEndsPendingAction(t *testing.T) {
	session := startActiveSession(t)
	request, _ := awaitActionRequired(t, session, approvalSpec("Bash", "ls", true))

	stopCtx, cancelStop := controllerContext(t)
	defer cancelStop()
	stopped := make(chan error, 1)
	go func() { stopped <- session.controller.Stop(stopCtx) }()

	decision := awaitDecision(t, request)
	if decision.Kind != projectclaude.DecisionDeny || !decision.Interrupt {
		t.Fatalf("decision = %+v, want an interrupting deny so shutdown can proceed", decision)
	}
	if session.host.interruptCount() != 0 {
		t.Fatalf("Interrupt called %d times, want 0 while an action was pending", session.host.interruptCount())
	}
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatalf("Stop() error = %v, want nil", err)
		}
	case <-time.After(controllerTestTimeout):
		t.Fatal("Stop() did not finish while an action was pending")
	}
	if snapshot := snapshotController(t, session.controller); snapshot.Turn.PendingAction != nil {
		t.Fatalf("final Snapshot.Turn = %#v, want no pending action", snapshot.Turn)
	}
}

func TestControllerTurnFailureEndsPendingAction(t *testing.T) {
	session := startActiveSession(t)
	request, _ := awaitActionRequired(t, session, approvalSpec("Bash", "ls", true))

	session.turn.emit(t, projectclaude.Event{
		Kind: projectclaude.EventTurnFailed, SessionID: "session-1", Text: "Claude API Error",
	})
	if decision := awaitDecision(t, request); decision.Kind != projectclaude.DecisionDeny {
		t.Fatalf("decision = %+v, want deny once the Turn failed", decision)
	}
	failed := receiveDomainEvent(t, session.events)
	if failed.Kind != EventTurnFailed || failed.TurnStatus != TurnFailed {
		t.Fatalf("event = %#v, want turn_failed", failed)
	}
	if failed.PendingAction != nil {
		t.Fatalf("turn_failed must not carry a pending action: %#v", failed.PendingAction)
	}
	if snapshot := snapshotController(t, session.controller); snapshot.Turn.PendingAction != nil {
		t.Fatalf("Snapshot.Turn = %#v, want no pending action after failure", snapshot.Turn)
	}
}

func TestControllerPublishesContentBlocksWithAssignedIDs(t *testing.T) {
	session := startActiveSession(t)
	failed := true
	adapterBlocks := []projectclaude.ContentBlock{
		{Kind: projectclaude.BlockThinking, Thinking: "weighing the options"},
		{
			Kind: projectclaude.BlockToolUse, ToolUseID: "tool-use-1",
			ToolClass: projectclaude.ToolMCP, MCPServer: "filesystem", ToolName: "read_file",
		},
		{Kind: projectclaude.BlockToolResult, ToolUseID: "tool-use-1", IsError: &failed},
	}

	var previousSeq uint64
	for index, block := range adapterBlocks {
		session.turn.emit(t, projectclaude.Event{
			Kind: projectclaude.EventContentBlock, SessionID: "session-1", Block: &block,
		})
		event := receiveDomainEvent(t, session.events)
		if event.Kind != EventContentBlock || event.Block == nil {
			t.Fatalf("event %d = %#v, want a content_block carrying a block", index, event)
		}
		// The Controller owns seq, so it also owns the display IDs the clients
		// correlate against (B2 §4.2).
		wantID := fmt.Sprintf("%s-block-%d", session.requestID, index+1)
		if event.Block.ID != wantID {
			t.Fatalf("Block.ID = %q, want %q", event.Block.ID, wantID)
		}
		if event.RequestID != session.requestID || event.Source != InputTerminal {
			t.Fatalf("event %d correlation = %#v, want the current terminal Turn", index, event)
		}
		if event.Seq <= previousSeq {
			t.Fatalf("Seq = %d, want strictly greater than %d", event.Seq, previousSeq)
		}
		previousSeq = event.Seq
		if event.Text != "" || event.PendingAction != nil || event.Code != "" {
			t.Fatalf("content_block must stay structurally minimal: %#v", event)
		}
	}

	if got := adapterBlocks[0]; got.Kind != projectclaude.BlockThinking {
		t.Fatalf("Adapter block mutated: %#v", got)
	}
	snapshot := snapshotController(t, session.controller)
	if snapshot.Turn.Message != "" || snapshot.Turn.Status != TurnRunning {
		t.Fatalf("Snapshot.Turn = %#v, want a running Turn with no accumulated text", snapshot.Turn)
	}
}

func TestControllerForwardsAdapterUnsupportedNotice(t *testing.T) {
	session := startActiveSession(t)
	session.turn.emit(t, projectclaude.Event{
		Kind: projectclaude.EventUnsupported, SessionID: "session-1",
		Code: projectclaude.UnsupportedUnknownSubtype,
	})

	notice := receiveDomainEvent(t, session.events)
	if notice.Kind != EventUnsupported || notice.Code != UnsupportedUnknownSubtype {
		t.Fatalf("event = %#v, want unsupported/unknown_subtype", notice)
	}
	if notice.RequestID != session.requestID || notice.Block != nil || notice.PendingAction != nil || notice.Text != "" {
		t.Fatalf("unsupported notice must stay structurally empty: %#v", notice)
	}

	// A degraded block only drops that one block; the Turn keeps streaming.
	session.turn.emit(t, projectclaude.Event{
		Kind: projectclaude.EventText, SessionID: "session-1", Text: "still here",
	})
	if event := receiveDomainEvent(t, session.events); event.Kind != EventTextDelta || event.Text != "still here" {
		t.Fatalf("event = %#v, want the following text delta", event)
	}
}

func TestControllerBuffersContentBlocksUntilSessionConfirmed(t *testing.T) {
	host := newControlledHost()
	controller := startController(t, host, withRequestIDGenerator(monotonicRequestIDs()))
	subscriptionContext, cancelSubscription := context.WithCancel(context.Background())
	t.Cleanup(cancelSubscription)
	_, events, err := controller.Subscribe(subscriptionContext, 64)
	if err != nil {
		t.Fatalf("Subscribe() error = %v, want nil", err)
	}
	requestID, err := submitTerminal(t, controller, "run the tool")
	if err != nil {
		t.Fatalf("Submit() error = %v, want nil", err)
	}
	turn := receiveControlledTurn(t, host)

	// Anything the Adapter emits before Claude confirms the session ID has to wait
	// for that ID, otherwise clients would see events for a session they cannot
	// name yet (A5.2 §4).
	turn.emitAndWaitForDelivery(t, projectclaude.Event{
		Kind: projectclaude.EventContentBlock, SessionID: "session-1",
		Block: &projectclaude.ContentBlock{Kind: projectclaude.BlockThinking, Thinking: "early"},
	})
	turn.emitAndWaitForDelivery(t, projectclaude.Event{
		Kind: projectclaude.EventText, SessionID: "session-1", Text: "early text",
	})
	assertNoDomainEvent(t, events)

	turn.emitAndWaitForDelivery(t, projectclaude.Event{
		Kind: projectclaude.EventSessionStarted, SessionID: "session-1",
	})
	for _, want := range []EventKind{EventSessionStarted, EventTurnStarted} {
		if event := receiveDomainEvent(t, events); event.Kind != want {
			t.Fatalf("event = %q, want %q", event.Kind, want)
		}
	}
	replayed := receiveDomainEvent(t, events)
	if replayed.Kind != EventContentBlock || replayed.Block == nil || replayed.Block.Thinking != "early" {
		t.Fatalf("event = %#v, want the buffered content block replayed first", replayed)
	}
	if want := fmt.Sprintf("%s-block-1", requestID); replayed.Block.ID != want {
		t.Fatalf("Block.ID = %q, want %q", replayed.Block.ID, want)
	}
	if replayed.SessionID != "session-1" {
		t.Fatalf("SessionID = %q, want the confirmed session", replayed.SessionID)
	}
	if event := receiveDomainEvent(t, events); event.Kind != EventTextDelta || event.Text != "early text" {
		t.Fatalf("event = %#v, want the buffered text delta in order", event)
	}
}
