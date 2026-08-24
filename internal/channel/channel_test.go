package channel

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	projectsession "github.com/tea4go/claude-mobile-remote/internal/session"
)

type fakeController struct {
	mu sync.Mutex

	lastInput projectsession.Input
	submitID  string
	submitErr error
	snapshot  projectsession.Snapshot
	events    chan projectsession.Event
	subDone   <-chan struct{}
}

func (f *fakeController) Submit(_ context.Context, input projectsession.Input) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastInput = input
	return f.submitID, f.submitErr
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
	requestID, err := sessionChannel.SubmitText(context.Background(), " hello ")
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
