package transport

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tea4go/claude-mobile-remote/internal/channel"
	"github.com/tea4go/claude-mobile-remote/internal/pairing"
	projectsession "github.com/tea4go/claude-mobile-remote/internal/session"
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
	outboundEventMaxBytes = projectsession.MaxPromptBytes + 64*1024
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

	requestID, submitErr := c.subscriptionChannel.SubmitText(c.context, payload.Text)
	resultPayload := CommandResultPayload{Accepted: submitErr == nil}
	resultRequestID := requestID
	if submitErr != nil {
		resultRequestID = ""
		resultPayload.Code, resultPayload.Message = commandError(submitErr)
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

func commandError(err error) (string, string) {
	switch {
	case errors.Is(err, projectsession.ErrInvalidInput):
		return CodeInvalidCommand, "invalid command"
	case errors.Is(err, projectsession.ErrQueueFull):
		return CodeQueueFull, "input queue full"
	case errors.Is(err, projectsession.ErrSessionMismatch), errors.Is(err, projectsession.ErrSessionUnavailable):
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
			// 出站事件同样受 kind 语义约束：不合法即视为内部错误，仅关闭本连接。
			if err := validateEvent(event); err != nil {
				c.shutdown()
				return
			}
			envelope, err := NewEnvelope(TypeEvent, c.sessionID, "", event.RequestID, event.Seq, EventPayload{Event: event})
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
	_ = c.enqueue(TypeError, "", "", 0, ErrorPayload{Code: code, Message: message})
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
