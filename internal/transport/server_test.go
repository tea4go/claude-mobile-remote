package transport

import (
	"bytes"
	"context"
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
			Kind: projectsession.EventTurnStarted, TurnStatus: projectsession.TurnRunning,
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

	fixture.controller.emit(projectsession.Event{SessionID: "session-1", RequestID: "request-1", Seq: 8, Kind: projectsession.EventTurnStarted})
	for name, connection := range map[string]*websocket.Conn{"first": first, "second": second} {
		event := readTestEnvelope(t, connection)
		if event.Type != TypeEvent || event.Seq != 8 {
			t.Fatalf("%s event = %#v", name, event)
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
