package transport

import (
	"encoding/json"
	"testing"
)

func TestHeartbeatEnvelopeValidation(t *testing.T) {
	valid := Envelope{Version: 6, Type: TypePing, SessionID: "session-1", Payload: json.RawMessage(`{}`)}
	for _, kind := range []MessageType{TypePing, TypePong} {
		e := valid
		e.Type = kind
		if err := ValidateEnvelope(e); err != nil {
			t.Fatal(err)
		}
	}
	for _, change := range []func(*Envelope){
		func(e *Envelope) { e.Version = 5 }, func(e *Envelope) { e.Seq = 1 },
		func(e *Envelope) { e.CommandID = "command" }, func(e *Envelope) { e.RequestID = "request" },
		func(e *Envelope) { e.Payload = json.RawMessage(`{"text":"forbidden"}`) },
	} {
		e := valid
		change(&e)
		if ValidateEnvelope(e) == nil {
			t.Fatalf("accepted invalid heartbeat: %+v", e)
		}
	}
}

func TestWebSocketHeartbeatDoesNotSubmitInput(t *testing.T) {
	fixture := newServerFixture(t, nil)
	connection := fixture.dial(t)
	defer connection.Close()
	writeVersionedTestEnvelope(t, connection, 6, TypeHello, "session-1", HelloPayload{PairToken: fixture.token})
	if readTestEnvelope(t, connection).Type != TypeHelloAck {
		t.Fatal("missing hello ack")
	}
	writeVersionedTestEnvelope(t, connection, 6, TypeSubscribe, "session-1", struct{}{})
	if readTestEnvelope(t, connection).Type != TypeSubscribed {
		t.Fatal("missing subscribed")
	}
	snapshot := readTestEnvelope(t, connection)
	if snapshot.Type != TypeSnapshot {
		t.Fatal("missing snapshot")
	}
	for i := 0; i < 3; i++ {
		writeVersionedTestEnvelope(t, connection, 6, TypePing, "session-1", struct{}{})
		pong := readTestEnvelope(t, connection)
		if pong.Type != TypePong || pong.SessionID != "session-1" || pong.Seq != 0 || pong.CommandID != "" || pong.RequestID != "" {
			t.Fatalf("invalid pong: %+v", pong)
		}
	}
	fixture.controller.mu.Lock()
	inputs := len(fixture.controller.inputs)
	fixture.controller.mu.Unlock()
	if inputs != 0 {
		t.Fatal("heartbeat submitted Claude input")
	}
	// A mismatched Session must terminate rather than acknowledge the probe.
	writeVersionedTestEnvelope(t, connection, 6, TypePing, "other", struct{}{})
	if readTestEnvelope(t, connection).Type != TypeError {
		t.Fatal("cross-session heartbeat accepted")
	}
}
