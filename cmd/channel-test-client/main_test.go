package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tea4go/claude-mobile-remote/internal/pairing"
	projectsession "github.com/tea4go/claude-mobile-remote/internal/session"
	"github.com/tea4go/claude-mobile-remote/internal/transport"
)

func TestRunClientClosedLoop(t *testing.T) {
	received := make(chan transport.MessageType, 3)
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		connection, err := upgrader.Upgrade(response, request, nil)
		if err != nil {
			return
		}
		defer connection.Close()
		read := func() transport.Envelope {
			_, data, readErr := connection.ReadMessage()
			if readErr != nil {
				t.Errorf("server ReadMessage() error = %v", readErr)
				return transport.Envelope{}
			}
			envelope, decodeErr := transport.DecodeEnvelope(data)
			if decodeErr != nil {
				t.Errorf("server DecodeEnvelope() error = %v", decodeErr)
			}
			received <- envelope.Type
			return envelope
		}
		write := func(messageType transport.MessageType, commandID, requestID string, seq uint64, payload any) {
			envelope, envelopeErr := transport.NewEnvelope(messageType, "session-1", commandID, requestID, seq, payload)
			if envelopeErr != nil {
				t.Errorf("server NewEnvelope() error = %v", envelopeErr)
				return
			}
			data, encodeErr := transport.EncodeEnvelope(envelope)
			if encodeErr != nil {
				t.Errorf("server EncodeEnvelope() error = %v", encodeErr)
				return
			}
			if writeErr := connection.WriteMessage(websocket.TextMessage, data); writeErr != nil {
				t.Errorf("server WriteMessage() error = %v", writeErr)
			}
		}

		read()
		write(transport.TypeHelloAck, "", "", 0, struct{}{})
		read()
		write(transport.TypeSubscribed, "", "", 0, struct{}{})
		write(transport.TypeSnapshot, "", "", 7, transport.SnapshotPayload{
			ConnectionStatus: "subscribed",
			Snapshot:         projectsession.Snapshot{SessionID: "session-1", SessionStatus: projectsession.SessionReady, Seq: 7},
		})
		command := read()
		write(transport.TypeCommandResult, command.CommandID, "request-1", 0, transport.CommandResultPayload{Accepted: true})
		write(transport.TypeEvent, "", "request-1", 8, transport.EventPayload{Event: projectsession.Event{
			SessionID: "session-1", RequestID: "request-1", Seq: 8, Kind: projectsession.EventTurnStarted,
		}})
		_ = connection.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "complete"), time.Now().Add(time.Second))
	}))
	defer server.Close()

	payload := pairing.Payload{
		Version:   pairing.PayloadVersion,
		WSURL:     "ws" + strings.TrimPrefix(server.URL, "http"),
		SessionID: "session-1", PairToken: "fixture-credential", ExpiresAt: time.Now().Add(time.Hour),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	var errorOutput bytes.Buffer
	exitCode := runClient(context.Background(), []string{"--pair", string(raw)}, strings.NewReader("hello\n"), &output, &errorOutput)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", exitCode, errorOutput.String())
	}
	for _, want := range []transport.MessageType{transport.TypeHello, transport.TypeSubscribe, transport.TypeCommand} {
		select {
		case got := <-received:
			if got != want {
				t.Fatalf("received type = %q, want %q", got, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for %q", want)
		}
	}
	if !strings.Contains(output.String(), "authenticated") || !strings.Contains(output.String(), "subscribed") ||
		!strings.Contains(output.String(), "request-1") || !strings.Contains(output.String(), "event seq=8") {
		t.Fatalf("output = %q", output.String())
	}
	if strings.Contains(output.String(), payload.PairToken) {
		t.Fatal("client echoed pair token")
	}
}

func TestParsePairingRejectsExpiredAndUnknownFields(t *testing.T) {
	expired := `{"version":1,"wsUrl":"ws://pc/ws","sessionId":"session-1","pairToken":"fixture","expiresAt":"2020-01-01T00:00:00Z"}`
	if _, err := parsePairing([]string{"--pair", expired}); err == nil {
		t.Fatal("parsePairing() accepted expired payload")
	}
	unknown := `{"version":1,"wsUrl":"ws://pc/ws","sessionId":"session-1","pairToken":"fixture","expiresAt":"2099-01-01T00:00:00Z","extra":true}`
	if _, err := parsePairing([]string{"--pair", unknown}); err == nil {
		t.Fatal("parsePairing() accepted unknown field")
	}
}

func TestRunClientReconnectDoesNotResendPreviousCommand(t *testing.T) {
	var mu sync.Mutex
	connections := 0
	commands := 0
	serverDone := make(chan struct{}, 2)
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		connection, err := upgrader.Upgrade(response, request, nil)
		if err != nil {
			return
		}
		defer connection.Close()
		defer func() { serverDone <- struct{}{} }()
		mu.Lock()
		connections++
		connectionNumber := connections
		mu.Unlock()
		read := func() transport.Envelope {
			_, data, readErr := connection.ReadMessage()
			if readErr != nil {
				return transport.Envelope{}
			}
			envelope, _ := transport.DecodeEnvelope(data)
			return envelope
		}
		write := func(messageType transport.MessageType, commandID, requestID string, seq uint64, payload any) {
			envelope, _ := transport.NewEnvelope(messageType, "session-1", commandID, requestID, seq, payload)
			data, _ := transport.EncodeEnvelope(envelope)
			_ = connection.WriteMessage(websocket.TextMessage, data)
		}
		read()
		write(transport.TypeHelloAck, "", "", 0, struct{}{})
		read()
		write(transport.TypeSubscribed, "", "", 0, struct{}{})
		seq := uint64(7)
		if connectionNumber == 2 {
			seq = 9
		}
		write(transport.TypeSnapshot, "", "", seq, transport.SnapshotPayload{ConnectionStatus: "subscribed", Snapshot: projectsession.Snapshot{SessionID: "session-1", Seq: seq}})
		command := read()
		if command.Type == transport.TypeCommand {
			mu.Lock()
			commands++
			mu.Unlock()
			write(transport.TypeCommandResult, command.CommandID, "request-1", 0, transport.CommandResultPayload{Accepted: true})
			_ = connection.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "complete"), time.Now().Add(time.Second))
		}
	}))
	defer server.Close()
	payload := pairing.Payload{Version: 1, WSURL: "ws" + strings.TrimPrefix(server.URL, "http"), SessionID: "session-1", PairToken: "fixture", ExpiresAt: time.Now().Add(time.Hour)}
	raw, _ := json.Marshal(payload)
	if code := runClient(context.Background(), []string{"--pair", string(raw)}, strings.NewReader("first\n"), io.Discard, io.Discard); code != 0 {
		t.Fatalf("first run exit code = %d", code)
	}
	var secondOutput bytes.Buffer
	if code := runClient(context.Background(), []string{"--pair", string(raw)}, strings.NewReader(":quit\n"), &secondOutput, io.Discard); code != 0 {
		t.Fatalf("second run exit code = %d", code)
	}
	for range 2 {
		select {
		case <-serverDone:
		case <-time.After(time.Second):
			t.Fatal("server connection did not finish")
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if commands != 1 || !strings.Contains(secondOutput.String(), "snapshot seq=9") {
		t.Fatalf("commands = %d, second output = %q", commands, secondOutput.String())
	}
}
