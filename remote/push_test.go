package remote

import (
	"context"
	"github.com/tea4go/claude-mobile-remote/protocol"
	"github.com/tea4go/claude-mobile-remote/push"
	"testing"
	"time"
)

type recordingPush struct{ received chan push.Notification }

func (p *recordingPush) Send(ctx context.Context, n push.Notification) error {
	select {
	case p.received <- n:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type pushController struct {
	fakeController
	stream     *EventStream
	subscribed chan struct{}
}

func (p *pushController) Subscribe(ctx context.Context, capacity int) (protocol.Snapshot, <-chan protocol.Event, error) {
	snapshot, events, err := p.stream.Subscribe(ctx, capacity)
	select {
	case p.subscribed <- struct{}{}:
	default:
	}
	return snapshot, events, err
}

func TestPushWorksWithoutMobileWebSocket(t *testing.T) {
	stream, _ := NewEventStream(protocol.Snapshot{SessionID: "session-1"})
	defer stream.Close()
	controller := &pushController{stream: stream, subscribed: make(chan struct{}, 1)}
	sender := &recordingPush{received: make(chan push.Notification, 2)}
	config := testConfig(controller)
	config.PushSender = sender
	server, err := newServer(config, fakeFactory(&fakeTransport{}))
	if err != nil {
		t.Fatal(err)
	}
	pairing, err := server.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close(context.Background())
	select {
	case <-controller.subscribed:
	case <-time.After(time.Second):
		t.Fatal("no push subscription")
	}
	_ = stream.Publish(protocol.Event{Kind: protocol.EventTextDelta, RequestID: "r", Text: "private Claude text"})
	_ = stream.Publish(protocol.Event{Kind: protocol.EventTurnCompleted, RequestID: "r", TurnStatus: protocol.TurnCompleted})
	select {
	case n := <-sender.received:
		if n.Topic != push.Topic(pairing.SessionID, pairing.PairToken) || n.Kind != "completed" || n.Seq != 2 {
			t.Fatal("wrong notification")
		}
	case <-time.After(time.Second):
		t.Fatal("no notification while no mobile is connected")
	}
}

func TestInvalidNotificationConfigDoesNotBlockRemote(t *testing.T) {
	t.Setenv("CMR_NOTIFY_URL", "http://insecure.invalid/v1/events")
	t.Setenv("CMR_NOTIFY_TOKEN", "fixture-notification-token-0000000000")
	config := testConfig(&fakeController{})
	config.EnablePush = true
	server, err := newServer(config, fakeFactory(&fakeTransport{}))
	if err != nil {
		t.Fatal("notification config blocked remote:", err)
	}
	if server.pushSender != nil {
		t.Fatal("invalid notification sender enabled")
	}
}
