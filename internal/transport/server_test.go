package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tea4go/claude-mobile-remote/internal/channel"
	"github.com/tea4go/claude-mobile-remote/internal/pairing"
	projectsession "github.com/tea4go/claude-mobile-remote/internal/session"
)

type transportFakeController struct {
	mu          sync.Mutex
	snapshot    projectsession.Snapshot
	inputs      []projectsession.Input
	submit      func(projectsession.Input) (string, error)
	subscribers map[chan projectsession.Event]<-chan struct{}
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

	// 三端共用契约：首轮前 snapshot 不得新增字段或浏览器特有分支。
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
		"seq": true, "turn": true, "queueDepth": true,
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
