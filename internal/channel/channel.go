package channel

import (
	"context"
	"errors"

	projectsession "github.com/tea4go/claude-mobile-remote/internal/session"
)

var ErrInvalidSequence = errors.New("invalid event sequence")

// Controller is the A3 boundary required by a network Session Channel.
type Controller interface {
	Submit(context.Context, projectsession.Input) (string, error)
	Subscribe(context.Context, int) (projectsession.Snapshot, <-chan projectsession.Event, error)
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

func (c *Channel) SubmitText(ctx context.Context, text string) (string, error) {
	return c.controller.Submit(ctx, projectsession.Input{
		SessionID: c.sessionID,
		Source:    projectsession.InputMobile,
		Text:      text,
	})
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
