package transport

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tea4go/claude-mobile-remote/internal/channel"
	"github.com/tea4go/claude-mobile-remote/internal/pairing"
	"github.com/tea4go/claude-mobile-remote/protocol"
)

type connectionState uint8

const (
	stateConnected connectionState = iota
	stateAuthenticated
	stateSubscribed
	stateClosing
)

const (
	// 单条出站事件预算：最大合法 prompt + JSON envelope 余量。普通队列与
	// 提前缓存至少容纳两条该预算的事件，数量上限保持不变。
	outboundEventMaxBytes = protocol.MaxPromptBytes + 64*1024
	outboxMaxItems        = 64
	outboxMaxBytes        = 2 * outboundEventMaxBytes
	earlyMaxItems         = 32
	earlyMaxBytes         = 2 * outboundEventMaxBytes
	readLimitSlack        = 64 * 1024
)

type connection struct {
	server *Server
	socket *websocket.Conn

	context context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	once    sync.Once
	subMu   sync.Mutex

	state               connectionState
	sessionID           string
	lease               *pairing.Lease
	subscriptionChannel *channel.Channel
	subscription        *channel.Subscription
	subscriptionDone    <-chan struct{}
	outbox              *outbox
	gate                *commandGate
	commandIDs          map[string]struct{}
	writerDone          chan struct{}
}

func newConnection(server *Server, socket *websocket.Conn) *connection {
	ctx, cancel := context.WithCancel(context.Background())
	queue := newOutbox(outboxMaxItems, outboxMaxBytes)
	return &connection{
		server:     server,
		socket:     socket,
		context:    ctx,
		cancel:     cancel,
		done:       make(chan struct{}),
		state:      stateConnected,
		outbox:     queue,
		gate:       newCommandGate(queue, earlyMaxItems, earlyMaxBytes),
		commandIDs: make(map[string]struct{}),
		writerDone: make(chan struct{}),
	}
}

func (c *connection) run() {
	defer close(c.done)
	c.socket.SetReadLimit(c.server.config.ReadLimit + readLimitSlack)
	_ = c.socket.SetReadDeadline(time.Now().Add(c.server.config.AuthTimeout))
	go c.writeLoop()
	c.readLoop()
	if c.subscription != nil {
		c.subscription.Cancel()
	}
	if c.lease != nil {
		c.lease.Close()
	}
	c.cancel()
	c.outbox.Close()
	select {
	case <-c.writerDone:
	case <-time.After(c.server.config.CloseTimeout):
	}
	_ = c.socket.Close()
}

func (c *connection) readLoop() {
	for {
		messageType, data, err := c.socket.ReadMessage()
		if err != nil {
			return
		}
		if int64(len(data)) > c.server.config.ReadLimit {
			c.fail(CodeMessageTooLarge, "message is too large")
			return
		}
		if messageType != websocket.TextMessage {
			c.fail(CodeInvalidMessage, "text messages are required")
			return
		}
		envelope, err := DecodeEnvelope(data)
		if err != nil {
			c.fail(CodeInvalidJSON, "invalid JSON message")
			return
		}
		if err := ValidateEnvelope(envelope); err != nil {
			var protocolErr *ProtocolError
			if errors.As(err, &protocolErr) {
				c.fail(protocolErr.Code, protocolErr.Message)
			} else {
				c.fail(CodeInvalidMessage, "invalid protocol message")
			}
			return
		}
		switch c.state {
		case stateConnected:
			if envelope.Type != TypeHello {
				c.fail(CodeInvalidState, "hello is required")
				return
			}
			if !c.handleHello(envelope) {
				return
			}
		case stateAuthenticated:
			if envelope.Type != TypeSubscribe || !c.handleSubscribe(envelope) {
				c.fail(CodeInvalidState, "invalid connection state")
				return
			}
		case stateSubscribed:
			if envelope.Type == TypeUnsubscribe {
				if !c.handleUnsubscribe(envelope) {
					return
				}
				continue
			}
			if envelope.Type != TypeCommand {
				c.fail(CodeInvalidState, "message is not allowed while subscribed")
				return
			}
			if !c.handleCommand(envelope) {
				return
			}
		default:
			return
		}
	}
}

func (c *connection) handleUnsubscribe(envelope Envelope) bool {
	if envelope.SessionID != c.sessionID || c.subscription == nil {
		c.fail(CodeInvalidState, "invalid connection state")
		return false
	}
	c.subMu.Lock()
	pumpDone := c.subscriptionDone
	c.subscription.Cancel()
	c.subMu.Unlock()
	if pumpDone != nil {
		select {
		case <-pumpDone:
		case <-c.context.Done():
			return false
		}
	}
	c.subMu.Lock()
	c.subscriptionDone = nil
	c.subMu.Unlock()
	c.subscription = nil
	c.subscriptionChannel = nil
	c.state = stateAuthenticated
	return c.enqueue(TypeUnsubscribed, "", "", 0, struct{}{}) == nil
}

func (c *connection) handleCommand(envelope Envelope) bool {
	if envelope.SessionID != c.sessionID {
		c.fail(CodeInvalidState, "invalid connection state")
		return false
	}
	var payload CommandPayload
	if err := DecodePayload(envelope, &payload); err != nil {
		c.fail(CodeInvalidCommand, "invalid command")
		return true
	}
	if c.server.isShuttingDown() {
		return c.enqueue(TypeCommandResult, envelope.CommandID, "", 0, CommandResultPayload{
			Accepted: false,
			Code:     CodeSessionUnavailable,
			Message:  "session unavailable",
		}) == nil
	}
	if _, duplicate := c.commandIDs[envelope.CommandID]; duplicate {
		return c.enqueue(TypeCommandResult, envelope.CommandID, "", 0, CommandResultPayload{
			Accepted: false,
			Code:     CodeDuplicateCommandID,
			Message:  "duplicate command ID",
		}) == nil
	}
	c.commandIDs[envelope.CommandID] = struct{}{}
	if err := c.gate.Begin(envelope.CommandID); err != nil {
		c.shutdown()
		return false
	}

	requestID, commandErr := c.dispatchCommand(payload)
	resultPayload := CommandResultPayload{Accepted: commandErr == nil}
	resultRequestID := requestID
	if commandErr != nil {
		resultRequestID = ""
		resultPayload.Code, resultPayload.Message = commandError(commandErr)
	}
	envelopeResult, err := NewEnvelope(TypeCommandResult, c.sessionID, envelope.CommandID, resultRequestID, 0, resultPayload)
	if err != nil {
		c.shutdown()
		return false
	}
	encoded, err := EncodeEnvelope(envelopeResult)
	if err != nil || c.gate.Finish(encoded) != nil {
		c.shutdown()
		return false
	}
	return true
}

// dispatchCommand turns one validated command into exactly one Channel call and
// reports the request ID an accepted result has to acknowledge. Every branch
// goes through the Channel, so the transport never touches the SDK or the
// pending permission callback itself.
func (c *connection) dispatchCommand(payload CommandPayload) (string, error) {
	switch payload.Kind {
	case CommandSendText:
		return c.subscriptionChannel.SubmitText(c.context, c.sessionID, payload.Text)
	case CommandResolveAction:
		return c.resolveAction(payload)
	case CommandCancelTurn:
		return payload.RequestID, c.subscriptionChannel.CancelTurn(c.context, c.sessionID, payload.RequestID)
	case CommandCancelQueued:
		return payload.RequestID, c.subscriptionChannel.CancelQueued(c.context, c.sessionID, payload.RequestID)
	default:
		return "", protocolError(CodeInvalidCommand, "unknown command kind")
	}
}

// resolveAction matches the resolution against the action currently pending in
// the session before handing it to the Controller, as required by design §4.5.
// The Controller still re-checks atomically: this pass only keeps malformed or
// stale resolutions away from the permission callback, and it decides which
// request ID the acknowledgement carries.
func (c *connection) resolveAction(payload CommandPayload) (string, error) {
	snapshot, err := c.subscriptionChannel.Snapshot(c.context, c.sessionID)
	if err != nil {
		return "", err
	}
	action := snapshot.Turn.PendingAction
	if action == nil {
		return "", protocolError(CodeActionNotPending, "action is no longer pending")
	}
	if err := ValidateActionResolution(*action, c.server.config.ServiceConfig, payload); err != nil {
		return "", err
	}
	decision, answers := sessionResolution(payload)
	if err := c.subscriptionChannel.ResolveAction(c.context, c.sessionID, payload.ActionID, decision, answers); err != nil {
		return "", err
	}
	return action.RequestID, nil
}

// sessionResolution lifts the resolution out of the command payload. The types
// now come straight from protocol, so there is no enum translation left; it only
// deep-copies the answers so a later mutation of the payload cannot alias the
// slice handed to the Controller.
func sessionResolution(payload CommandPayload) (protocol.ActionDecision, []protocol.QuestionAnswer) {
	if payload.Decision != "" {
		return payload.Decision, nil
	}
	if len(payload.Answers) == 0 {
		return "", nil
	}
	answers := make([]protocol.QuestionAnswer, 0, len(payload.Answers))
	for _, answer := range payload.Answers {
		answers = append(answers, protocol.QuestionAnswer{
			QuestionID: answer.QuestionID,
			Values:     append([]string(nil), answer.Values...),
		})
	}
	return "", answers
}

func commandError(err error) (string, string) {
	// A ProtocolError already carries the exact §4.5 code decided upstream, so it
	// must win over the sentinel mapping below.
	var protocolErr *ProtocolError
	if errors.As(err, &protocolErr) {
		return protocolErr.Code, protocolErr.Message
	}
	switch {
	case errors.Is(err, protocol.ErrInvalidInput):
		return CodeInvalidCommand, "invalid command"
	case errors.Is(err, protocol.ErrQueueFull):
		return CodeQueueFull, "input queue full"
	case errors.Is(err, protocol.ErrActionNotPending):
		return CodeActionNotPending, "action is no longer pending"
	case errors.Is(err, protocol.ErrInvalidAction):
		return CodeInvalidAction, "invalid action resolution"
	case errors.Is(err, protocol.ErrInvalidAnswer):
		return CodeInvalidAnswer, "invalid answer"
	case errors.Is(err, protocol.ErrTurnNotCancellable):
		return CodeTurnNotCancellable, "turn cannot be cancelled"
	case errors.Is(err, protocol.ErrRequestNotQueued):
		return CodeRequestNotQueued, "request is not queued"
	case errors.Is(err, protocol.ErrSessionMismatch), errors.Is(err, protocol.ErrSessionUnavailable):
		return CodeSessionUnavailable, "session unavailable"
	default:
		return CodeInternal, "internal error"
	}
}

func (c *connection) handleHello(envelope Envelope) bool {
	var payload HelloPayload
	if DecodePayload(envelope, &payload) != nil {
		c.fail(CodeUnauthorized, "unauthorized")
		return false
	}
	lease, err := c.server.authenticator.Authenticate(envelope.SessionID, payload.PairToken)
	if err != nil {
		c.fail(CodeUnauthorized, "unauthorized")
		return false
	}
	c.lease = lease
	c.sessionID = envelope.SessionID
	c.state = stateAuthenticated
	_ = c.socket.SetReadDeadline(time.Time{})
	go func() {
		select {
		case <-lease.Done():
			c.shutdown()
		case <-c.context.Done():
		}
	}()
	return c.enqueue(TypeHelloAck, "", "", 0, struct{}{}) == nil
}

func (c *connection) handleSubscribe(envelope Envelope) bool {
	if envelope.SessionID != c.sessionID {
		return false
	}
	sessionChannel, ok := c.server.registry.Get(envelope.SessionID)
	if !ok {
		return false
	}
	subscription, err := sessionChannel.OpenSubscription(c.context, outboxMaxItems)
	if err != nil {
		return false
	}
	c.subscriptionChannel = sessionChannel
	c.subscription = subscription
	c.state = stateSubscribed
	if err := c.enqueue(TypeSubscribed, "", "", 0, struct{}{}); err != nil {
		return false
	}
	if err := c.enqueue(TypeSnapshot, "", "", subscription.Snapshot.Seq, SnapshotPayload{
		ConnectionStatus: "subscribed",
		Snapshot:         subscription.Snapshot,
	}); err != nil {
		return false
	}
	pumpDone := make(chan struct{})
	c.subMu.Lock()
	c.subscriptionDone = pumpDone
	c.subMu.Unlock()
	go func() {
		defer close(pumpDone)
		c.forwardSubscription(subscription)
	}()
	return true
}

func (c *connection) forwardSubscription(subscription *channel.Subscription) {
	events := subscription.Events
	errorsChannel := subscription.Errors
	for events != nil || errorsChannel != nil {
		select {
		case <-c.context.Done():
			return
		case _, ok := <-errorsChannel:
			if !ok {
				errorsChannel = nil
				continue
			}
			c.shutdown()
			return
		case event, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			envelope, err := c.outboundEvent(event)
			if err != nil {
				c.shutdown()
				return
			}
			data, err := EncodeEnvelope(envelope)
			if err != nil || c.gate.EnqueueEvent(data) != nil {
				c.shutdown()
				return
			}
		}
	}
}

// outboundEvent maps one domain event onto the wire message that carries it.
// The unsupported notice owns a separate envelope type (B2 §4.2), so it is
// translated here rather than pushed through validateEvent, which refuses it as
// an ordinary event by design; only the fixed code travels, never the event's
// session fields. Every other kind keeps the outbound semantics check: an
// illegal event is an internal error and only closes this connection.
func (c *connection) outboundEvent(event protocol.Event) (Envelope, error) {
	if event.Kind == protocol.EventUnsupported {
		if !event.Code.IsKnown() {
			return Envelope{}, ErrInvalidMessage
		}
		return NewEnvelope(TypeUnsupported, c.sessionID, "", event.RequestID, event.Seq, UnsupportedPayload{Code: event.Code})
	}
	if err := validateEvent(event); err != nil {
		return Envelope{}, err
	}
	return NewEnvelope(TypeEvent, c.sessionID, "", event.RequestID, event.Seq, EventPayload{Event: event})
}

func (c *connection) enqueue(messageType MessageType, commandID, requestID string, seq uint64, payload any) error {
	envelope, err := NewEnvelope(messageType, c.sessionID, commandID, requestID, seq, payload)
	if err != nil {
		return err
	}
	data, err := EncodeEnvelope(envelope)
	if err != nil {
		return err
	}
	return c.outbox.Enqueue(data)
}

func (c *connection) fail(code, message string) {
	_ = c.enqueue(TypeError, "", "", 0, newErrorPayload(code, message))
}

func (c *connection) writeLoop() {
	defer close(c.writerDone)
	for {
		data, ok := c.outbox.Next()
		if !ok {
			return
		}
		if err := c.socket.WriteMessage(websocket.TextMessage, data); err != nil {
			return
		}
	}
}

func (c *connection) shutdown() {
	c.once.Do(func() {
		c.cancel()
		_ = c.socket.Close()
	})
}

func (c *connection) gracefulShutdown(ctx context.Context) error {
	c.subMu.Lock()
	pumpDone := c.subscriptionDone
	c.subMu.Unlock()
	if pumpDone != nil {
		select {
		case <-pumpDone:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	c.outbox.Close()
	select {
	case <-c.writerDone:
	case <-ctx.Done():
		return ctx.Err()
	}
	c.once.Do(func() {
		c.cancel()
		_ = c.socket.Close()
	})
	return nil
}
