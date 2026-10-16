package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	projectclaude "github.com/tea4go/claude-mobile-remote/internal/claude"
	"github.com/tea4go/claude-mobile-remote/internal/pairing"
	projectsession "github.com/tea4go/claude-mobile-remote/internal/session"
	"github.com/tea4go/claude-mobile-remote/internal/transport"
)

func newManagedFixture(t *testing.T) (*managedCLI, string) {
	t.Helper()
	config := cliConfig{
		listen: "127.0.0.1:0", advertiseURL: "ws://127.0.0.1:8765/ws",
		pairTokenTTL: time.Hour, cwd: t.TempDir(), maxSessions: 4,
		fakeHostScenario: projectclaude.FakeScenarioText,
	}
	m, err := newManagedCLI(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := m.Close(ctx); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	initial, err := m.startRuntime(true)
	if err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	m.sessions[initial.id] = initial
	m.active = 1
	m.started = true
	m.mu.Unlock()
	return m, initial.id
}

func TestManagedCLICreateIsolationIdempotencyAndCapacity(t *testing.T) {
	m, a := newManagedFixture(t)
	ctx := context.Background()
	b, err := m.CreateSession(ctx, a, "create-b")
	if err != nil {
		t.Fatal(err)
	}
	if b.SessionID == a || b.SessionID == "" || b.WSURL != m.config.advertiseURL {
		t.Fatalf("unexpected B identity: %q", b.SessionID)
	}
	if len(b.Capabilities) != 2 {
		t.Fatalf("B capabilities = %v", b.Capabilities)
	}
	if !validPCID(b.PCID) || b.PCID != m.sessions[a].payload.PCID {
		t.Fatal("sessions on the same CLI have different PC IDs")
	}
	again, err := m.CreateSession(ctx, a, "create-b")
	if err != nil || again.SessionID != b.SessionID || again.PairToken != b.PairToken {
		t.Fatalf("retry created a different Session: %q, %v", again.SessionID, err)
	}
	if len(m.SessionIDs()) != 2 {
		t.Fatalf("Session count = %d, want 2", len(m.SessionIDs()))
	}
	if _, err := m.Authenticate(a, b.PairToken); !errors.Is(err, pairing.ErrUnauthorized) {
		t.Fatalf("B token authenticated as A: %v", err)
	}
	lease, err := m.Authenticate(b.SessionID, b.PairToken)
	if err != nil {
		t.Fatalf("B token rejected for B: %v", err)
	}
	lease.Close()
	controllerA, _, _ := m.Session(a)
	controllerB, _, _ := m.Session(b.SessionID)
	if _, err := controllerA.Submit(ctx, projectsession.Input{Source: projectsession.InputTerminal, Text: "first"}); err != nil {
		t.Fatal(err)
	}
	if _, err := controllerB.Submit(ctx, projectsession.Input{SessionID: b.SessionID, Source: projectsession.InputMobile, Text: "second"}); err != nil {
		t.Fatal(err)
	}
	snapshotA, err := controllerA.WaitIdle(ctx)
	if err != nil {
		t.Fatal(err)
	}
	snapshotB, err := controllerB.WaitIdle(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if snapshotA.SessionID != a || snapshotB.SessionID != b.SessionID ||
		snapshotA.Turn.Source != projectsession.InputTerminal || snapshotB.Turn.Source != projectsession.InputMobile ||
		snapshotA.Turn.RequestID == snapshotB.Turn.RequestID {
		t.Fatalf("Session inputs crossed: A=%+v B=%+v", snapshotA.Turn, snapshotB.Turn)
	}
	if snapshotA.ProjectCwd != m.cwd || snapshotB.ProjectCwd != m.cwd ||
		snapshotA.SessionTitle != "first" || snapshotB.SessionTitle != "second" ||
		snapshotA.CreatedAt == "" || snapshotB.CreatedAt == "" ||
		snapshotA.LastActivityAt == "" || snapshotB.LastActivityAt == "" {
		t.Fatalf("display metadata crossed or missing: A=%+v B=%+v", snapshotA, snapshotB)
	}

	c, err := m.CreateSession(ctx, b.SessionID, "create-c")
	if err != nil {
		t.Fatal(err)
	}
	d, err := m.CreateSession(ctx, c.SessionID, "create-d")
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.CreateSession(ctx, d.SessionID, "create-e")
	var protocolErr *transport.ProtocolError
	if !errors.As(err, &protocolErr) || protocolErr.Code != "session_capacity" {
		t.Fatalf("capacity error = %v", err)
	}
	if err := m.StopSession(ctx, b.SessionID); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	active := m.active
	m.mu.Unlock()
	if active != 3 {
		t.Fatalf("active count after stopping B = %d, want 3", active)
	}
	retained, _, ok := m.Session(b.SessionID)
	if !ok {
		t.Fatal("stopped Session was removed")
	}
	snapshot, err := retained.Snapshot(ctx)
	if err != nil || snapshot.SessionStatus != projectsession.SessionStopped {
		t.Fatalf("stopped snapshot = %v, %v", snapshot.SessionStatus, err)
	}
	channel, ok := m.registry.Get(b.SessionID)
	if !ok {
		t.Fatal("stopped Session channel missing")
	}
	subCtx, cancel := context.WithCancel(ctx)
	subscription, err := channel.OpenSubscription(subCtx, 2)
	if err != nil || subscription.Snapshot.SessionStatus != projectsession.SessionStopped {
		t.Fatalf("retained subscription = %v, %v", subscription, err)
	}
	cancel()
	_, err = m.CreateSession(ctx, d.SessionID, "create-e")
	if err != nil {
		t.Fatalf("stopped B did not free capacity: %v", err)
	}
	if _, err := m.Authenticate(b.SessionID, b.PairToken); err != nil {
		t.Fatalf("stopped B credential no longer usable: %v", err)
	}
}

func TestManagedCLIRejectsUnknownSourceAndRateLimits(t *testing.T) {
	m, a := newManagedFixture(t)
	_, err := m.CreateSession(context.Background(), "other", "command")
	var protocolErr *transport.ProtocolError
	if !errors.As(err, &protocolErr) || protocolErr.Code != transport.CodeSessionUnavailable {
		t.Fatalf("unknown source error = %v", err)
	}
	if _, err := m.CreateSession(context.Background(), a, "first"); err != nil {
		t.Fatal(err)
	}
	_, err = m.CreateSession(context.Background(), a, "second")
	if !errors.As(err, &protocolErr) || protocolErr.Code != "create_rate_limited" {
		t.Fatalf("rate limit error = %v", err)
	}
}

func TestManagedCLIStopDoesNotLeaveStoppingAfterCallerCancellation(t *testing.T) {
	m, sessionID := newManagedFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = m.StopSession(ctx, sessionID)
	m.mu.Lock()
	state := m.sessions[sessionID].state
	m.mu.Unlock()
	if state == "stopping" {
		t.Fatal("canceled stop left Session permanently stopping")
	}
}

func TestManagedTerminalCommandsSelectCreateAndStop(t *testing.T) {
	m, a := newManagedFixture(t)
	selected := a
	var output bytes.Buffer
	attach := func(string) {}
	if err := runManagedCommand(context.Background(), m, ":new", &selected, attach, &output); err != nil {
		t.Fatal(err)
	}
	b := selected
	if b == a {
		t.Fatal(":new did not select new Session")
	}
	if err := runManagedCommand(context.Background(), m, ":use "+a[:8], &selected, attach, &output); err != nil || selected != a {
		t.Fatalf(":use did not select A: %v", err)
	}
	if err := runManagedCommand(context.Background(), m, ":stop "+b, &selected, attach, &output); err != nil {
		t.Fatal(err)
	}
	if err := runManagedCommand(context.Background(), m, ":sessions", &selected, attach, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), b+" stopped") || !strings.Contains(output.String(), a) {
		t.Fatal("terminal list did not show both isolated Sessions")
	}
}

func TestManagedCLIWebSocketCreateStopAndRetain(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := probe.Addr().String()
	_ = probe.Close()
	config := cliConfig{
		listen: address, advertiseURL: "ws://" + address + "/ws",
		pairTokenTTL: time.Hour, cwd: t.TempDir(), maxSessions: 4,
		fakeHostScenario: projectclaude.FakeScenarioText,
	}
	m, err := newManagedCLI(config)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	t.Cleanup(func() {
		closeCtx, done := context.WithTimeout(context.Background(), 3*time.Second)
		defer done()
		if err := m.Close(closeCtx); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	a, err := m.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	connect := func(payload pairing.Payload) *websocket.Conn {
		t.Helper()
		conn, _, err := websocket.DefaultDialer.Dial(payload.WSURL, nil)
		if err != nil {
			t.Fatal(err)
		}
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		sendEnvelope(t, conn, transport.TypeHello, payload.SessionID, "", transport.HelloPayload{PairToken: payload.PairToken})
		readType(t, conn, transport.TypeHelloAck)
		sendEnvelope(t, conn, transport.TypeSubscribe, payload.SessionID, "", struct{}{})
		readType(t, conn, transport.TypeSubscribed)
		readType(t, conn, transport.TypeSnapshot)
		return conn
	}
	first := connect(a)
	defer first.Close()
	sendEnvelope(t, first, transport.TypeCreateSession, a.SessionID, "create-b", struct{}{})
	result := readType(t, first, transport.TypeCreateSessionResult)
	var created transport.ManagementResultPayload
	if err := json.Unmarshal(result.Payload, &created); err != nil || !created.Accepted || created.Pairing == nil {
		t.Fatalf("create result invalid: %v, accepted=%v", err, created.Accepted)
	}
	b := *created.Pairing
	if b.SessionID == a.SessionID {
		t.Fatal("new Session reused A identity")
	}
	second := connect(b)
	defer second.Close()
	sendEnvelope(t, second, transport.TypeStopSession, b.SessionID, "stop-b", struct{}{})
	for {
		message := readAny(t, second)
		if message.Type == transport.TypeStopSessionResult {
			var stopped transport.ManagementResultPayload
			if err := json.Unmarshal(message.Payload, &stopped); err != nil || !stopped.Accepted {
				t.Fatalf("stop result invalid: %v, accepted=%v", err, stopped.Accepted)
			}
			break
		}
	}
	third := connect(b)
	defer third.Close()
	sendEnvelope(t, third, transport.TypeCreateSession, b.SessionID, "create-c", struct{}{})
	result = readType(t, third, transport.TypeCreateSessionResult)
	var next transport.ManagementResultPayload
	if err := json.Unmarshal(result.Payload, &next); err != nil || !next.Accepted || next.Pairing == nil {
		t.Fatalf("create from stopped Session failed: %v", err)
	}
	if _, err := m.Authenticate(a.SessionID, b.PairToken); !errors.Is(err, pairing.ErrUnauthorized) {
		t.Fatalf("B credential authorized A: %v", err)
	}
}

func TestManagedCLIEOFWaitsForTerminalTurn(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := probe.Addr().String()
	_ = probe.Close()
	var output bytes.Buffer
	var errorOutput bytes.Buffer
	code := runManagedMain(context.Background(), []string{
		"--fake-host", string(projectclaude.FakeScenarioText),
		"--cwd", t.TempDir(), "--listen", address,
		"--advertise-url", "ws://" + address + "/ws",
	}, strings.NewReader("hello\n"), &output, &errorOutput)
	if code != 0 || !strings.Contains(output.String(), "这是假 Host 的固定回复") {
		t.Fatalf("EOF exited before Turn completion: code=%d stderr=%q", code, errorOutput.String())
	}
}

func sendEnvelope(t *testing.T, conn *websocket.Conn, kind transport.MessageType, sessionID, commandID string, payload any) {
	t.Helper()
	envelope, err := transport.NewEnvelope(kind, sessionID, commandID, "", 0, payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.WriteJSON(envelope); err != nil {
		t.Fatal(err)
	}
}

func readAny(t *testing.T, conn *websocket.Conn) transport.Envelope {
	t.Helper()
	var message transport.Envelope
	if err := conn.ReadJSON(&message); err != nil {
		t.Fatal(err)
	}
	return message
}

func readType(t *testing.T, conn *websocket.Conn, kind transport.MessageType) transport.Envelope {
	t.Helper()
	message := readAny(t, conn)
	if message.Type != kind {
		t.Fatalf("message type = %s, want %s", message.Type, kind)
	}
	return message
}
