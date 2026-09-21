package channel

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	projectsession "github.com/tea4go/claude-mobile-remote/internal/session"
)

type resolveCall struct {
	actionID string
	decision projectsession.ActionDecision
	answers  []projectsession.QuestionAnswer
}

type fakeController struct {
	mu sync.Mutex

	lastInput projectsession.Input
	submitID  string
	submitErr error
	snapshot  projectsession.Snapshot
	events    chan projectsession.Event
	subDone   <-chan struct{}

	calls       []string
	resolves    []resolveCall
	cancelled   []string
	dequeued    []string
	snapshotErr error
	resolveErr  error
	cancelErr   error
	dequeueErr  error
}

func (f *fakeController) Submit(_ context.Context, input projectsession.Input) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastInput = input
	f.calls = append(f.calls, "submit")
	return f.submitID, f.submitErr
}

func (f *fakeController) Snapshot(context.Context) (projectsession.Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "snapshot")
	if f.snapshotErr != nil {
		return projectsession.Snapshot{}, f.snapshotErr
	}
	return f.snapshot, nil
}

func (f *fakeController) ResolveAction(_ context.Context, actionID string, decision projectsession.ActionDecision, answers []projectsession.QuestionAnswer) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "resolve_action")
	f.resolves = append(f.resolves, resolveCall{actionID: actionID, decision: decision, answers: answers})
	return f.resolveErr
}

func (f *fakeController) CancelTurn(_ context.Context, requestID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "cancel_turn")
	f.cancelled = append(f.cancelled, requestID)
	return f.cancelErr
}

func (f *fakeController) CancelQueued(_ context.Context, requestID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "cancel_queued")
	f.dequeued = append(f.dequeued, requestID)
	return f.dequeueErr
}

func (f *fakeController) observedCalls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func (f *fakeController) Subscribe(ctx context.Context, capacity int) (projectsession.Snapshot, <-chan projectsession.Event, error) {
	if capacity <= 0 {
		return projectsession.Snapshot{}, nil, projectsession.ErrInvalidInput
	}
	f.mu.Lock()
	f.subDone = ctx.Done()
	f.mu.Unlock()
	return f.snapshot, f.events, nil
}

func (f *fakeController) input() projectsession.Input {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastInput
}

func (f *fakeController) subscriptionDone() <-chan struct{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.subDone
}

func TestChannelSubmitTextForcesMobileSource(t *testing.T) {
	controller := &fakeController{submitID: "request-1"}
	sessionChannel := New("session-1", controller)
	requestID, err := sessionChannel.SubmitText(context.Background(), "session-1", " hello ")
	if err != nil || requestID != "request-1" {
		t.Fatalf("SubmitText() = %q, %v", requestID, err)
	}
	want := projectsession.Input{SessionID: "session-1", Source: projectsession.InputMobile, Text: " hello "}
	if got := controller.input(); got != want {
		t.Fatalf("controller input = %#v, want %#v", got, want)
	}
}

func TestRegistryDoesNotCrossSessions(t *testing.T) {
	registry := NewRegistry()
	first := New("session-1", &fakeController{})
	if err := registry.Register(first); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if got, ok := registry.Get("session-1"); !ok || got != first {
		t.Fatalf("Get(session-1) = %v, %v", got, ok)
	}
	if _, ok := registry.Get("session-2"); ok {
		t.Fatal("Get(session-2) returned a cross-session channel")
	}
}

func TestRegistryRejectsDuplicateAndUnregisters(t *testing.T) {
	registry := NewRegistry()
	first := New("session-1", &fakeController{})
	if first.SessionID() != "session-1" {
		t.Fatalf("SessionID() = %q", first.SessionID())
	}
	if err := registry.Register(first); err != nil {
		t.Fatalf("Register(first) error = %v", err)
	}
	if err := registry.Register(New("session-1", &fakeController{})); !errors.Is(err, ErrAlreadyRegistered) {
		t.Fatalf("Register(duplicate) error = %v, want ErrAlreadyRegistered", err)
	}
	registry.Unregister("session-1")
	if _, ok := registry.Get("session-1"); ok {
		t.Fatal("Unregister() left the channel registered")
	}
}

func TestChannelOpenSubscriptionPreservesAtomicBoundary(t *testing.T) {
	controller := &fakeController{
		snapshot: projectsession.Snapshot{SessionID: "session-1", Seq: 7},
		events:   make(chan projectsession.Event, 1),
	}
	sessionChannel := New("session-1", controller)
	subscription, err := sessionChannel.OpenSubscription(context.Background(), 4)
	if err != nil {
		t.Fatalf("OpenSubscription() error = %v", err)
	}
	if subscription.Snapshot.Seq != 7 {
		t.Fatalf("snapshot seq = %d, want 7", subscription.Snapshot.Seq)
	}
	controller.events <- projectsession.Event{SessionID: "session-1", Seq: 8, Kind: projectsession.EventTurnStarted}
	if event := <-subscription.Events; event.Seq != 8 {
		t.Fatalf("event seq = %d, want 8", event.Seq)
	}

	subscription.Cancel()
	select {
	case <-controller.subscriptionDone():
	case <-time.After(time.Second):
		t.Fatal("Cancel() did not release Controller subscription")
	}
}

func TestChannelOpenSubscriptionRejectsMismatchedSnapshot(t *testing.T) {
	controller := &fakeController{
		snapshot: projectsession.Snapshot{SessionID: "session-2", Seq: 7},
		events:   make(chan projectsession.Event),
	}
	_, err := New("session-1", controller).OpenSubscription(context.Background(), 4)
	if !errors.Is(err, projectsession.ErrSessionMismatch) {
		t.Fatalf("OpenSubscription() error = %v, want ErrSessionMismatch", err)
	}
}

func TestChannelSubscriptionRejectsInvalidEvents(t *testing.T) {
	tests := []struct {
		name    string
		event   projectsession.Event
		wantErr error
	}{
		{name: "cross session", event: projectsession.Event{SessionID: "session-2", Seq: 8}, wantErr: projectsession.ErrSessionMismatch},
		{name: "non increasing seq", event: projectsession.Event{SessionID: "session-1", Seq: 7}, wantErr: ErrInvalidSequence},
		{name: "sequence gap", event: projectsession.Event{SessionID: "session-1", Seq: 9}, wantErr: ErrInvalidSequence},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			controller := &fakeController{
				snapshot: projectsession.Snapshot{SessionID: "session-1", Seq: 7},
				events:   make(chan projectsession.Event, 1),
			}
			subscription, err := New("session-1", controller).OpenSubscription(context.Background(), 4)
			if err != nil {
				t.Fatalf("OpenSubscription() error = %v", err)
			}
			defer subscription.Cancel()
			controller.events <- test.event
			select {
			case got := <-subscription.Errors:
				if !errors.Is(got, test.wantErr) {
					t.Fatalf("subscription error = %v, want %v", got, test.wantErr)
				}
			case <-time.After(time.Second):
				t.Fatalf("timed out waiting for %v", test.wantErr)
			}
		})
	}
}

func TestChannelSubscriptionReportsUnexpectedSourceClose(t *testing.T) {
	controller := &fakeController{
		snapshot: projectsession.Snapshot{SessionID: "session-1", Seq: 7},
		events:   make(chan projectsession.Event),
	}
	subscription, err := New("session-1", controller).OpenSubscription(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	close(controller.events)
	if got := <-subscription.Errors; !errors.Is(got, ErrSubscriptionClosed) {
		t.Fatalf("subscription error = %v, want ErrSubscriptionClosed", got)
	}
}

// TestChannelControlCommandsReachTheControllerVerbatim pins that the three B2
// control commands travel through the Controller unchanged. The Channel adds no
// interpretation of its own, so it can never reach the SDK behind the actor.
func TestChannelControlCommandsReachTheControllerVerbatim(t *testing.T) {
	controller := &fakeController{}
	sessionChannel := New("session-1", controller)
	answers := []projectsession.QuestionAnswer{{QuestionID: "question-1", Values: []string{"A"}}}

	if err := sessionChannel.ResolveAction(context.Background(), "session-1", "action-1", projectsession.DecisionAllowOnce, nil); err != nil {
		t.Fatalf("ResolveAction(approval) error = %v", err)
	}
	if err := sessionChannel.ResolveAction(context.Background(), "session-1", "action-2", "", answers); err != nil {
		t.Fatalf("ResolveAction(answers) error = %v", err)
	}
	if err := sessionChannel.CancelTurn(context.Background(), "session-1", "request-1"); err != nil {
		t.Fatalf("CancelTurn() error = %v", err)
	}
	if err := sessionChannel.CancelQueued(context.Background(), "session-1", "request-2"); err != nil {
		t.Fatalf("CancelQueued() error = %v", err)
	}

	wantCalls := []string{"resolve_action", "resolve_action", "cancel_turn", "cancel_queued"}
	if got := controller.observedCalls(); !reflect.DeepEqual(got, wantCalls) {
		t.Fatalf("controller calls = %v, want %v", got, wantCalls)
	}
	wantResolves := []resolveCall{
		{actionID: "action-1", decision: projectsession.DecisionAllowOnce},
		{actionID: "action-2", answers: answers},
	}
	if !reflect.DeepEqual(controller.resolves, wantResolves) {
		t.Fatalf("resolutions = %#v, want %#v", controller.resolves, wantResolves)
	}
	if !reflect.DeepEqual(controller.cancelled, []string{"request-1"}) || !reflect.DeepEqual(controller.dequeued, []string{"request-2"}) {
		t.Fatalf("cancel targets = %v / %v", controller.cancelled, controller.dequeued)
	}
}

// TestChannelRejectsCommandsForAnotherSession keeps the Channel a single-session
// boundary: a command addressed to a different sessionId is refused before the
// Controller is touched, so a routing mistake cannot cross sessions.
func TestChannelRejectsCommandsForAnotherSession(t *testing.T) {
	controller := &fakeController{submitID: "request-1"}
	sessionChannel := New("session-1", controller)
	tests := []struct {
		name string
		call func() error
	}{
		{name: "send_text", call: func() error {
			_, err := sessionChannel.SubmitText(context.Background(), "session-2", "hello")
			return err
		}},
		{name: "snapshot", call: func() error {
			_, err := sessionChannel.Snapshot(context.Background(), "session-2")
			return err
		}},
		{name: "resolve_action", call: func() error {
			return sessionChannel.ResolveAction(context.Background(), "session-2", "action-1", projectsession.DecisionAllowOnce, nil)
		}},
		{name: "cancel_turn", call: func() error {
			return sessionChannel.CancelTurn(context.Background(), "session-2", "request-1")
		}},
		{name: "cancel_queued", call: func() error {
			return sessionChannel.CancelQueued(context.Background(), "session-2", "request-1")
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); !errors.Is(err, projectsession.ErrSessionMismatch) {
				t.Fatalf("%s error = %v, want ErrSessionMismatch", test.name, err)
			}
		})
	}
	if got := controller.observedCalls(); len(got) != 0 {
		t.Fatalf("controller was reached by cross-session commands: %v", got)
	}
}

// TestChannelSnapshotRejectsForeignSnapshot mirrors the OpenSubscription guard:
// a Controller that answers with another session's state is a wiring bug, and
// the Channel must not pass that state on to a client.
func TestChannelSnapshotRejectsForeignSnapshot(t *testing.T) {
	controller := &fakeController{snapshot: projectsession.Snapshot{SessionID: "session-2", Seq: 7}}
	if _, err := New("session-1", controller).Snapshot(context.Background(), "session-1"); !errors.Is(err, projectsession.ErrSessionMismatch) {
		t.Fatalf("Snapshot() error = %v, want ErrSessionMismatch", err)
	}
}

// TestChannelControlCommandsPropagateControllerErrors keeps the Controller the
// only judge of a command outcome: the Channel forwards its verdict unwrapped so
// Transport can map the sentinels onto stable wire codes.
func TestChannelControlCommandsPropagateControllerErrors(t *testing.T) {
	controller := &fakeController{
		resolveErr: projectsession.ErrActionNotPending,
		cancelErr:  projectsession.ErrTurnNotCancellable,
		dequeueErr: projectsession.ErrRequestNotQueued,
	}
	sessionChannel := New("session-1", controller)
	if err := sessionChannel.ResolveAction(context.Background(), "session-1", "action-1", projectsession.DecisionDeny, nil); !errors.Is(err, projectsession.ErrActionNotPending) {
		t.Fatalf("ResolveAction() error = %v, want ErrActionNotPending", err)
	}
	if err := sessionChannel.CancelTurn(context.Background(), "session-1", "request-1"); !errors.Is(err, projectsession.ErrTurnNotCancellable) {
		t.Fatalf("CancelTurn() error = %v, want ErrTurnNotCancellable", err)
	}
	if err := sessionChannel.CancelQueued(context.Background(), "session-1", "request-1"); !errors.Is(err, projectsession.ErrRequestNotQueued) {
		t.Fatalf("CancelQueued() error = %v, want ErrRequestNotQueued", err)
	}
}
