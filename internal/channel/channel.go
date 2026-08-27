package channel

import (
	"context"
	"errors"

	projectsession "github.com/tea4go/claude-mobile-remote/internal/session"
)

var ErrInvalidSequence = errors.New("invalid event sequence")

// Controller is the A3/B2 boundary required by a network Session Channel. Every
// command a remote client can issue has to travel through this interface so the
// Channel never reaches the SDK on its own.
type Controller interface {
	Submit(context.Context, projectsession.Input) (string, error)
	Subscribe(context.Context, int) (projectsession.Snapshot, <-chan projectsession.Event, error)
	Snapshot(context.Context) (projectsession.Snapshot, error)
	ResolveAction(context.Context, string, projectsession.ActionDecision, []projectsession.QuestionAnswer) error
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
		return projectsession.ErrSessionMismatch
	}
	return nil
}

func (c *Channel) SubmitText(ctx context.Context, sessionID, text string) (string, error) {
	if err := c.guard(sessionID); err != nil {
		return "", err
	}
	return c.controller.Submit(ctx, projectsession.Input{
		SessionID: c.sessionID,
		Source:    projectsession.InputMobile,
		Text:      text,
	})
}

// Snapshot exposes the current state so a caller can validate a resolution
// against the pending action before asking the Controller to apply it.
func (c *Channel) Snapshot(ctx context.Context, sessionID string) (projectsession.Snapshot, error) {
	if err := c.guard(sessionID); err != nil {
		return projectsession.Snapshot{}, err
	}
	snapshot, err := c.controller.Snapshot(ctx)
	if err != nil {
		return projectsession.Snapshot{}, err
	}
	if snapshot.SessionID != c.sessionID {
		return projectsession.Snapshot{}, projectsession.ErrSessionMismatch
	}
	return snapshot, nil
}

func (c *Channel) ResolveAction(
	ctx context.Context,
	sessionID string,
	actionID string,
	decision projectsession.ActionDecision,
	answers []projectsession.QuestionAnswer,
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
	Snapshot projectsession.Snapshot
	Events   <-chan projectsession.Event
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
		return nil, projectsession.ErrSessionMismatch
	}

	events := make(chan projectsession.Event, capacity)
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
	source <-chan projectsession.Event,
	destination chan<- projectsession.Event,
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
				errorsChannel <- projectsession.ErrSessionMismatch
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
