package push

import (
	"context"
	"log"
	"time"

	"github.com/tea4go/claude-mobile-remote/protocol"
)

type EventSource interface {
	Subscribe(context.Context, int) (protocol.Snapshot, <-chan protocol.Event, error)
}

// Watch keeps a bounded notification worker alive independently of mobile sockets.
// A currentPairing callback prevents expired or closed sessions from receiving pushes.
func Watch(parent context.Context, source EventSource, sender Sender, currentPairing func() (protocol.Pairing, bool)) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	pending := make(chan Notification, 16)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		for {
			select {
			case <-ctx.Done():
				return
			case notice := <-pending:
				if ctx.Err() != nil {
					return
				}
				if err := sender.Send(ctx, notice); err != nil && ctx.Err() == nil {
					log.Printf("[remote-push] notification delivery failed: %v", err)
				}
			}
		}
	}()
	defer func() { cancel(); <-workerDone }()
	var watermark uint64
	for ctx.Err() == nil {
		subscriptionCtx, stopSubscription := context.WithCancel(ctx)
		snapshot, events, err := source.Subscribe(subscriptionCtx, 256)
		if err != nil {
			stopSubscription()
			<-ctx.Done()
			return
		}
		if snapshot.Seq > watermark {
			watermark = snapshot.Seq
		}
		open := true
		for open {
			select {
			case <-ctx.Done():
				stopSubscription()
				return
			case event, ok := <-events:
				if !ok {
					open = false
					break
				}
				if event.Seq <= watermark {
					continue
				}
				watermark = event.Seq
				kind := Kind(event)
				if kind == "" {
					continue
				}
				pairing, valid := currentPairing()
				if !valid || event.SessionID != pairing.SessionID || !pairing.ExpiresAt.After(time.Now()) {
					continue
				}
				notice := Notification{Topic: Topic(pairing.SessionID, pairing.PairToken), SessionID: event.SessionID, Seq: event.Seq, Kind: kind}
				select {
				case pending <- notice:
				default:
					select {
					case <-pending:
					default:
					}
					select {
					case pending <- notice:
					default:
					}
				}
			}
		}
		stopSubscription()
		select {
		case <-ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
}
