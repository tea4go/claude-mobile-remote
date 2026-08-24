package transport

import (
	"encoding/json"
	"errors"
	"testing"

	projectsession "github.com/tea4go/claude-mobile-remote/internal/session"
)

func TestDecodeEnvelope(t *testing.T) {
	input := []byte(`{"version":1,"type":"command","sessionId":"session-1","commandId":"command-1","requestId":"","seq":0,"payload":{"kind":"send_text","text":"hello"}}`)
	envelope, err := DecodeEnvelope(input)
	if err != nil {
		t.Fatalf("DecodeEnvelope() error = %v", err)
	}
	if envelope.Version != Version || envelope.Type != TypeCommand || envelope.SessionID != "session-1" || envelope.CommandID != "command-1" {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
	var command CommandPayload
	if err := DecodePayload(envelope, &command); err != nil {
		t.Fatalf("DecodePayload() error = %v", err)
	}
	if command.Kind != CommandSendText || command.Text != "hello" {
		t.Fatalf("unexpected command: %#v", command)
	}
}

func TestValidateEnvelope(t *testing.T) {
	tests := []struct {
		name     string
		envelope Envelope
		wantCode string
	}{
		{
			name:     "unsupported version",
			envelope: Envelope{Version: 2, Type: TypeHello, SessionID: "s", Payload: json.RawMessage(`{"pairToken":"fixture"}`)},
			wantCode: CodeUnsupportedVersion,
		},
		{
			name:     "missing session",
			envelope: Envelope{Version: 1, Type: TypeHello, Payload: json.RawMessage(`{"pairToken":"fixture"}`)},
			wantCode: CodeInvalidMessage,
		},
		{
			name:     "command missing id",
			envelope: Envelope{Version: 1, Type: TypeCommand, SessionID: "s", Payload: json.RawMessage(`{"kind":"send_text","text":"x"}`)},
			wantCode: CodeInvalidMessage,
		},
		{
			name:     "command carries request id",
			envelope: Envelope{Version: 1, Type: TypeCommand, SessionID: "s", CommandID: "c", RequestID: "r", Payload: json.RawMessage(`{"kind":"send_text","text":"x"}`)},
			wantCode: CodeInvalidMessage,
		},
		{
			name:     "unknown message type",
			envelope: Envelope{Version: 1, Type: "other", SessionID: "s", Payload: json.RawMessage(`{}`)},
			wantCode: CodeInvalidMessage,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateEnvelope(test.envelope)
			var protocolErr *ProtocolError
			if !errors.As(err, &protocolErr) || protocolErr.Code != test.wantCode {
				t.Fatalf("ValidateEnvelope() error = %v, want code %q", err, test.wantCode)
			}
		})
	}
}

func TestValidateEnvelopeAcceptsCommand(t *testing.T) {
	envelope := Envelope{
		Version:   Version,
		Type:      TypeCommand,
		SessionID: "session-1",
		CommandID: "command-1",
		Payload:   json.RawMessage(`{"kind":"send_text","text":"hello"}`),
	}
	if err := ValidateEnvelope(envelope); err != nil {
		t.Fatalf("ValidateEnvelope() error = %v", err)
	}
}

func TestNewEnvelopeRoundTrip(t *testing.T) {
	envelope, err := NewEnvelope(TypeCommand, "session-1", "command-1", "", 0, CommandPayload{
		Kind: CommandSendText,
		Text: "hello",
	})
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}
	data, err := EncodeEnvelope(envelope)
	if err != nil {
		t.Fatalf("EncodeEnvelope() error = %v", err)
	}
	decoded, err := DecodeEnvelope(data)
	if err != nil {
		t.Fatalf("DecodeEnvelope() error = %v", err)
	}
	if err := ValidateEnvelope(decoded); err != nil {
		t.Fatalf("ValidateEnvelope() error = %v", err)
	}
}

func TestValidateEnvelopeAcceptsServiceMessages(t *testing.T) {
	snapshot := projectsession.Snapshot{SessionID: "session-1", SessionStatus: projectsession.SessionReady, Seq: 7}
	event := projectsession.Event{SessionID: "session-1", Kind: projectsession.EventTurnStarted, RequestID: "request-1", Seq: 8}
	tests := []struct {
		name      string
		typeName  MessageType
		commandID string
		requestID string
		seq       uint64
		payload   any
	}{
		{name: "hello ack", typeName: TypeHelloAck, payload: struct{}{}},
		{name: "snapshot", typeName: TypeSnapshot, seq: 7, payload: SnapshotPayload{ConnectionStatus: "subscribed", Snapshot: snapshot}},
		{name: "event", typeName: TypeEvent, requestID: "request-1", seq: 8, payload: EventPayload{Event: event}},
		{name: "accepted command", typeName: TypeCommandResult, commandID: "command-1", requestID: "request-1", payload: CommandResultPayload{Accepted: true}},
		{name: "rejected command", typeName: TypeCommandResult, commandID: "command-1", payload: CommandResultPayload{Code: CodeQueueFull, Message: "input queue full"}},
		{name: "error", typeName: TypeError, payload: ErrorPayload{Code: CodeInvalidState, Message: "invalid connection state"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			envelope, err := NewEnvelope(test.typeName, "session-1", test.commandID, test.requestID, test.seq, test.payload)
			if err != nil {
				t.Fatalf("NewEnvelope() error = %v", err)
			}
			if err := ValidateEnvelope(envelope); err != nil {
				t.Fatalf("ValidateEnvelope() error = %v", err)
			}
		})
	}
}

func TestValidateEnvelopeRejectsMismatchedServiceSequence(t *testing.T) {
	envelope, err := NewEnvelope(TypeSnapshot, "session-1", "", "", 8, SnapshotPayload{
		ConnectionStatus: "subscribed",
		Snapshot:         projectsession.Snapshot{SessionID: "session-1", Seq: 7},
	})
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v", err)
	}
	if err := ValidateEnvelope(envelope); err == nil {
		t.Fatal("ValidateEnvelope() accepted mismatched snapshot seq")
	}
}

func TestDecodeEnvelopeRejectsUnknownAndTrailingData(t *testing.T) {
	inputs := [][]byte{
		[]byte(`{"version":1,"type":"hello","sessionId":"s","commandId":"","requestId":"","seq":0,"payload":{},"extra":true}`),
		[]byte(`{"version":1,"type":"hello","sessionId":"s","commandId":"","requestId":"","seq":0,"payload":{}} {}`),
	}
	for _, input := range inputs {
		if _, err := DecodeEnvelope(input); !errors.Is(err, ErrInvalidMessage) {
			t.Fatalf("DecodeEnvelope(%q) error = %v, want ErrInvalidMessage", input, err)
		}
	}
}

func FuzzDecodeEnvelope(f *testing.F) {
	f.Add([]byte(`{"version":1,"type":"hello","sessionId":"s","commandId":"","requestId":"","seq":0,"payload":{"pairToken":"fixture"}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodeEnvelope(data)
	})
}
