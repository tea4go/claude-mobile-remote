package transport

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/tea4go/claude-mobile-remote/protocol"
)

func TestWebSocketToolDetailsAndLegacyProjection(t *testing.T) {
	fixture := newServerFixture(t, nil)
	output := strings.Repeat("first line\nsecond line\n", 3300)
	preview, truncated, detail := protocol.ToolResultDisplay(output)
	result := fixture.controller.toolDetails.Record("turn", &protocol.ContentBlock{
		ID: "result", Kind: protocol.BlockToolResult, ToolUseID: "tool", IsError: protocol.BoolPointer(false),
		Output: preview, Truncated: truncated, Detail: &detail,
	})
	child := &protocol.ContentBlock{
		ID: "block-child", Kind: protocol.BlockToolUse, ToolUseID: "child", ParentToolUseID: "tool",
		ToolClass: protocol.ToolBuiltin, ToolName: "Read", TitleSummary: "sample.go",
	}
	fixture.controller.mu.Lock()
	fixture.controller.snapshot.Turn = protocol.TurnSnapshot{Status: protocol.TurnCompleted, RequestID: "turn", Source: protocol.InputMobile,
		Display: &protocol.TurnDisplay{Items: []protocol.TurnDisplayItem{{Block: child}, {Block: result}, {Text: "reply", Model: "example-model"}}}}
	fixture.controller.mu.Unlock()

	connect := func(version int, titles bool) *websocket.Conn {
		address := "ws" + strings.TrimPrefix(fixture.testingServer.URL, "http") + "/ws"
		if titles {
			address += "?toolTitles=1"
		}
		socket, _, err := websocket.DefaultDialer.Dial(address, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = socket.Close() })
		writeVersionedTestEnvelope(t, socket, version, TypeHello, "session-1", HelloPayload{PairToken: fixture.token})
		_ = readTestEnvelope(t, socket)
		writeVersionedTestEnvelope(t, socket, version, TypeSubscribe, "session-1", struct{}{})
		_ = readTestEnvelope(t, socket)
		snapshot := readTestEnvelope(t, socket)
		if err := ValidateEnvelope(snapshot); err != nil {
			t.Fatal(err)
		}
		if version == 12 && (!bytes.Contains(snapshot.Payload, []byte("first line")) ||
			bytes.Contains(snapshot.Payload, []byte("example-model")) || bytes.Contains(snapshot.Payload, []byte("block-child"))) {
			t.Fatal("v12 did not receive the legacy snapshot projection")
		}
		if version == 13 && (bytes.Contains(snapshot.Payload, []byte("first line")) ||
			!bytes.Contains(snapshot.Payload, []byte("example-model")) || !bytes.Contains(snapshot.Payload, []byte("block-child"))) {
			t.Fatal("v13 did not receive a metadata-only tool snapshot")
		}
		if bytes.Contains(snapshot.Payload, []byte("sample.go")) != (version == 13 && titles) {
			t.Fatal("wrong title in snapshot")
		}
		return socket
	}
	current := connect(13, true)
	original := connect(13, false)
	legacy := connect(12, true)
	fixture.controller.emit(protocol.Event{SessionID: "session-1", RequestID: "turn", Seq: 8, Kind: protocol.EventContentBlock, Block: child})
	if got := readTestEnvelope(t, legacy); got.Type != TypeUnsupported || got.Seq != 8 {
		t.Fatalf("v12 child = %v/%d", got.Type, got.Seq)
	}
	if got := readTestEnvelope(t, original); bytes.Contains(got.Payload, []byte("titleSummary")) || got.Type != TypeEvent {
		t.Fatal("original v13 client received incompatible event")
	}
	if got := readTestEnvelope(t, current); !bytes.Contains(got.Payload, []byte("sample.go")) || got.Type != TypeEvent || got.Seq != 8 || !bytes.Contains(got.Payload, []byte("parentToolUseId")) {
		t.Fatalf("v13 child = %v/%d", got.Type, got.Seq)
	}
	fixture.controller.emit(protocol.Event{SessionID: "session-1", RequestID: "turn", Seq: 9, Kind: protocol.EventContentBlock, Block: result})
	if got := readTestEnvelope(t, current); got.Seq != 9 || bytes.Contains(got.Payload, []byte("first line")) || !bytes.Contains(got.Payload, []byte("detailAvailable")) {
		t.Fatal("v13 result returned inline content or lost availability")
	}
	if got := readTestEnvelope(t, legacy); got.Seq != 9 || !bytes.Contains(got.Payload, []byte("first line")) {
		t.Fatal("legacy result lost inline preview")
	}

	request := func(sessionID, requestID, commandID, toolID string) Envelope {
		t.Helper()
		message, err := NewEnvelope(TypeToolDetailGet, sessionID, commandID, requestID, 0, ToolDetailRequestPayload{ToolUseID: toolID})
		if err != nil {
			t.Fatal(err)
		}
		message.Version = 13
		if err := current.WriteJSON(message); err != nil {
			t.Fatal(err)
		}
		return readTestEnvelope(t, current)
	}
	got := request("session-1", "turn", "fetch", "tool")
	var payload ToolDetailResultPayload
	if got.Type != TypeToolDetailResult || got.CommandID != "fetch" || got.RequestID != "turn" || got.Seq != 0 ||
		ValidateEnvelope(got) != nil || DecodePayload(got, &payload) != nil || payload.Detail == nil || payload.Detail.Output != output {
		t.Fatalf("wrong detail response: %s/%s", got.Type, got.CommandID)
	}
	got = request("session-1", "other-turn", "missing", "tool")
	payload = ToolDetailResultPayload{}
	if got.Type != TypeToolDetailResult || DecodePayload(got, &payload) != nil || payload.Detail != nil {
		t.Fatal("lookup crossed a turn boundary")
	}
	if got := request("another-session", "turn", "wrong-session", "tool"); got.Type != TypeError || bytes.Contains(got.Payload, []byte("first line")) {
		t.Fatal("lookup crossed a session boundary")
	}
}

func TestWebSocketBackgroundTaskVersionAndSequence(t *testing.T) {
	fixture := newServerFixture(t, nil)
	task := protocol.SubagentTask{RequestID: "origin", ToolUseID: "agent", Status: protocol.SubagentRunning, Summary: "Observe"}
	block := protocol.ContentBlock{ID: "child", Kind: protocol.BlockToolUse, ToolUseID: "reader", ParentToolUseID: task.ToolUseID, ToolClass: protocol.ToolBuiltin, ToolName: "Read", TitleSummary: "tool.go"}
	initial := task
	initial.Blocks = []protocol.ContentBlock{block}
	fixture.controller.mu.Lock()
	fixture.controller.snapshot.SubagentTasks = []protocol.SubagentTask{initial}
	fixture.controller.mu.Unlock()
	address := "ws" + strings.TrimPrefix(fixture.testingServer.URL, "http") + "/ws?toolTitles=1"
	connect := func(version int) *websocket.Conn {
		t.Helper()
		socket, _, err := websocket.DefaultDialer.Dial(address, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = socket.Close() })
		writeVersionedTestEnvelope(t, socket, version, TypeHello, "session-1", HelloPayload{PairToken: fixture.token})
		_ = readTestEnvelope(t, socket)
		writeVersionedTestEnvelope(t, socket, version, TypeSubscribe, "session-1", struct{}{})
		_ = readTestEnvelope(t, socket)
		snapshot := readTestEnvelope(t, socket)
		if err := ValidateEnvelope(snapshot); err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(snapshot.Payload, []byte("subagentTasks")) != (version >= subagentVersion) {
			t.Fatalf("unexpected subagent snapshot for version %d", version)
		}
		return socket
	}
	previous := connect(13)
	current := connect(14)
	task.Status = protocol.SubagentCompleted
	fixture.controller.emit(protocol.Event{Kind: protocol.EventSubagentTask, SessionID: "session-1", RequestID: task.RequestID, Seq: 8, SubagentTask: &task})
	fixture.controller.emit(protocol.Event{Kind: protocol.EventContentBlock, SessionID: "session-1", RequestID: task.RequestID, Seq: 9, Block: &block, BackgroundChild: true})
	fixture.controller.emit(protocol.Event{Kind: protocol.EventTextDelta, SessionID: "session-1", RequestID: task.RequestID, Seq: 10, Source: protocol.InputTerminal, Text: "continued", LateTurnContent: true})
	for _, seq := range []uint64{8, 9, 10} {
		legacy := readTestEnvelope(t, previous)
		live := readTestEnvelope(t, current)
		if legacy.Type != TypeUnsupported || legacy.Seq != seq || live.Type != TypeEvent || live.Seq != seq || ValidateEnvelope(live) != nil {
			t.Fatalf("unexpected projections for seq %d: %v, %v", seq, legacy.Type, live.Type)
		}
	}
}
