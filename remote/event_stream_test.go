package remote

import (
	"context"
	"errors"
	"testing"

	"github.com/tea4go/claude-mobile-remote/protocol"
)

func TestEventStreamBroadcastsIndependently(t *testing.T) {
	stream, err := NewEventStream(protocol.Snapshot{SessionID: "session-1"})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	_, first, err := stream.Subscribe(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	_, second, err := stream.Subscribe(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}

	event := protocol.Event{Kind: protocol.EventTurnStarted, SessionID: "session-1", RequestID: "request-1", Text: "hello", TurnStatus: protocol.TurnRunning}
	if err := stream.Publish(event); err != nil {
		t.Fatal(err)
	}
	for name, events := range map[string]<-chan protocol.Event{"first": first, "second": second} {
		got := <-events
		if got.Seq != 1 || got.RequestID != "request-1" {
			t.Fatalf("%s subscriber event = %#v", name, got)
		}
	}
}

func TestEventStreamRejectsCrossSessionData(t *testing.T) {
	stream, err := NewEventStream(protocol.Snapshot{SessionID: "session-1"})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	if err := stream.Publish(protocol.Event{SessionID: "session-2"}); !errors.Is(err, protocol.ErrSessionMismatch) {
		t.Fatalf("Publish() error = %v", err)
	}
	if err := stream.UpdateSnapshot(protocol.Snapshot{SessionID: "session-2"}); !errors.Is(err, protocol.ErrSessionMismatch) {
		t.Fatalf("UpdateSnapshot() error = %v", err)
	}
}

func TestEventStreamClosesOnlySlowSubscriber(t *testing.T) {
	stream, err := NewEventStream(protocol.Snapshot{SessionID: "session-1"})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	_, slow, err := stream.Subscribe(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	_, fast, err := stream.Subscribe(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}

	first := protocol.Event{Kind: protocol.EventTextDelta, SessionID: "session-1", RequestID: "request-1", Text: "a"}
	second := protocol.Event{Kind: protocol.EventTextDelta, SessionID: "session-1", RequestID: "request-1", Text: "b"}
	if err := stream.Publish(first); err != nil {
		t.Fatal(err)
	}
	if err := stream.Publish(second); err != nil {
		t.Fatal(err)
	}
	if got := <-fast; got.Text != "a" {
		t.Fatalf("fast first event = %#v", got)
	}
	if got := <-fast; got.Text != "b" {
		t.Fatalf("fast second event = %#v", got)
	}
	if got := <-slow; got.Text != "a" {
		t.Fatalf("slow buffered event = %#v", got)
	}
	if _, ok := <-slow; ok {
		t.Fatal("slow subscriber remained open after overflow")
	}
}
