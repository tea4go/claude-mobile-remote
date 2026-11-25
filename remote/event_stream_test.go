package remote

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/tea4go/claude-mobile-remote/protocol"
)

func TestPublishQueueKeepsSnapshotAndEventAtSameSequence(t *testing.T) {
	stream, err := NewEventStream(protocol.Snapshot{SessionID: "s"})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	_, events, err := stream.Subscribe(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	items := []protocol.QueuedRequest{{RequestID: "q", Source: protocol.InputTerminal, Preview: "hello"}}
	if err := stream.PublishQueue(items); err != nil {
		t.Fatal(err)
	}
	items[0].Preview = "changed"
	event := <-events
	snapshot, err := stream.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if event.Seq != snapshot.Seq || event.Queue == nil || len(*event.Queue) != 1 || (*event.Queue)[0].Preview != "hello" || snapshot.QueueDepth != 1 || snapshot.QueuedRequests[0].Preview != "hello" {
		t.Fatalf("event=%#v snapshot=%#v", event, snapshot)
	}
	if err := stream.PublishQueue(nil); err != nil {
		t.Fatal(err)
	}
	event = <-events
	if event.Queue == nil || len(*event.Queue) != 0 {
		t.Fatalf("empty queue event=%#v", event)
	}
}

func TestEventStreamBroadcastsIndependently(t *testing.T) {
	stream, err := NewEventStream(protocol.Snapshot{SessionID: "session-1"})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	_, first, err := stream.Subscribe(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	_, second, err := stream.Subscribe(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}

	event := protocol.Event{Kind: protocol.EventTurnStarted, SessionID: "session-1", RequestID: "request-1", Text: "hello", TurnStatus: protocol.TurnRunning}
	if err := stream.Publish(event); err != nil {
		t.Fatal(err)
	}
	for name, events := range map[string]<-chan protocol.Event{"first": first, "second": second} {
		got := <-events
		if got.Seq != 1 || got.RequestID != "request-1" {
			t.Fatalf("%s subscriber event = %#v", name, got)
		}
	}
}

func TestEventStreamKeepsFirstTitleAndActivityWithoutSubscribers(t *testing.T) {
	stream, err := NewEventStream(protocol.Snapshot{SessionID: "session-1", ProjectCwd: "/work/project"})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	first := protocol.Event{Kind: protocol.EventTurnStarted, SessionID: "session-1", RequestID: "r1", Text: "修复 登录\n后续正文", TurnStatus: protocol.TurnRunning}
	if err := stream.Publish(first); err != nil {
		t.Fatal(err)
	}
	if err := stream.Publish(protocol.Event{Kind: protocol.EventTurnStarted, SessionID: "session-1", RequestID: "r2", Text: "第二轮", TurnStatus: protocol.TurnRunning}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := stream.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ProjectCwd != "/work/project" || snapshot.SessionTitle != "修复 登录" || snapshot.LastActivityAt == "" {
		t.Fatalf("display snapshot = %+v", snapshot)
	}
	if err := stream.UpdateSnapshot(protocol.Snapshot{SessionID: "session-1", ProjectCwd: "/work/project"}); err != nil {
		t.Fatal(err)
	}
	snapshot, err = stream.Snapshot(context.Background())
	if err != nil || snapshot.SessionTitle != "修复 登录" || snapshot.LastActivityAt == "" {
		t.Fatalf("display metadata was lost after refresh: %+v, %v", snapshot, err)
	}
}

func TestEventStreamRejectsCrossSessionData(t *testing.T) {
	stream, err := NewEventStream(protocol.Snapshot{SessionID: "session-1"})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	if err := stream.Publish(protocol.Event{SessionID: "session-2"}); !errors.Is(err, protocol.ErrSessionMismatch) {
		t.Fatalf("Publish() error = %v", err)
	}
	if err := stream.UpdateSnapshot(protocol.Snapshot{SessionID: "session-2"}); !errors.Is(err, protocol.ErrSessionMismatch) {
		t.Fatalf("UpdateSnapshot() error = %v", err)
	}
}

func TestEventStreamClosesOnlySlowSubscriber(t *testing.T) {
	stream, err := NewEventStream(protocol.Snapshot{SessionID: "session-1"})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	_, slow, err := stream.Subscribe(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	_, fast, err := stream.Subscribe(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}

	first := protocol.Event{Kind: protocol.EventTextDelta, SessionID: "session-1", RequestID: "request-1", Text: "a"}
	second := protocol.Event{Kind: protocol.EventTextDelta, SessionID: "session-1", RequestID: "request-1", Text: "b"}
	if err := stream.Publish(first); err != nil {
		t.Fatal(err)
	}
	if err := stream.Publish(second); err != nil {
		t.Fatal(err)
	}
	if got := <-fast; got.Text != "a" {
		t.Fatalf("fast first event = %#v", got)
	}
	if got := <-fast; got.Text != "b" {
		t.Fatalf("fast second event = %#v", got)
	}
	if got := <-slow; got.Text != "a" {
		t.Fatalf("slow buffered event = %#v", got)
	}
	if _, ok := <-slow; ok {
		t.Fatal("slow subscriber remained open after overflow")
	}
}

func TestSnapshotRetainsWholeReplyAfterHostRefresh(t *testing.T) {
	stream, _ := NewEventStream(protocol.Snapshot{SessionID: "s"})
	for _, event := range []protocol.Event{
		{Kind: protocol.EventTurnStarted, RequestID: "r", Source: protocol.InputMobile, Text: "question", TurnStatus: protocol.TurnRunning},
		{Kind: protocol.EventTextDelta, RequestID: "r", Text: "first "},
		{Kind: protocol.EventTextDelta, RequestID: "r", Text: "second"},
		{Kind: protocol.EventTurnCompleted, RequestID: "r", TurnStatus: protocol.TurnCompleted},
	} {
		if err := stream.Publish(event); err != nil {
			t.Fatal(err)
		}
	}
	if err := stream.UpdateSnapshot(protocol.Snapshot{SessionID: "s", Turn: protocol.TurnSnapshot{Status: protocol.TurnIdle}}); err != nil {
		t.Fatal(err)
	}
	snapshot, _, err := stream.Subscribe(context.Background(), 1)
	if err != nil || snapshot.Turn.InputText != "question" || snapshot.Turn.Message != "first second" || snapshot.Turn.Status != protocol.TurnCompleted || snapshot.Turn.RequestID != "r" {
		t.Fatalf("recovery = %+v, %v", snapshot, err)
	}
	_ = stream.Publish(protocol.Event{Kind: protocol.EventTurnStarted, RequestID: "r2", Text: "next", TurnStatus: protocol.TurnRunning})
	snapshot, _ = stream.Snapshot(context.Background())
	if snapshot.Turn.Message != "" || snapshot.Turn.InputText != "next" {
		t.Fatal("new turn did not replace previous messages")
	}
}

func TestReconnectSnapshotKeepsOrderedTurnDisplay(t *testing.T) {
	stream, _ := NewEventStream(protocol.Snapshot{SessionID: "s"})
	for _, event := range []protocol.Event{
		{Kind: protocol.EventTurnStarted, RequestID: "r", Source: protocol.InputTerminal, Text: "input", TurnStatus: protocol.TurnRunning},
		{Kind: protocol.EventTextDelta, RequestID: "r", Text: "before"},
		{Kind: protocol.EventContentBlock, RequestID: "r", Block: &protocol.ContentBlock{ID: "b", Kind: protocol.BlockThinking, Thinking: "think"}},
		{Kind: protocol.EventTextDelta, RequestID: "r", Text: "after"},
		{Kind: protocol.EventTurnCompleted, RequestID: "r", TurnStatus: protocol.TurnCompleted},
	} {
		if err := stream.Publish(event); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, events, err := stream.Subscribe(context.Background(), 2)
	if err != nil || snapshot.Turn.Display == nil || len(snapshot.Turn.Display.Items) != 3 || snapshot.Turn.Display.Items[0].Text != "before" || snapshot.Turn.Display.Items[1].Block.ID != "b" || snapshot.Turn.Display.Items[2].Text != "after" {
		t.Fatalf("snapshot = %#v err=%v", snapshot.Turn.Display, err)
	}
	if err = stream.Publish(protocol.Event{Kind: protocol.EventSessionStarted}); err != nil {
		t.Fatal(err)
	}
	if event := <-events; event.Seq != snapshot.Seq+1 {
		t.Fatalf("sequence %d after snapshot %d", event.Seq, snapshot.Seq)
	}
}

func TestPairingDuringLongTurnKeepsReplyWhenStepsOverflow(t *testing.T) {
	stream, err := NewEventStream(protocol.Snapshot{SessionID: "s"})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	publish := func(event protocol.Event) {
		t.Helper()
		event.RequestID, event.Source = "active", protocol.InputTerminal
		if err := stream.Publish(event); err != nil {
			t.Fatal(err)
		}
	}
	publish(protocol.Event{Kind: protocol.EventTurnStarted, TurnStatus: protocol.TurnRunning, Text: "question"})
	publish(protocol.Event{Kind: protocol.EventTextDelta, Text: "earlier reply", Model: "example"})
	for i := range protocol.MaxTurnDisplayItems {
		publish(protocol.Event{Kind: protocol.EventContentBlock, Block: &protocol.ContentBlock{ID: strconv.Itoa(i), Kind: protocol.BlockThinking, Thinking: "step"}})
	}
	publish(protocol.Event{Kind: protocol.EventTextDelta, Text: "later reply", Model: "example"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	snapshot, events, err := stream.Subscribe(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	display := snapshot.Turn.Display
	if snapshot.Turn.Status != protocol.TurnRunning || snapshot.Turn.InputText != "question" || display == nil || !display.Truncated {
		t.Fatalf("expected a running snapshot with truncated steps: %+v", snapshot.Turn)
	}
	items := display.Items
	if len(items) != protocol.MaxTurnDisplayItems || items[0].Text != "earlier reply" || items[0].Model != "example" || items[len(items)-1].Text != "later reply" {
		t.Fatal("trimming steps lost or reordered the reply")
	}
	if items[1].Block.ID != "2" || items[len(items)-2].Block.ID != strconv.Itoa(protocol.MaxTurnDisplayItems-1) {
		t.Fatal("remaining steps lost their order")
	}
	publish(protocol.Event{Kind: protocol.EventTextDelta, Text: " continues"})
	if event := <-events; event.Seq != snapshot.Seq+1 || event.Text != " continues" {
		t.Fatalf("first live event after pairing: %+v", event)
	}
}

func TestSteeredInputDoesNotReplaceActiveTurn(t *testing.T) {
	stream, err := NewEventStream(protocol.Snapshot{SessionID: "session-1", SessionStatus: protocol.SessionReady,
		Turn: protocol.TurnSnapshot{RequestID: "active", Source: protocol.InputTerminal, Status: protocol.TurnRunning}})
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.Publish(protocol.Event{Kind: protocol.EventInputSteered, RequestID: "queued", Source: protocol.InputMobile, Text: "follow-up"}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := stream.Snapshot(context.Background())
	if err != nil || snapshot.Seq != 1 || snapshot.Turn.RequestID != "active" || snapshot.Turn.Source != protocol.InputTerminal {
		t.Fatalf("steered input changed active turn: %+v, %v", snapshot, err)
	}
}

func TestBackgroundSubagentSnapshotKeepsForegroundAndQueue(t *testing.T) {
	stream, err := NewEventStream(protocol.Snapshot{SessionID: "session"})
	if err != nil {
		t.Fatal(err)
	}
	publish := func(event protocol.Event) {
		t.Helper()
		if err := stream.Publish(event); err != nil {
			t.Fatal(err)
		}
	}
	turn := func(id string) protocol.Event {
		return protocol.Event{Kind: protocol.EventTurnStarted, RequestID: id, Source: protocol.InputTerminal, Text: id, TurnStatus: protocol.TurnRunning}
	}
	task := func(status protocol.SubagentStatus) protocol.Event {
		return protocol.Event{Kind: protocol.EventSubagentTask, RequestID: "origin",
			SubagentTask: &protocol.SubagentTask{RequestID: "origin", ToolUseID: "agent", Status: status, Summary: "Inspect files"}}
	}
	publish(turn("origin"))
	publish(protocol.Event{Kind: protocol.EventContentBlock, RequestID: "origin", Block: &protocol.ContentBlock{ID: "call", Kind: protocol.BlockToolUse, ToolUseID: "child", ParentToolUseID: "agent", ToolClass: protocol.ToolBuiltin, ToolName: "Read"}})
	publish(task(protocol.SubagentRunning))
	publish(protocol.Event{Kind: protocol.EventTurnCompleted, RequestID: "origin", TurnStatus: protocol.TurnCompleted})
	if err := stream.PublishQueue([]protocol.QueuedRequest{{RequestID: "queued", Source: protocol.InputMobile, Preview: "after"}}); err != nil {
		t.Fatal(err)
	}
	publish(turn("next"))
	publish(protocol.Event{Kind: protocol.EventTextDelta, RequestID: "next", Source: protocol.InputTerminal, Text: "foreground reply"})
	if request, root := stream.SubagentOrigin("child"); request != "origin" || root != "agent" {
		t.Fatalf("nested tool origin = %q, %q", request, root)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, updates, err := stream.Subscribe(ctx, 5)
	if err != nil {
		t.Fatal(err)
	}
	publish(protocol.Event{Kind: protocol.EventContentBlock, RequestID: "origin", Block: &protocol.ContentBlock{ID: "result", Kind: protocol.BlockToolResult, ToolUseID: "child", ParentToolUseID: "agent", Output: "secret detail", IsError: protocol.BoolPointer(false)}})
	publish(task(protocol.SubagentCompleted))
	publish(task(protocol.SubagentRunning))
	late := <-updates
	if !late.BackgroundChild || late.RequestID != "origin" {
		t.Fatalf("late event projection = %+v", late)
	}
	<-updates // completed task
	<-updates // stale task progress, retained only in the sequenced stream
	publish(protocol.Event{Kind: protocol.EventTextDelta, RequestID: "origin", Source: protocol.InputTerminal, Text: "late continuation"})
	if late := <-updates; !late.LateTurnContent || late.RequestID != "origin" || late.Text != "late continuation" {
		t.Fatalf("late continuation = %+v", late)
	}
	snapshot, err := stream.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.UpdateSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot, _, err = stream.Subscribe(ctx, 1)
	if err != nil || snapshot.Turn.RequestID != "next" || snapshot.Turn.Status != protocol.TurnRunning || snapshot.Turn.Message != "foreground reply" || len(snapshot.Turn.Display.Items) != 1 || len(snapshot.QueuedRequests) != 1 || snapshot.QueuedRequests[0].RequestID != "queued" {
		t.Fatalf("background work touched foreground state: %+v, %v", snapshot, err)
	}
	if len(snapshot.SubagentTasks) != 1 || snapshot.SubagentTasks[0].Status != protocol.SubagentCompleted || len(snapshot.SubagentTasks[0].Blocks) != 2 ||
		snapshot.SubagentTasks[0].Blocks[1].Output != "" {
		t.Fatalf("bad background snapshot: %+v", snapshot.SubagentTasks)
	}
}
