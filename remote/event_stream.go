package remote

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/tea4go/claude-mobile-remote/protocol"
)

// ErrEventStreamClosed reports that a host tried to use a closed EventStream.
var ErrEventStreamClosed = errors.New("remote event stream closed")

// EventStream is the reusable single-session event source for embedded hosts.
// Publish fans one event out to every subscriber through an independent bounded
// channel. A slow subscriber is closed without blocking the publisher or any
// other subscriber.
type EventStream struct {
	mu          sync.Mutex
	snapshot    protocol.Snapshot
	subscribers map[uint64]chan protocol.Event
	nextID      uint64
	closed      bool
}

// NewEventStream constructs a stream bound permanently to one session.
func NewEventStream(snapshot protocol.Snapshot) (*EventStream, error) {
	if strings.TrimSpace(snapshot.SessionID) == "" {
		return nil, errors.New("session ID is required")
	}
	return &EventStream{
		snapshot:    cloneSnapshot(snapshot),
		subscribers: make(map[uint64]chan protocol.Event),
	}, nil
}

// Subscribe atomically returns the current snapshot and registers one
// independent event channel.
func (s *EventStream) Subscribe(ctx context.Context, capacity int) (protocol.Snapshot, <-chan protocol.Event, error) {
	if err := contextError(ctx); err != nil {
		return protocol.Snapshot{}, nil, err
	}
	if capacity <= 0 {
		capacity = 1
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return protocol.Snapshot{}, nil, ErrEventStreamClosed
	}
	s.nextID++
	id := s.nextID
	events := make(chan protocol.Event, capacity)
	s.subscribers[id] = events
	snapshot := cloneSnapshot(s.snapshot)
	s.mu.Unlock()

	if ctx != nil {
		go func() {
			<-ctx.Done()
			s.unsubscribe(id)
		}()
	}
	return snapshot, events, nil
}

// Snapshot returns a detached copy of the current stream state.
func (s *EventStream) Snapshot(ctx context.Context) (protocol.Snapshot, error) {
	if err := contextError(ctx); err != nil {
		return protocol.Snapshot{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return protocol.Snapshot{}, ErrEventStreamClosed
	}
	return cloneSnapshot(s.snapshot), nil
}

// UpdateSnapshot replaces host-derived state without changing the event
// sequence. It rejects data belonging to another session.
func (s *EventStream) UpdateSnapshot(snapshot protocol.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrEventStreamClosed
	}
	if snapshot.SessionID != s.snapshot.SessionID {
		return protocol.ErrSessionMismatch
	}
	snapshot.Seq = s.snapshot.Seq
	s.snapshot = cloneSnapshot(snapshot)
	return nil
}

// Publish assigns the next sequence number, updates the snapshot projection and
// broadcasts the event to every current subscriber.
func (s *EventStream) Publish(event protocol.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrEventStreamClosed
	}
	if event.SessionID == "" {
		event.SessionID = s.snapshot.SessionID
	}
	if event.SessionID != s.snapshot.SessionID {
		return protocol.ErrSessionMismatch
	}
	s.snapshot = applyEvent(s.snapshot, event)
	s.snapshot.Seq++
	event.Seq = s.snapshot.Seq
	for id, events := range s.subscribers {
		select {
		case events <- event:
		default:
			delete(s.subscribers, id)
			close(events)
		}
	}
	return nil
}

// Close closes all subscriber channels. It is idempotent.
func (s *EventStream) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	for id, events := range s.subscribers {
		delete(s.subscribers, id)
		close(events)
	}
}

func (s *EventStream) unsubscribe(id uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if events, ok := s.subscribers[id]; ok {
		delete(s.subscribers, id)
		close(events)
	}
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func applyEvent(snapshot protocol.Snapshot, event protocol.Event) protocol.Snapshot {
	if event.SessionStatus != "" {
		snapshot.SessionStatus = event.SessionStatus
	}
	if event.TurnStatus != "" {
		snapshot.Turn.Status = event.TurnStatus
	}
	if event.RequestID != "" {
		snapshot.Turn.RequestID = event.RequestID
	}
	if event.Source != "" {
		snapshot.Turn.Source = event.Source
	}
	if event.Text != "" {
		snapshot.Turn.Message = event.Text
	}
	if event.Error != nil {
		snapshot.Turn.Error = cloneError(event.Error)
	}
	if event.PermissionMode.IsKnown() {
		snapshot.PermissionMode = event.PermissionMode
	}
	snapshot.Turn.PendingAction = clonePendingAction(event.PendingAction)
	return snapshot
}

func cloneSnapshot(snapshot protocol.Snapshot) protocol.Snapshot {
	snapshot.SessionError = cloneError(snapshot.SessionError)
	snapshot.Turn.Error = cloneError(snapshot.Turn.Error)
	snapshot.Turn.PendingAction = clonePendingAction(snapshot.Turn.PendingAction)
	snapshot.QueuedRequests = append([]protocol.QueuedRequest(nil), snapshot.QueuedRequests...)
	return snapshot
}

func cloneError(value *protocol.ErrorInfo) *protocol.ErrorInfo {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func clonePendingAction(value *protocol.PendingAction) *protocol.PendingAction {
	if value == nil {
		return nil
	}
	copy := *value
	copy.Questions = append([]protocol.Question(nil), value.Questions...)
	return &copy
}
