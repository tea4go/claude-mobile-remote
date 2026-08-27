package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tea4go/claude-mobile-remote/internal/channel"
	"github.com/tea4go/claude-mobile-remote/internal/pairing"
	projectsession "github.com/tea4go/claude-mobile-remote/internal/session"
)

type transportResolveCall struct {
	actionID string
	decision projectsession.ActionDecision
	answers  []projectsession.QuestionAnswer
}

type transportFakeController struct {
	mu          sync.Mutex
	snapshot    projectsession.Snapshot
	inputs      []projectsession.Input
	submit      func(projectsession.Input) (string, error)
	subscribers map[chan projectsession.Event]<-chan struct{}

	resolves   []transportResolveCall
	cancelled  []string
	dequeued   []string
	resolve    func(string) error
	resolveErr error
	cancelErr  error
	dequeueErr error
}

func (f *transportFakeController) Snapshot(context.Context) (projectsession.Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.snapshot, nil
}

func (f *transportFakeController) ResolveAction(_ context.Context, actionID string, decision projectsession.ActionDecision, answers []projectsession.QuestionAnswer) error {
	f.mu.Lock()
	f.resolves = append(f.resolves, transportResolveCall{actionID: actionID, decision: decision, answers: answers})
	hook := f.resolve
	err := f.resolveErr
	f.mu.Unlock()
	if hook != nil {
		return hook(actionID)
	}
	return err
}

func (f *transportFakeController) CancelTurn(_ context.Context, requestID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancelled = append(f.cancelled, requestID)
	return f.cancelErr
}

func (f *transportFakeController) CancelQueued(_ context.Context, requestID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dequeued = append(f.dequeued, requestID)
	return f.dequeueErr
}

func (f *transportFakeController) resolutions() []transportResolveCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]transportResolveCall(nil), f.resolves...)
}

func (f *transportFakeController) setPendingAction(action *projectsession.PendingAction) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.snapshot.Turn.Status = projectsession.TurnWaiting
	f.snapshot.Turn.PendingAction = action
	if action != nil {
		f.snapshot.Turn.RequestID = action.RequestID
	}
}

func (f *transportFakeController) Submit(_ context.Context, input projectsession.Input) (string, error) {
	f.mu.Lock()
	f.inputs = append(f.inputs, input)
	submit := f.submit
	f.mu.Unlock()
	if submit != nil {
		return submit(input)
	}
	return "request-1", nil
}

func (f *transportFakeController) Subscribe(ctx context.Context, _ int) (projectsession.Snapshot, <-chan projectsession.Event, error) {
	channel := make(chan projectsession.Event, 16)
	f.mu.Lock()
	if f.subscribers == nil {
		f.subscribers = make(map[chan projectsession.Event]<-chan struct{})
	}
	f.subscribers[channel] = ctx.Done()
	snapshot := f.snapshot
	f.mu.Unlock()
	go func() {
		<-ctx.Done()
		f.mu.Lock()
		if _, exists := f.subscribers[channel]; exists {
			delete(f.subscribers, channel)
			close(channel)
		}
		f.mu.Unlock()
	}()
	return snapshot, channel, nil
}

func (f *transportFakeController) finishEvents() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for subscriber := range f.subscribers {
		delete(f.subscribers, subscriber)
		close(subscriber)
	}
}

func (f *transportFakeController) emit(event projectsession.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for subscriber, done := range f.subscribers {
		select {
		case <-done:
		case subscriber <- event:
		}
	}
}

type serverFixture struct {
	testingServer *httptest.Server
	server        *Server
	manager       *pairing.Manager
	token         string
	controller    *transportFakeController
}

func newServerFixture(t *testing.T, configure func(*Config)) *serverFixture {
	t.Helper()
	manager, err := pairing.NewManager(
		"ws://pc/ws",
		time.Hour,
		pairing.WithRandom(bytes.NewReader(bytes.Repeat([]byte{0x55}, 32))),
	)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := manager.Issue("session-1")
	if err != nil {
		t.Fatal(err)
	}
	controller := &transportFakeController{
		snapshot: projectsession.Snapshot{SessionID: "session-1", SessionStatus: projectsession.SessionReady, Seq: 7},
	}
	registry := channel.NewRegistry()
	if err := registry.Register(channel.New("session-1", controller)); err != nil {
		t.Fatal(err)
	}
	config := DefaultConfig()
	if configure != nil {
		configure(&config)
	}
	server, err := NewServer(config, manager, registry)
	if err != nil {
		t.Fatal(err)
	}
	testingServer := httptest.NewServer(server.Handler())
	fixture := &serverFixture{testingServer: testingServer, server: server, manager: manager, token: payload.PairToken, controller: controller}
	t.Cleanup(func() {
		testingServer.Close()
		closeContext, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Close(closeContext)
	})
	return fixture
}

func (f *serverFixture) dial(t *testing.T) *websocket.Conn {
	t.Helper()
	connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(f.testingServer.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	return connection
}

func (f *serverFixture) dialWithOrigin(t *testing.T, origin string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	header := http.Header{}
	if origin != "" {
		header.Set("Origin", origin)
	}
	return websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(f.testingServer.URL, "http")+"/ws", header)
}

func writeTestEnvelope(t *testing.T, connection *websocket.Conn, messageType MessageType, sessionID, commandID, requestID string, seq uint64, payload any) {
	t.Helper()
	envelope, err := NewEnvelope(messageType, sessionID, commandID, requestID, seq, payload)
	if err != nil {
		t.Fatal(err)
	}
	data, err := EncodeEnvelope(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if err := connection.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("WriteMessage() error = %v", err)
	}
}

func readTestEnvelope(t *testing.T, connection *websocket.Conn) Envelope {
	t.Helper()
	_ = connection.SetReadDeadline(time.Now().Add(time.Second))
	_, data, err := connection.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	envelope, err := DecodeEnvelope(data)
	if err != nil {
		t.Fatalf("DecodeEnvelope() error = %v", err)
	}
	return envelope
}

func authenticateAndSubscribe(t *testing.T, fixture *serverFixture, connection *websocket.Conn) Envelope {
	t.Helper()
	writeTestEnvelope(t, connection, TypeHello, "session-1", "", "", 0, HelloPayload{PairToken: fixture.token})
	if response := readTestEnvelope(t, connection); response.Type != TypeHelloAck {
		t.Fatalf("hello response = %#v", response)
	}
	writeTestEnvelope(t, connection, TypeSubscribe, "session-1", "", "", 0, struct{}{})
	if response := readTestEnvelope(t, connection); response.Type != TypeSubscribed {
		t.Fatalf("subscribe response = %#v", response)
	}
	return readTestEnvelope(t, connection)
}

func TestWebSocketAuthenticateSubscribeSnapshot(t *testing.T) {
	fixture := newServerFixture(t, nil)
	connection := fixture.dial(t)

	writeTestEnvelope(t, connection, TypeHello, "session-1", "", "", 0, HelloPayload{PairToken: fixture.token})
	if got := readTestEnvelope(t, connection); got.Type != TypeHelloAck {
		t.Fatalf("first response type = %q, want hello_ack", got.Type)
	}
	writeTestEnvelope(t, connection, TypeSubscribe, "session-1", "", "", 0, struct{}{})
	if got := readTestEnvelope(t, connection); got.Type != TypeSubscribed {
		t.Fatalf("second response type = %q, want subscribed", got.Type)
	}
	snapshot := readTestEnvelope(t, connection)
	if snapshot.Type != TypeSnapshot || snapshot.Seq != 7 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}

func TestWebSocketRejectsInvalidCredentialWithoutEcho(t *testing.T) {
	fixture := newServerFixture(t, nil)
	connection := fixture.dial(t)
	invalidToken := "credential-that-must-not-be-echoed"
	writeTestEnvelope(t, connection, TypeHello, "session-1", "", "", 0, HelloPayload{PairToken: invalidToken})
	response := readTestEnvelope(t, connection)
	if response.Type != TypeError {
		t.Fatalf("response type = %q, want error", response.Type)
	}
	var payload ErrorPayload
	if err := DecodePayload(response, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Code != CodeUnauthorized || strings.Contains(payload.Message, invalidToken) {
		t.Fatalf("unauthorized payload = %#v", payload)
	}
}

func TestWebSocketRequiresHelloBeforeTimeout(t *testing.T) {
	fixture := newServerFixture(t, func(config *Config) { config.AuthTimeout = 20 * time.Millisecond })
	connection := fixture.dial(t)
	_ = connection.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err := connection.ReadMessage(); err == nil {
		t.Fatal("connection remained open without hello")
	}
}

func TestWebSocketRejectsMessageBeforeHello(t *testing.T) {
	fixture := newServerFixture(t, nil)
	connection := fixture.dial(t)
	writeTestEnvelope(t, connection, TypeSubscribe, "session-1", "", "", 0, struct{}{})
	response := readTestEnvelope(t, connection)
	var payload ErrorPayload
	if response.Type != TypeError || DecodePayload(response, &payload) != nil || payload.Code != CodeInvalidState {
		t.Fatalf("response = %#v, payload = %#v", response, payload)
	}
}

func TestWebSocketOriginPolicy(t *testing.T) {
	fixture := newServerFixture(t, nil)
	allowed, _, err := fixture.dialWithOrigin(t, "http://127.0.0.1:5173")
	if err != nil {
		t.Fatalf("allowed origin Dial() error = %v", err)
	}
	_ = allowed.Close()
	denied, response, err := fixture.dialWithOrigin(t, "https://untrusted.example")
	if denied != nil {
		_ = denied.Close()
	}
	if err == nil || response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("denied origin = connection:%v response:%v error:%v", denied, response, err)
	}
}

func TestWebSocketReturnsStableMessageTooLargeError(t *testing.T) {
	fixture := newServerFixture(t, func(config *Config) { config.ReadLimit = 128 })
	connection := fixture.dial(t)
	data := bytes.Repeat([]byte{'x'}, 256)
	if err := connection.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("WriteMessage() error = %v", err)
	}
	response := readTestEnvelope(t, connection)
	var payload ErrorPayload
	if response.Type != TypeError || DecodePayload(response, &payload) != nil || payload.Code != CodeMessageTooLarge {
		t.Fatalf("response = %#v, payload = %#v", response, payload)
	}
}

func TestWebSocketCommandResultPrecedesTurnEvent(t *testing.T) {
	fixture := newServerFixture(t, nil)
	connection := fixture.dial(t)
	writeTestEnvelope(t, connection, TypeHello, "session-1", "", "", 0, HelloPayload{PairToken: fixture.token})
	_ = readTestEnvelope(t, connection)
	writeTestEnvelope(t, connection, TypeSubscribe, "session-1", "", "", 0, struct{}{})
	_ = readTestEnvelope(t, connection)
	_ = readTestEnvelope(t, connection)

	fixture.controller.submit = func(projectsession.Input) (string, error) {
		fixture.controller.emit(projectsession.Event{
			SessionID: "session-1", RequestID: "request-1", Seq: 8,
			Kind: projectsession.EventTurnStarted, Source: projectsession.InputMobile,
			Text: "hello", TurnStatus: projectsession.TurnRunning,
		})
		return "request-1", nil
	}
	writeTestEnvelope(t, connection, TypeCommand, "session-1", "command-1", "", 0, CommandPayload{Kind: CommandSendText, Text: "hello"})
	result := readTestEnvelope(t, connection)
	if result.Type != TypeCommandResult || result.CommandID != "command-1" || result.RequestID != "request-1" {
		t.Fatalf("first command response = %#v", result)
	}
	event := readTestEnvelope(t, connection)
	if event.Type != TypeEvent || event.RequestID != "request-1" || event.Seq != 8 {
		t.Fatalf("second command response = %#v", event)
	}

	writeTestEnvelope(t, connection, TypeCommand, "session-1", "command-1", "", 0, CommandPayload{Kind: CommandSendText, Text: "again"})
	duplicate := readTestEnvelope(t, connection)
	var duplicatePayload CommandResultPayload
	if duplicate.Type != TypeCommandResult || DecodePayload(duplicate, &duplicatePayload) != nil || duplicatePayload.Code != CodeDuplicateCommandID {
		t.Fatalf("duplicate response = %#v, payload = %#v", duplicate, duplicatePayload)
	}
	fixture.controller.mu.Lock()
	defer fixture.controller.mu.Unlock()
	if len(fixture.controller.inputs) != 1 || fixture.controller.inputs[0].Source != projectsession.InputMobile {
		t.Fatalf("submitted inputs = %#v", fixture.controller.inputs)
	}
}

func TestWebSocketUnsubscribeAndResubscribe(t *testing.T) {
	fixture := newServerFixture(t, nil)
	connection := fixture.dial(t)
	writeTestEnvelope(t, connection, TypeHello, "session-1", "", "", 0, HelloPayload{PairToken: fixture.token})
	_ = readTestEnvelope(t, connection)
	writeTestEnvelope(t, connection, TypeSubscribe, "session-1", "", "", 0, struct{}{})
	_ = readTestEnvelope(t, connection)
	_ = readTestEnvelope(t, connection)

	writeTestEnvelope(t, connection, TypeUnsubscribe, "session-1", "", "", 0, struct{}{})
	if response := readTestEnvelope(t, connection); response.Type != TypeUnsubscribed {
		t.Fatalf("unsubscribe response type = %q", response.Type)
	}
	writeTestEnvelope(t, connection, TypeSubscribe, "session-1", "", "", 0, struct{}{})
	if response := readTestEnvelope(t, connection); response.Type != TypeSubscribed {
		t.Fatalf("resubscribe response type = %q", response.Type)
	}
	if response := readTestEnvelope(t, connection); response.Type != TypeSnapshot || response.Seq != 7 {
		t.Fatalf("resubscribe snapshot = %#v", response)
	}
}

func TestWebSocketMultipleClientsReceiveSameEvent(t *testing.T) {
	fixture := newServerFixture(t, nil)
	first := fixture.dial(t)
	second := fixture.dial(t)
	_ = authenticateAndSubscribe(t, fixture, first)
	_ = authenticateAndSubscribe(t, fixture, second)

	fixture.controller.emit(projectsession.Event{
		SessionID: "session-1", RequestID: "request-1", Seq: 8,
		Kind: projectsession.EventTurnStarted, Source: projectsession.InputTerminal,
		Text: "from terminal", TurnStatus: projectsession.TurnRunning,
	})
	for name, connection := range map[string]*websocket.Conn{"first": first, "second": second} {
		event := readTestEnvelope(t, connection)
		if event.Type != TypeEvent || event.Seq != 8 {
			t.Fatalf("%s event = %#v", name, event)
		}
	}
}

func TestWebSocketTwoPhonesSubmitConcurrently(t *testing.T) {
	fixture := newServerFixture(t, nil)
	first := fixture.dial(t)
	second := fixture.dial(t)
	_ = authenticateAndSubscribe(t, fixture, first)
	_ = authenticateAndSubscribe(t, fixture, second)

	// 两个连接近乎同时发送不同命令。
	results := make(chan Envelope, 2)
	for name, item := range map[string]struct {
		connection *websocket.Conn
		commandID  string
	}{
		"first":  {connection: first, commandID: "command-1"},
		"second": {connection: second, commandID: "command-2"},
	} {
		go func(name string, connection *websocket.Conn, commandID string) {
			_ = connection.SetWriteDeadline(time.Now().Add(3 * time.Second))
			writeTestEnvelope(t, connection, TypeCommand, "session-1", commandID, "", 0, CommandPayload{Kind: CommandSendText, Text: "from " + name})
			_ = connection.SetReadDeadline(time.Now().Add(3 * time.Second))
			result := readTestEnvelope(t, connection)
			results <- result
		}(name, item.connection, item.commandID)
	}

	seen := map[string]Envelope{}
	for range 2 {
		result := <-results
		seen[result.CommandID] = result
	}
	if len(seen) != 2 {
		t.Fatalf("command results = %#v, want two distinct", seen)
	}
	fixture.controller.mu.Lock()
	defer fixture.controller.mu.Unlock()
	if len(fixture.controller.inputs) != 2 {
		t.Fatalf("controller inputs = %#v, want two", fixture.controller.inputs)
	}
	texts := map[string]bool{}
	for _, input := range fixture.controller.inputs {
		if input.Source != projectsession.InputMobile {
			t.Fatalf("input source = %q, want mobile", input.Source)
		}
		texts[input.Text] = true
	}
	if !texts["from first"] || !texts["from second"] {
		t.Fatalf("input texts = %#v, want both phones' texts", texts)
	}
}

func TestWebSocketAllSubscribersObserveSameTurnOrder(t *testing.T) {
	fixture := newServerFixture(t, nil)
	first := fixture.dial(t)
	second := fixture.dial(t)
	_ = authenticateAndSubscribe(t, fixture, first)
	_ = authenticateAndSubscribe(t, fixture, second)

	// 按 Controller 接受顺序广播两个递增 seq 的 turn_started。
	fixture.controller.emit(projectsession.Event{
		SessionID: "session-1", RequestID: "request-1", Seq: 8,
		Kind: projectsession.EventTurnStarted, Source: projectsession.InputMobile,
		Text: "from first", TurnStatus: projectsession.TurnRunning,
	})
	fixture.controller.emit(projectsession.Event{
		SessionID: "session-1", RequestID: "request-2", Seq: 9,
		Kind: projectsession.EventTurnStarted, Source: projectsession.InputMobile,
		Text: "from second", TurnStatus: projectsession.TurnRunning,
	})
	for name, connection := range map[string]*websocket.Conn{"first": first, "second": second} {
		for wantSeq, wantText := range []string{"from first", "from second"} {
			event := readTestEnvelope(t, connection)
			var payload EventPayload
			if event.Type != TypeEvent || event.Seq != uint64(8+wantSeq) ||
				DecodePayload(event, &payload) != nil || payload.Event.Text != wantText ||
				payload.Event.Source != projectsession.InputMobile {
				t.Fatalf("%s event = %#v, want seq %d text %q mobile", name, event, 8+wantSeq, wantText)
			}
		}
	}
}

func TestWebSocketMaximumPromptEventDoesNotCloseConnection(t *testing.T) {
	fixture := newServerFixture(t, nil)
	connection := fixture.dial(t)
	_ = authenticateAndSubscribe(t, fixture, connection)

	fixture.controller.emit(projectsession.Event{
		SessionID: "session-1", RequestID: "request-1", Seq: 8,
		Kind:       projectsession.EventTurnStarted,
		Source:     projectsession.InputTerminal,
		Text:       strings.Repeat("x", projectsession.MaxPromptBytes),
		TurnStatus: projectsession.TurnRunning,
	})
	big := readTestEnvelope(t, connection)
	if big.Type != TypeEvent || big.Seq != 8 {
		t.Fatalf("big event = %#v, want event seq 8", big)
	}

	// 同一连接继续接收下一条小事件，证明未被容量关闭。
	fixture.controller.emit(projectsession.Event{
		SessionID: "session-1", RequestID: "request-2", Seq: 9,
		Kind: projectsession.EventTextDelta,
		Text: "still connected",
	})
	small := readTestEnvelope(t, connection)
	if small.Type != TypeEvent || small.Seq != 9 {
		t.Fatalf("small event = %#v, want event seq 9", small)
	}
}

func TestWebSocketSlowClientIsolation(t *testing.T) {
	fixture := newServerFixture(t, nil)
	fast := fixture.dial(t)
	slow := fixture.dial(t)
	_ = authenticateAndSubscribe(t, fixture, fast)
	_ = authenticateAndSubscribe(t, fixture, slow)

	// fast 连接持续读取，避免它自己的 outbox 溢出；读取循环一直跑到底，
	// gorilla 连接不允许并发读取，最终断言也在同一 goroutine 里做。
	fast.SetReadLimit(4 * 1024 * 1024)
	type readOutcome struct {
		envelope Envelope
		err      error
	}
	outcomes := make(chan readOutcome, 64)
	go func() {
		for {
			_, data, err := fast.ReadMessage()
			if err != nil {
				outcomes <- readOutcome{err: err}
				return
			}
			envelope, decodeErr := DecodeEnvelope(data)
			outcomes <- readOutcome{envelope: envelope, err: decodeErr}
		}
	}()

	// 反复入队大事件，直至不再读取的慢连接触发字节上限被关闭。
	for i := range 16 {
		fixture.controller.emit(projectsession.Event{
			SessionID: "session-1", RequestID: "request-1", Seq: uint64(8 + i),
			Kind:       projectsession.EventTurnStarted,
			Source:     projectsession.InputTerminal,
			Text:       strings.Repeat("x", 512*1024),
			TurnStatus: projectsession.TurnRunning,
		})
		// 慢连接的关闭路径需要时间收敛；观察它是否已被移除。
		fixture.controller.mu.Lock()
		slowRemoved := len(fixture.controller.subscribers) < 2
		fixture.controller.mu.Unlock()
		if slowRemoved {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// fast 连接必须仍然可用：发出一条小事件并读到它。
	deadline := time.After(3 * time.Second)
	fixture.controller.emit(projectsession.Event{
		SessionID: "session-1", RequestID: "request-2", Seq: 100,
		Kind: projectsession.EventTextDelta, Text: "fast alive",
	})
	for {
		select {
		case outcome := <-outcomes:
			if outcome.err != nil {
				t.Fatalf("fast connection closed: %v", outcome.err)
			}
			if outcome.envelope.Type == TypeError {
				t.Fatalf("fast connection got error envelope: %#v", outcome.envelope)
			}
			if outcome.envelope.Seq == 100 {
				return
			}
		case <-deadline:
			t.Fatal("fast connection never received the sentinel event")
		}
	}
}

func TestWebSocketReconnectReceivesLatestSnapshotOnly(t *testing.T) {
	fixture := newServerFixture(t, nil)
	first := fixture.dial(t)
	if snapshot := authenticateAndSubscribe(t, fixture, first); snapshot.Seq != 7 {
		t.Fatalf("first snapshot seq = %d", snapshot.Seq)
	}
	_ = first.Close()

	fixture.controller.mu.Lock()
	fixture.controller.snapshot.Seq = 9
	fixture.controller.mu.Unlock()
	second := fixture.dial(t)
	if snapshot := authenticateAndSubscribe(t, fixture, second); snapshot.Seq != 9 {
		t.Fatalf("reconnected snapshot seq = %d, want 9", snapshot.Seq)
	}
}

func TestWebSocketRevocationClosesAuthenticatedConnection(t *testing.T) {
	fixture := newServerFixture(t, nil)
	connection := fixture.dial(t)
	writeTestEnvelope(t, connection, TypeHello, "session-1", "", "", 0, HelloPayload{PairToken: fixture.token})
	_ = readTestEnvelope(t, connection)
	fixture.manager.Revoke()
	_ = connection.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err := connection.ReadMessage(); err == nil {
		t.Fatal("revoked connection remained open")
	}
}

func TestWebSocketBeginShutdownRejectsNewCommand(t *testing.T) {
	fixture := newServerFixture(t, nil)
	connection := fixture.dial(t)
	_ = authenticateAndSubscribe(t, fixture, connection)
	fixture.server.BeginShutdown()
	writeTestEnvelope(t, connection, TypeCommand, "session-1", "command-1", "", 0, CommandPayload{Kind: CommandSendText, Text: "hello"})
	result := readTestEnvelope(t, connection)
	var payload CommandResultPayload
	if result.Type != TypeCommandResult || DecodePayload(result, &payload) != nil || payload.Code != CodeSessionUnavailable {
		t.Fatalf("shutdown command result = %#v, payload = %#v", result, payload)
	}
	fixture.controller.mu.Lock()
	defer fixture.controller.mu.Unlock()
	if len(fixture.controller.inputs) != 0 {
		t.Fatalf("shutdown submitted inputs = %#v", fixture.controller.inputs)
	}
}

func TestWebSocketCloseFlushesFinalSessionEvent(t *testing.T) {
	fixture := newServerFixture(t, nil)
	connection := fixture.dial(t)
	_ = authenticateAndSubscribe(t, fixture, connection)
	fixture.server.BeginShutdown()
	fixture.controller.emit(projectsession.Event{
		SessionID: "session-1", Seq: 8, Kind: projectsession.EventSessionStopped,
		SessionStatus: projectsession.SessionStopped,
	})
	fixture.controller.finishEvents()
	closeContext, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := fixture.server.Close(closeContext); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	finalEvent := readTestEnvelope(t, connection)
	if finalEvent.Type != TypeEvent || finalEvent.Seq != 8 {
		t.Fatalf("final event = %#v", finalEvent)
	}
}

func TestWebSocketCloseForcesEveryConnectionAfterContextCancellation(t *testing.T) {
	fixture := newServerFixture(t, nil)
	connections := []*websocket.Conn{fixture.dial(t), fixture.dial(t)}
	for _, connection := range connections {
		_ = authenticateAndSubscribe(t, fixture, connection)
	}

	closeContext, cancel := context.WithCancel(context.Background())
	cancel()
	if err := fixture.server.Close(closeContext); !errors.Is(err, context.Canceled) {
		t.Fatalf("Close() error = %v, want context.Canceled", err)
	}
	for index, connection := range connections {
		_ = connection.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		if _, _, err := connection.ReadMessage(); err == nil {
			t.Fatalf("connection %d remained readable", index)
		} else if networkError, ok := err.(net.Error); ok && networkError.Timeout() {
			t.Fatalf("connection %d remained open until the read deadline", index)
		}
	}
}

// ---- A5.2 首轮前配对的共用契约回归（设计 §9/§12.1）----

// reservePreTurnSnapshot 把 fixture 改为首轮前状态：sessionId 已预留、Session
// starting、Turn idle、seq=0、queueDepth=0。
func reservePreTurnSnapshot(fixture *serverFixture) {
	fixture.controller.mu.Lock()
	defer fixture.controller.mu.Unlock()
	fixture.controller.snapshot = projectsession.Snapshot{
		SessionID:     "session-1",
		SessionStatus: projectsession.SessionStarting,
		Seq:           0,
		Turn:          projectsession.TurnSnapshot{Status: projectsession.TurnIdle},
	}
}

func TestWebSocketPreTurnSubscribeReceivesStartingSnapshot(t *testing.T) {
	fixture := newServerFixture(t, nil)
	reservePreTurnSnapshot(fixture)
	connection := fixture.dial(t)

	snapshot := authenticateAndSubscribe(t, fixture, connection)
	if snapshot.Type != TypeSnapshot || snapshot.Version != Version || snapshot.Seq != 0 {
		t.Fatalf("pre-turn snapshot envelope = %#v, want version %d snapshot seq 0", snapshot, Version)
	}
	var payload SnapshotPayload
	if err := DecodePayload(snapshot, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Snapshot.SessionID != "session-1" {
		t.Fatalf("pre-turn sessionId = %q, want the reserved real ID", payload.Snapshot.SessionID)
	}
	if payload.Snapshot.SessionStatus != projectsession.SessionStarting {
		t.Fatalf("pre-turn sessionStatus = %q, want starting", payload.Snapshot.SessionStatus)
	}
	if payload.Snapshot.Turn.Status != projectsession.TurnIdle || payload.Snapshot.Turn.RequestID != "" {
		t.Fatalf("pre-turn turn = %#v, want idle without request", payload.Snapshot.Turn)
	}
	if payload.Snapshot.Seq != 0 || payload.Snapshot.QueueDepth != 0 || payload.Snapshot.SessionError != nil {
		t.Fatalf("pre-turn snapshot = %#v, want seq 0, empty queue, no error", payload.Snapshot)
	}
	if payload.Snapshot.QueuedRequests != nil {
		t.Fatalf("pre-turn queuedRequests = %#v, want none", payload.Snapshot.QueuedRequests)
	}

	// 三端共用契约：首轮前 snapshot 不得新增字段或浏览器特有分支。
	// B2 §4.4 新增 queuedRequests，它与 queueDepth 一起构成同一 FIFO 视图。
	var object map[string]any
	if err := json.Unmarshal(snapshot.Payload, &object); err != nil {
		t.Fatal(err)
	}
	inner, ok := object["snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("snapshot payload = %#v", object)
	}
	wantKeys := map[string]bool{
		"sessionId": true, "sessionStatus": true, "sessionError": true,
		"seq": true, "turn": true, "queueDepth": true, "queuedRequests": true,
	}
	if len(inner) != len(wantKeys) {
		t.Fatalf("snapshot keys = %#v, want exactly %#v", inner, wantKeys)
	}
	for key := range inner {
		if !wantKeys[key] {
			t.Fatalf("unexpected snapshot key %q", key)
		}
	}
}

// TestWebSocketPreTurnSnapshotTurnWireShape 固化首轮前 Turn 的 wire 形状。
// 人工验收缺陷：首轮前 turn.source 为空串，而端侧解码器若按“非空枚举”
// 校验 source 就会拒绝整个 snapshot 并断开。该断言是三端共用契约，任何
// 端的解码器都必须接受此形状。
func TestWebSocketPreTurnSnapshotTurnWireShape(t *testing.T) {
	fixture := newServerFixture(t, nil)
	reservePreTurnSnapshot(fixture)
	connection := fixture.dial(t)

	snapshot := authenticateAndSubscribe(t, fixture, connection)
	var payload struct {
		Snapshot struct {
			Turn json.RawMessage `json:"turn"`
		} `json:"snapshot"`
	}
	if err := json.Unmarshal(snapshot.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	const wantTurn = `{"status":"idle","requestId":"","source":"","message":"","error":null,"pendingAction":null}`
	if got := string(payload.Snapshot.Turn); got != wantTurn {
		t.Fatalf("pre-turn turn wire shape = %s, want %s", got, wantTurn)
	}
}

// TestWebSocketPreTurnMobileFirstCommandStreamsToAllSubscribers 验证设计 §6：
// 首条输入来自手机时，发送连接先收 command_result，随后所有订阅端按
// session_started → turn_started → text_delta 收到同一套事件。
func TestWebSocketPreTurnMobileFirstCommandStreamsToAllSubscribers(t *testing.T) {
	fixture := newServerFixture(t, nil)
	reservePreTurnSnapshot(fixture)
	phone := fixture.dial(t)
	observer := fixture.dial(t)
	_ = authenticateAndSubscribe(t, fixture, phone)
	_ = authenticateAndSubscribe(t, fixture, observer)

	// Controller 在 init 确认后才发布生命周期事件，首轮顺序与 A5.1 一致。
	fixture.controller.submit = func(projectsession.Input) (string, error) {
		fixture.controller.emit(projectsession.Event{
			SessionID: "session-1", Seq: 1,
			Kind: projectsession.EventSessionStarted, SessionStatus: projectsession.SessionReady,
		})
		fixture.controller.emit(projectsession.Event{
			SessionID: "session-1", RequestID: "request-1", Seq: 2,
			Kind: projectsession.EventTurnStarted, Source: projectsession.InputMobile,
			Text: "from phone", TurnStatus: projectsession.TurnRunning,
		})
		fixture.controller.emit(projectsession.Event{
			SessionID: "session-1", RequestID: "request-1", Seq: 3,
			Kind: projectsession.EventTextDelta, Text: "hi", TurnStatus: projectsession.TurnRunning,
		})
		return "request-1", nil
	}
	writeTestEnvelope(t, phone, TypeCommand, "session-1", "command-1", "", 0, CommandPayload{Kind: CommandSendText, Text: "from phone"})

	// 发送连接：command_result 仍先于 early 事件（A4 顺序门）。
	result := readTestEnvelope(t, phone)
	if result.Type != TypeCommandResult || result.CommandID != "command-1" || result.RequestID != "request-1" {
		t.Fatalf("first command response = %#v, want command_result", result)
	}

	wantKinds := []projectsession.EventKind{
		projectsession.EventSessionStarted,
		projectsession.EventTurnStarted,
		projectsession.EventTextDelta,
	}
	for name, connection := range map[string]*websocket.Conn{"phone": phone, "observer": observer} {
		for index, wantKind := range wantKinds {
			event := readTestEnvelope(t, connection)
			var payload EventPayload
			if event.Type != TypeEvent || DecodePayload(event, &payload) != nil {
				t.Fatalf("%s message %d = %#v, want event", name, index, event)
			}
			if payload.Event.Kind != wantKind || event.Seq != uint64(index+1) {
				t.Fatalf("%s event %d = kind %q seq %d, want kind %q seq %d",
					name, index, payload.Event.Kind, event.Seq, wantKind, index+1)
			}
			if payload.Event.SessionID != "session-1" {
				t.Fatalf("%s event %d sessionId = %q, want the reserved ID", name, index, payload.Event.SessionID)
			}
		}
	}

	fixture.controller.mu.Lock()
	defer fixture.controller.mu.Unlock()
	if len(fixture.controller.inputs) != 1 || fixture.controller.inputs[0].Source != projectsession.InputMobile {
		t.Fatalf("submitted inputs = %#v, want one mobile input", fixture.controller.inputs)
	}
}

func approvalAction(allowSession bool) *projectsession.PendingAction {
	return &projectsession.PendingAction{
		ActionID:              "request-1-action-1",
		RequestID:             "request-1",
		Kind:                  projectsession.ActionToolApproval,
		ToolName:              "Bash",
		Preview:               "echo hi",
		AllowSessionAvailable: allowSession,
	}
}

func questionAction() *projectsession.PendingAction {
	return &projectsession.PendingAction{
		ActionID:  "request-1-action-1",
		RequestID: "request-1",
		Kind:      projectsession.ActionAskUserQuestion,
		ToolName:  projectsession.AskUserQuestionToolName,
		Questions: []projectsession.Question{
			{
				QuestionID: "request-1-action-1-q1", Header: "Approach", Question: "Which one?",
				InputType: projectsession.QuestionChoice,
				Options:   []projectsession.QuestionOption{{Label: "A"}, {Label: "B"}},
			},
			{
				QuestionID: "request-1-action-1-q2", Header: "Checks", Question: "Which checks?",
				InputType: projectsession.QuestionChoice, MultiSelect: true,
				Options: []projectsession.QuestionOption{{Label: "X"}, {Label: "Y"}},
			},
		},
	}
}

// readCommandResult reads the next message and requires it to be the result of
// the named command, so ordering mistakes surface as test failures instead of
// being skipped over.
func readCommandResult(t *testing.T, connection *websocket.Conn, commandID string) (Envelope, CommandResultPayload) {
	t.Helper()
	envelope := readTestEnvelope(t, connection)
	var payload CommandResultPayload
	if envelope.Type != TypeCommandResult || envelope.CommandID != commandID || DecodePayload(envelope, &payload) != nil {
		t.Fatalf("command response = %#v, want the result of %q", envelope, commandID)
	}
	return envelope, payload
}

// TestWebSocketResolveActionRoutesThroughTheController pins that an approval
// decision reaches the Controller and that the accepted result correlates with
// the request the action belongs to.
func TestWebSocketResolveActionRoutesThroughTheController(t *testing.T) {
	fixture := newServerFixture(t, nil)
	fixture.controller.setPendingAction(approvalAction(true))
	connection := fixture.dial(t)
	_ = authenticateAndSubscribe(t, fixture, connection)

	writeTestEnvelope(t, connection, TypeCommand, "session-1", "command-1", "", 0, CommandPayload{
		Kind: CommandResolveAction, ActionID: "request-1-action-1", Decision: DecisionAllowSession,
	})
	envelope, payload := readCommandResult(t, connection, "command-1")
	if !payload.Accepted || payload.Code != "" || envelope.RequestID != "request-1" {
		t.Fatalf("resolve result = %#v, payload = %#v", envelope, payload)
	}
	want := []transportResolveCall{{actionID: "request-1-action-1", decision: projectsession.DecisionAllowSession}}
	if got := fixture.controller.resolutions(); !reflect.DeepEqual(got, want) {
		t.Fatalf("controller resolutions = %#v, want %#v", got, want)
	}
}

// TestWebSocketResolveActionAcceptsQuestionAnswers checks the answer form of the
// same command, including the multi-select question, and that the values reach
// the Controller unchanged.
func TestWebSocketResolveActionAcceptsQuestionAnswers(t *testing.T) {
	fixture := newServerFixture(t, nil)
	fixture.controller.setPendingAction(questionAction())
	connection := fixture.dial(t)
	_ = authenticateAndSubscribe(t, fixture, connection)

	writeTestEnvelope(t, connection, TypeCommand, "session-1", "command-1", "", 0, CommandPayload{
		Kind: CommandResolveAction, ActionID: "request-1-action-1",
		Answers: []QuestionAnswer{
			{QuestionID: "request-1-action-1-q1", Values: []string{"A"}},
			{QuestionID: "request-1-action-1-q2", Values: []string{"X", "Y"}},
		},
	})
	envelope, payload := readCommandResult(t, connection, "command-1")
	if !payload.Accepted || envelope.RequestID != "request-1" {
		t.Fatalf("resolve result = %#v, payload = %#v", envelope, payload)
	}
	want := []transportResolveCall{{
		actionID: "request-1-action-1",
		answers: []projectsession.QuestionAnswer{
			{QuestionID: "request-1-action-1-q1", Values: []string{"A"}},
			{QuestionID: "request-1-action-1-q2", Values: []string{"X", "Y"}},
		},
	}}
	if got := fixture.controller.resolutions(); !reflect.DeepEqual(got, want) {
		t.Fatalf("controller resolutions = %#v, want %#v", got, want)
	}
}

// TestWebSocketResolveActionRejectionsNeverReachTheController keeps the pending
// action the single authority for what a decision may say. Every rejection is a
// business outcome: it answers with a stable code and leaves the Session alone.
func TestWebSocketResolveActionRejectionsNeverReachTheController(t *testing.T) {
	tests := []struct {
		name     string
		action   *projectsession.PendingAction
		payload  CommandPayload
		wantCode string
	}{
		{
			name:     "allow_session was not offered",
			action:   approvalAction(false),
			payload:  CommandPayload{Kind: CommandResolveAction, ActionID: "request-1-action-1", Decision: DecisionAllowSession},
			wantCode: CodeInvalidAction,
		},
		{
			name:     "no action is pending",
			action:   nil,
			payload:  CommandPayload{Kind: CommandResolveAction, ActionID: "request-1-action-1", Decision: DecisionAllowOnce},
			wantCode: CodeActionNotPending,
		},
		{
			name:     "another action is pending",
			action:   approvalAction(true),
			payload:  CommandPayload{Kind: CommandResolveAction, ActionID: "request-9-action-9", Decision: DecisionAllowOnce},
			wantCode: CodeActionNotPending,
		},
		{
			name:     "approval answered with question answers",
			action:   approvalAction(true),
			payload:  CommandPayload{Kind: CommandResolveAction, ActionID: "request-1-action-1", Answers: []QuestionAnswer{{QuestionID: "request-1-action-1-q1", Values: []string{"A"}}}},
			wantCode: CodeInvalidAction,
		},
		{
			name:     "question answered with an approval decision",
			action:   questionAction(),
			payload:  CommandPayload{Kind: CommandResolveAction, ActionID: "request-1-action-1", Decision: DecisionAllowOnce},
			wantCode: CodeInvalidAction,
		},
		{
			name:    "partial answers",
			action:  questionAction(),
			payload: CommandPayload{Kind: CommandResolveAction, ActionID: "request-1-action-1", Answers: []QuestionAnswer{{QuestionID: "request-1-action-1-q1", Values: []string{"A"}}}},

			wantCode: CodeInvalidAnswer,
		},
		{
			name:   "option was not offered",
			action: questionAction(),
			payload: CommandPayload{Kind: CommandResolveAction, ActionID: "request-1-action-1", Answers: []QuestionAnswer{
				{QuestionID: "request-1-action-1-q1", Values: []string{"Z"}},
				{QuestionID: "request-1-action-1-q2", Values: []string{"X"}},
			}},
			wantCode: CodeInvalidAnswer,
		},
		{
			name:   "single select answered twice",
			action: questionAction(),
			payload: CommandPayload{Kind: CommandResolveAction, ActionID: "request-1-action-1", Answers: []QuestionAnswer{
				{QuestionID: "request-1-action-1-q1", Values: []string{"A", "B"}},
				{QuestionID: "request-1-action-1-q2", Values: []string{"X"}},
			}},
			wantCode: CodeInvalidAnswer,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newServerFixture(t, nil)
			fixture.controller.setPendingAction(test.action)
			connection := fixture.dial(t)
			_ = authenticateAndSubscribe(t, fixture, connection)

			writeTestEnvelope(t, connection, TypeCommand, "session-1", "command-1", "", 0, test.payload)
			envelope, payload := readCommandResult(t, connection, "command-1")
			if payload.Accepted || payload.Code != test.wantCode || envelope.RequestID != "" {
				t.Fatalf("result = %#v, payload = %#v, want code %q", envelope, payload, test.wantCode)
			}
			if got := fixture.controller.resolutions(); len(got) != 0 {
				t.Fatalf("rejected resolution reached the Controller: %#v", got)
			}
		})
	}
}

// TestWebSocketCancelCommandsAcknowledgeTheTargetRequest checks both cancel
// kinds: the Controller receives the named request and the accepted result
// correlates with it, which is the only acknowledgement cancel_queued gets.
func TestWebSocketCancelCommandsAcknowledgeTheTargetRequest(t *testing.T) {
	fixture := newServerFixture(t, nil)
	connection := fixture.dial(t)
	_ = authenticateAndSubscribe(t, fixture, connection)

	writeTestEnvelope(t, connection, TypeCommand, "session-1", "command-1", "", 0, CommandPayload{Kind: CommandCancelTurn, RequestID: "request-1"})
	envelope, payload := readCommandResult(t, connection, "command-1")
	if !payload.Accepted || envelope.RequestID != "request-1" {
		t.Fatalf("cancel_turn result = %#v, payload = %#v", envelope, payload)
	}
	writeTestEnvelope(t, connection, TypeCommand, "session-1", "command-2", "", 0, CommandPayload{Kind: CommandCancelQueued, RequestID: "request-2"})
	envelope, payload = readCommandResult(t, connection, "command-2")
	if !payload.Accepted || envelope.RequestID != "request-2" {
		t.Fatalf("cancel_queued result = %#v, payload = %#v", envelope, payload)
	}

	fixture.controller.mu.Lock()
	defer fixture.controller.mu.Unlock()
	if !reflect.DeepEqual(fixture.controller.cancelled, []string{"request-1"}) ||
		!reflect.DeepEqual(fixture.controller.dequeued, []string{"request-2"}) {
		t.Fatalf("cancel targets = %v / %v", fixture.controller.cancelled, fixture.controller.dequeued)
	}
}

// TestWebSocketControlCommandsMapControllerVerdicts pins the Controller's
// sentinels onto the stable §4.5 codes. The race is decided inside the actor, so
// Transport must report its verdict rather than guess a code of its own.
func TestWebSocketControlCommandsMapControllerVerdicts(t *testing.T) {
	tests := []struct {
		name     string
		prepare  func(*transportFakeController)
		payload  CommandPayload
		wantCode string
	}{
		{
			name:     "late resolution",
			prepare:  func(c *transportFakeController) { c.resolveErr = projectsession.ErrActionNotPending },
			payload:  CommandPayload{Kind: CommandResolveAction, ActionID: "request-1-action-1", Decision: DecisionDeny},
			wantCode: CodeActionNotPending,
		},
		{
			name:     "controller refuses the shape",
			prepare:  func(c *transportFakeController) { c.resolveErr = projectsession.ErrInvalidAction },
			payload:  CommandPayload{Kind: CommandResolveAction, ActionID: "request-1-action-1", Decision: DecisionDeny},
			wantCode: CodeInvalidAction,
		},
		{
			name:     "controller refuses the answers",
			prepare:  func(c *transportFakeController) { c.resolveErr = projectsession.ErrInvalidAnswer },
			payload:  CommandPayload{Kind: CommandResolveAction, ActionID: "request-1-action-1", Decision: DecisionDeny},
			wantCode: CodeInvalidAnswer,
		},
		{
			name:     "turn is not the running one",
			prepare:  func(c *transportFakeController) { c.cancelErr = projectsession.ErrTurnNotCancellable },
			payload:  CommandPayload{Kind: CommandCancelTurn, RequestID: "request-1"},
			wantCode: CodeTurnNotCancellable,
		},
		{
			name:     "request already left the queue",
			prepare:  func(c *transportFakeController) { c.dequeueErr = projectsession.ErrRequestNotQueued },
			payload:  CommandPayload{Kind: CommandCancelQueued, RequestID: "request-2"},
			wantCode: CodeRequestNotQueued,
		},
		{
			name:     "session is gone",
			prepare:  func(c *transportFakeController) { c.cancelErr = projectsession.ErrSessionUnavailable },
			payload:  CommandPayload{Kind: CommandCancelTurn, RequestID: "request-1"},
			wantCode: CodeSessionUnavailable,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newServerFixture(t, nil)
			fixture.controller.setPendingAction(approvalAction(true))
			test.prepare(fixture.controller)
			connection := fixture.dial(t)
			_ = authenticateAndSubscribe(t, fixture, connection)

			writeTestEnvelope(t, connection, TypeCommand, "session-1", "command-1", "", 0, test.payload)
			envelope, payload := readCommandResult(t, connection, "command-1")
			if payload.Accepted || payload.Code != test.wantCode || envelope.RequestID != "" {
				t.Fatalf("result = %#v, payload = %#v, want code %q", envelope, payload, test.wantCode)
			}
		})
	}
}

// TestWebSocketCommandResultPrecedesResolutionEvents extends the A4 ordering gate
// to the control commands: events produced by the resolution must not overtake
// the acknowledgement of the command that caused them.
func TestWebSocketCommandResultPrecedesResolutionEvents(t *testing.T) {
	fixture := newServerFixture(t, nil)
	fixture.controller.setPendingAction(approvalAction(false))
	connection := fixture.dial(t)
	_ = authenticateAndSubscribe(t, fixture, connection)

	fixture.controller.resolve = func(string) error {
		fixture.controller.emit(projectsession.Event{
			SessionID: "session-1", RequestID: "request-1", Seq: 8,
			Kind: projectsession.EventActionResolved, TurnStatus: projectsession.TurnRunning,
		})
		return nil
	}
	writeTestEnvelope(t, connection, TypeCommand, "session-1", "command-1", "", 0, CommandPayload{
		Kind: CommandResolveAction, ActionID: "request-1-action-1", Decision: DecisionAllowOnce,
	})
	if envelope, payload := readCommandResult(t, connection, "command-1"); !payload.Accepted || envelope.RequestID != "request-1" {
		t.Fatalf("first message = %#v, payload = %#v", envelope, payload)
	}
	event := readTestEnvelope(t, connection)
	var eventPayload EventPayload
	if event.Type != TypeEvent || DecodePayload(event, &eventPayload) != nil ||
		eventPayload.Event.Kind != projectsession.EventActionResolved || event.Seq != 8 {
		t.Fatalf("second message = %#v, payload = %#v", event, eventPayload)
	}
}

// TestWebSocketStructuredEventsKeepSeqMonotonic checks that the B2 event kinds
// travel on the same monotonic sequence as the A4 lifecycle events, so a client
// can still detect a gap without knowing the new kinds.
func TestWebSocketStructuredEventsKeepSeqMonotonic(t *testing.T) {
	fixture := newServerFixture(t, nil)
	connection := fixture.dial(t)
	_ = authenticateAndSubscribe(t, fixture, connection)

	falseValue := false
	sent := []projectsession.Event{
		{
			SessionID: "session-1", RequestID: "request-1", Seq: 8, Kind: projectsession.EventContentBlock,
			TurnStatus: projectsession.TurnRunning,
			Block:      &projectsession.ContentBlock{ID: "block-1", Kind: projectsession.BlockThinking, Thinking: "plan first"},
		},
		{
			SessionID: "session-1", RequestID: "request-1", Seq: 9, Kind: projectsession.EventContentBlock,
			TurnStatus: projectsession.TurnRunning,
			Block: &projectsession.ContentBlock{
				ID: "block-2", Kind: projectsession.BlockToolUse, ToolUseID: "tool-1",
				ToolClass: projectsession.ToolBuiltin, ToolName: "Bash",
			},
		},
		{
			SessionID: "session-1", RequestID: "request-1", Seq: 10, Kind: projectsession.EventActionRequired,
			TurnStatus: projectsession.TurnWaiting, PendingAction: approvalAction(true),
		},
		{
			SessionID: "session-1", RequestID: "request-1", Seq: 11, Kind: projectsession.EventContentBlock,
			TurnStatus: projectsession.TurnRunning,
			Block: &projectsession.ContentBlock{
				ID: "block-3", Kind: projectsession.BlockToolResult, ToolUseID: "tool-1", IsError: &falseValue,
			},
		},
	}
	for _, event := range sent {
		fixture.controller.emit(event)
	}

	lastSeq := uint64(7)
	for index, want := range sent {
		envelope := readTestEnvelope(t, connection)
		var payload EventPayload
		if envelope.Type != TypeEvent || DecodePayload(envelope, &payload) != nil {
			t.Fatalf("message %d = %#v, want event", index, envelope)
		}
		if envelope.Seq <= lastSeq || envelope.Seq != payload.Event.Seq || payload.Event.Kind != want.Kind {
			t.Fatalf("event %d = seq %d kind %q, want seq > %d kind %q", index, envelope.Seq, payload.Event.Kind, lastSeq, want.Kind)
		}
		lastSeq = envelope.Seq
	}
}

// TestWebSocketUnsupportedNoticeTravelsAsItsOwnType covers the one domain event
// the Controller publishes but §4.2 forbids on the ordinary event path. The
// notice has to reach the client as type=unsupported carrying nothing but the
// fixed code, and the connection must survive it: an unmappable SDK block is a
// safety notice, not a protocol violation.
func TestWebSocketUnsupportedNoticeTravelsAsItsOwnType(t *testing.T) {
	fixture := newServerFixture(t, nil)
	connection := fixture.dial(t)
	_ = authenticateAndSubscribe(t, fixture, connection)

	fixture.controller.emit(projectsession.Event{
		SessionID: "session-1", RequestID: "request-1", Seq: 8,
		Kind: projectsession.EventUnsupported, Source: projectsession.InputMobile,
		Code: projectsession.UnsupportedUnknownBlock,
	})

	envelope := readTestEnvelope(t, connection)
	var payload UnsupportedPayload
	if envelope.Type != TypeUnsupported || DecodePayload(envelope, &payload) != nil {
		t.Fatalf("message = %#v, want an unsupported notice", envelope)
	}
	if payload.Code != projectsession.UnsupportedUnknownBlock {
		t.Fatalf("code = %q, want %q", payload.Code, projectsession.UnsupportedUnknownBlock)
	}
	if envelope.RequestID != "request-1" || envelope.Seq != 8 {
		t.Fatalf("correlation = requestId %q seq %d, want request-1 and 8", envelope.RequestID, envelope.Seq)
	}
	// The notice must not smuggle the session data an ordinary event carries.
	if err := ValidateEnvelope(envelope); err != nil {
		t.Fatalf("ValidateEnvelope() error = %v", err)
	}

	// The stream continues: the client keeps receiving later events in order.
	fixture.controller.emit(projectsession.Event{
		SessionID: "session-1", RequestID: "request-1", Seq: 9,
		Kind: projectsession.EventTextDelta, Text: "still here",
	})
	next := readTestEnvelope(t, connection)
	var eventPayload EventPayload
	if next.Type != TypeEvent || DecodePayload(next, &eventPayload) != nil ||
		eventPayload.Event.Kind != projectsession.EventTextDelta {
		t.Fatalf("message after the notice = %#v, want a text_delta event", next)
	}
}

// TestWebSocketRejectsCommandForAnotherSession keeps a subscribed connection
// bound to the session it authenticated for: a command naming another session is
// a protocol violation and must never reach a Controller.
func TestWebSocketRejectsCommandForAnotherSession(t *testing.T) {
	fixture := newServerFixture(t, nil)
	connection := fixture.dial(t)
	_ = authenticateAndSubscribe(t, fixture, connection)

	writeTestEnvelope(t, connection, TypeCommand, "session-2", "command-1", "", 0, CommandPayload{Kind: CommandSendText, Text: "hello"})
	response := readTestEnvelope(t, connection)
	var payload ErrorPayload
	if response.Type != TypeError || DecodePayload(response, &payload) != nil || payload.Code != CodeInvalidState {
		t.Fatalf("response = %#v, payload = %#v, want an invalid_state error", response, payload)
	}
	if _, _, err := connection.ReadMessage(); err == nil {
		t.Fatal("connection stayed open after a cross-session command")
	}
	fixture.controller.mu.Lock()
	defer fixture.controller.mu.Unlock()
	if len(fixture.controller.inputs) != 0 {
		t.Fatalf("cross-session command reached the Controller: %#v", fixture.controller.inputs)
	}
}

// TestWebSocketRejectsStaleProtocolVersionWithoutSessionData refuses an A4/B1
// client outright. The answer names the expected version so the client can
// report an upgrade, and carries no Session state of any kind.
func TestWebSocketRejectsStaleProtocolVersionWithoutSessionData(t *testing.T) {
	fixture := newServerFixture(t, nil)
	connection := fixture.dial(t)

	stale := `{"version":1,"type":"hello","sessionId":"session-1","commandId":"","requestId":"","seq":0,"payload":{"pairToken":"` + fixture.token + `"}}`
	if err := connection.WriteMessage(websocket.TextMessage, []byte(stale)); err != nil {
		t.Fatalf("WriteMessage() error = %v", err)
	}
	_ = connection.SetReadDeadline(time.Now().Add(time.Second))
	_, data, err := connection.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	envelope, err := DecodeEnvelope(data)
	if err != nil {
		t.Fatalf("DecodeEnvelope() error = %v", err)
	}
	var payload ErrorPayload
	if envelope.Type != TypeError || DecodePayload(envelope, &payload) != nil {
		t.Fatalf("response = %#v, want an error", envelope)
	}
	if payload.Code != CodeUnsupportedVersion || payload.ExpectedVersion != Version {
		t.Fatalf("error payload = %#v, want %q with expectedVersion %d", payload, CodeUnsupportedVersion, Version)
	}
	if envelope.SessionID != "" || envelope.RequestID != "" || envelope.Seq != 0 {
		t.Fatalf("refusal carried session data: %#v", envelope)
	}
	if strings.Contains(string(data), fixture.token) {
		t.Fatal("refusal echoed the pair token")
	}
}

// TestWebSocketRejectsMalformedControlCommands keeps each command kind to its own
// field subset over the wire: an extra, missing or misplaced field closes the
// connection instead of being read as a different command.
func TestWebSocketRejectsMalformedControlCommands(t *testing.T) {
	tests := []struct {
		name      string
		requestID string
		payload   CommandPayload
	}{
		{name: "resolve_action without an action", payload: CommandPayload{Kind: CommandResolveAction, Decision: DecisionAllowOnce}},
		{name: "resolve_action with both forms", payload: CommandPayload{
			Kind: CommandResolveAction, ActionID: "request-1-action-1", Decision: DecisionAllowOnce,
			Answers: []QuestionAnswer{{QuestionID: "request-1-action-1-q1", Values: []string{"A"}}},
		}},
		{name: "resolve_action with neither form", payload: CommandPayload{Kind: CommandResolveAction, ActionID: "request-1-action-1"}},
		{name: "unknown decision", payload: CommandPayload{Kind: CommandResolveAction, ActionID: "request-1-action-1", Decision: "allow_forever"}},
		{name: "cancel_turn carrying an action", payload: CommandPayload{Kind: CommandCancelTurn, RequestID: "request-1", ActionID: "request-1-action-1"}},
		{name: "cancel_queued without a request", payload: CommandPayload{Kind: CommandCancelQueued}},
		{name: "send_text carrying a decision", payload: CommandPayload{Kind: CommandSendText, Text: "hello", Decision: DecisionDeny}},
		{name: "unknown kind", payload: CommandPayload{Kind: "approve_everything", Text: "hello"}},
		{name: "blank answer value", payload: CommandPayload{
			Kind: CommandResolveAction, ActionID: "request-1-action-1",
			Answers: []QuestionAnswer{{QuestionID: "request-1-action-1-q1", Values: []string{"  "}}},
		}},
		{
			name:      "command correlated with a request",
			requestID: "request-1",
			payload:   CommandPayload{Kind: CommandCancelTurn, RequestID: "request-1"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newServerFixture(t, nil)
			fixture.controller.setPendingAction(questionAction())
			connection := fixture.dial(t)
			_ = authenticateAndSubscribe(t, fixture, connection)

			writeTestEnvelope(t, connection, TypeCommand, "session-1", "command-1", test.requestID, 0, test.payload)
			response := readTestEnvelope(t, connection)
			var payload ErrorPayload
			if response.Type != TypeError || DecodePayload(response, &payload) != nil || payload.Code != CodeInvalidMessage {
				t.Fatalf("response = %#v, payload = %#v, want an invalid_message error", response, payload)
			}
			if got := fixture.controller.resolutions(); len(got) != 0 {
				t.Fatalf("malformed command reached the Controller: %#v", got)
			}
		})
	}
}
