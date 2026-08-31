package channel

import (
	"context"
	"errors"

	"github.com/tea4go/claude-mobile-remote/protocol"
)

var ErrInvalidSequence = errors.New("invalid event sequence")

// Controller is the A3/B2 boundary required by a network Session Channel. Every
// command a remote client can issue has to travel through this interface so the
// Channel never reaches the SDK on its own.
type Controller interface {
	Submit(context.Context, protocol.Input) (string, error)
	Subscribe(context.Context, int) (protocol.Snapshot, <-chan protocol.Event, error)
	Snapshot(context.Context) (protocol.Snapshot, error)
	ResolveAction(context.Context, string, protocol.ActionDecision, []protocol.QuestionAnswer) error
	CancelTurn(context.Context, string) error
	CancelQueued(context.Context, string) error
}

// Channel routes one exact session ID to its Controller.
type Channel struct {
	sessionID  string
	controller Controller
}

func New(sessionID string, controller Controller) *Channel {
	return &Channel{sessionID: sessionID, controller: controller}
}

func (c *Channel) SessionID() string {
	if c == nil {
		return ""
	}
	return c.sessionID
}

// guard rejects any command addressed to a different session before the
// Controller is touched, keeping session isolation independent of the caller.
func (c *Channel) guard(sessionID string) error {
	if sessionID != c.sessionID {
		return protocol.ErrSessionMismatch
	}
	return nil
}

func (c *Channel) SubmitText(ctx context.Context, sessionID, text string) (string, error) {
	if err := c.guard(sessionID); err != nil {
		return "", err
	}
	return c.controller.Submit(ctx, protocol.Input{
		SessionID: c.sessionID,
		Source:    protocol.InputMobile,
		Text:      text,
	})
}

// Snapshot exposes the current state so a caller can validate a resolution
// against the pending action before asking the Controller to apply it.
func (c *Channel) Snapshot(ctx context.Context, sessionID string) (protocol.Snapshot, error) {
	if err := c.guard(sessionID); err != nil {
		return protocol.Snapshot{}, err
	}
	snapshot, err := c.controller.Snapshot(ctx)
	if err != nil {
		return protocol.Snapshot{}, err
	}
	if snapshot.SessionID != c.sessionID {
		return protocol.Snapshot{}, protocol.ErrSessionMismatch
	}
	return snapshot, nil
}

func (c *Channel) ResolveAction(
	ctx context.Context,
	sessionID string,
	actionID string,
	decision protocol.ActionDecision,
	answers []protocol.QuestionAnswer,
) error {
	if err := c.guard(sessionID); err != nil {
		return err
	}
	return c.controller.ResolveAction(ctx, actionID, decision, answers)
}

func (c *Channel) CancelTurn(ctx context.Context, sessionID, requestID string) error {
	if err := c.guard(sessionID); err != nil {
		return err
	}
	return c.controller.CancelTurn(ctx, requestID)
}

func (c *Channel) CancelQueued(ctx context.Context, sessionID, requestID string) error {
	if err := c.guard(sessionID); err != nil {
		return err
	}
	return c.controller.CancelQueued(ctx, requestID)
}

type Subscription struct {
	Snapshot protocol.Snapshot
	Events   <-chan protocol.Event
	Errors   <-chan error
	Cancel   context.CancelFunc
}

func (c *Channel) OpenSubscription(ctx context.Context, capacity int) (*Subscription, error) {
	childContext, cancel := context.WithCancel(ctx)
	snapshot, source, err := c.controller.Subscribe(childContext, capacity)
	if err != nil {
		cancel()
		return nil, err
	}
	if snapshot.SessionID != c.sessionID {
		cancel()
		return nil, protocol.ErrSessionMismatch
	}

	events := make(chan protocol.Event, capacity)
	errorsChannel := make(chan error, 1)
	go forwardEvents(childContext, cancel, c.sessionID, snapshot.Seq, source, events, errorsChannel)
	return &Subscription{
		Snapshot: snapshot,
		Events:   events,
		Errors:   errorsChannel,
		Cancel:   cancel,
	}, nil
}

func forwardEvents(
	ctx context.Context,
	cancel context.CancelFunc,
	sessionID string,
	lastSeq uint64,
	source <-chan protocol.Event,
	destination chan<- protocol.Event,
	errorsChannel chan<- error,
) {
	defer cancel()
	defer close(destination)
	defer close(errorsChannel)
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-source:
			if !ok {
				return
			}
			if event.SessionID != sessionID {
				errorsChannel <- protocol.ErrSessionMismatch
				return
			}
			if event.Seq <= lastSeq {
				errorsChannel <- ErrInvalidSequence
				return
			}
			select {
			case destination <- event:
				lastSeq = event.Seq
			case <-ctx.Done():
				return
			}
		}
	}
}
