package push

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/tea4go/claude-mobile-remote/protocol"
)

type watchSource struct {
	events     chan protocol.Event
	subscribed chan struct{}
}

func (s *watchSource) Subscribe(context.Context, int) (protocol.Snapshot, <-chan protocol.Event, error) {
	close(s.subscribed)
	return protocol.Snapshot{}, s.events, nil
}
func TestWatchDoesNotBlockEventsAndDropsRevokedQueuedRoutes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source := &watchSource{events: make(chan protocol.Event), subscribed: make(chan struct{})}
	pairing := protocol.Pairing{SessionID: "s", PairToken: "first-pairing", ExpiresAt: time.Now().Add(time.Hour)}
	var mu sync.Mutex
	current := func() (protocol.Pairing, bool) { mu.Lock(); defer mu.Unlock(); return pairing, true }
	entered, release := make(chan struct{}), make(chan struct{})
	calls := 0
	delivered := make(chan Notification, 64)
	sender := senderFunc(func(ctx context.Context, n Notification) error {
		calls++
		if calls == 1 {
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
			}
		} else {
			delivered <- n
		}
		return nil
	})
	done := make(chan struct{})
	go func() { Watch(ctx, source, sender, current); close(done) }()
	<-source.subscribed
	source.events <- protocol.Event{SessionID: "s", Seq: 1, Kind: protocol.EventTurnCompleted, TurnStatus: protocol.TurnCompleted}
	<-entered
	// All these events must be consumed while the external sender is blocked.
	consumed := make(chan struct{})
	go func() {
		for i := uint64(2); i < 50; i++ {
			source.events <- protocol.Event{SessionID: "s", Seq: i, Kind: protocol.EventTurnCompleted, TurnStatus: protocol.TurnCompleted}
		}
		source.events <- protocol.Event{SessionID: "s", Seq: 50, Kind: protocol.EventTextDelta}
		close(consumed)
	}()
	select {
	case <-consumed:
	case <-time.After(time.Second):
		t.Fatal("push blocked host events")
	}
	mu.Lock()
	pairing.PairToken = "rotated-pairing"
	mu.Unlock()
	close(release)
	source.events <- protocol.Event{SessionID: "s", Seq: 51, Kind: protocol.EventTurnCompleted, TurnStatus: protocol.TurnCompleted}
	select {
	case notice := <-delivered:
		if notice.Topic != Topic("s", "rotated-pairing") {
			t.Fatal("revoked queued route was sent")
		}
	case <-time.After(time.Second):
		t.Fatal("new route was not delivered after old queue drained")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("watch failed to stop")
	}
	if calls != 2 {
		t.Fatal("revoked queued route was sent")
	}
}
