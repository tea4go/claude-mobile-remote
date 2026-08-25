package claude

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	claudecode "github.com/tea4go/claude-agent-sdk-go"
)

type fakeClient struct {
	connectErr    error
	queryErr      error
	disconnectErr error
	queries       []string
	iterators     []claudecode.MessageIterator
	connectCalls  int
	disconnects   int
}

func (f *fakeClient) Connect(context.Context, ...claudecode.StreamMessage) error {
	f.connectCalls++
	return f.connectErr
}

func (f *fakeClient) Query(_ context.Context, prompt string) error {
	f.queries = append(f.queries, prompt)
	return f.queryErr
}

func (f *fakeClient) ReceiveResponse(context.Context) claudecode.MessageIterator {
	iterator := f.iterators[0]
	f.iterators = f.iterators[1:]
	return iterator
}

func (f *fakeClient) Disconnect() error {
	f.disconnects++
	return f.disconnectErr
}

type iteratorStep struct {
	message claudecode.Message
	err     error
}

type fakeIterator struct {
	steps  []iteratorStep
	closed int
}

func (f *fakeIterator) Next(context.Context) (claudecode.Message, error) {
	if len(f.steps) == 0 {
		return nil, claudecode.ErrNoMoreMessages
	}
	step := f.steps[0]
	f.steps = f.steps[1:]
	return step.message, step.err
}

func (f *fakeIterator) Close() error {
	f.closed++
	return nil
}

func messageStep(message claudecode.Message) iteratorStep {
	return iteratorStep{message: message}
}

func collectEvents() (*[]Event, func(Event)) {
	events := []Event{}
	return &events, func(event Event) { events = append(events, event) }
}

func connectHost(t *testing.T, client *fakeClient) *Host {
	t.Helper()
	host := NewHost(withClient(client))
	if err := host.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	return host
}

func TestHostTwoTurnsKeepClientConnected(t *testing.T) {
	first := &fakeIterator{steps: []iteratorStep{
		messageStep(&claudecode.SystemMessage{Subtype: "init", Data: map[string]any{"session_id": "session-1"}}),
		messageStep(&claudecode.AssistantMessage{Content: []claudecode.ContentBlock{&claudecode.TextBlock{Text: "one"}}}),
		messageStep(&claudecode.ResultMessage{SessionID: "session-1"}),
	}}
	second := &fakeIterator{steps: []iteratorStep{
		messageStep(&claudecode.AssistantMessage{Content: []claudecode.ContentBlock{&claudecode.TextBlock{Text: "two"}}}),
		messageStep(&claudecode.ResultMessage{SessionID: "session-1"}),
	}}
	client := &fakeClient{iterators: []claudecode.MessageIterator{first, second}}
	host := connectHost(t, client)

	firstEvents, emitFirst := collectEvents()
	if err := host.RunTurn(context.Background(), "first", emitFirst); err != nil {
		t.Fatalf("first RunTurn() error = %v", err)
	}
	secondEvents, emitSecond := collectEvents()
	if err := host.RunTurn(context.Background(), "second", emitSecond); err != nil {
		t.Fatalf("second RunTurn() error = %v", err)
	}

	if !reflect.DeepEqual(client.queries, []string{"first", "second"}) {
		t.Fatalf("queries = %#v", client.queries)
	}
	if host.SessionID() != "session-1" {
		t.Fatalf("SessionID() = %q", host.SessionID())
	}
	if first.closed != 1 || second.closed != 1 {
		t.Fatalf("iterator closes = %d, %d", first.closed, second.closed)
	}
	if client.disconnects != 0 {
		t.Fatalf("disconnects before Close = %d", client.disconnects)
	}
	if (*firstEvents)[0].Kind != EventSessionStarted || (*firstEvents)[0].Source != SessionIDFromInit {
		t.Fatalf("first event = %#v", (*firstEvents)[0])
	}
	if (*secondEvents)[len(*secondEvents)-1].Kind != EventTurnCompleted {
		t.Fatalf("second final event = %#v", (*secondEvents)[len(*secondEvents)-1])
	}
	if err := host.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if client.disconnects != 1 {
		t.Fatalf("disconnects after Close = %d", client.disconnects)
	}
}

func TestHostResultSessionFallback(t *testing.T) {
	result := "fallback"
	client := &fakeClient{iterators: []claudecode.MessageIterator{&fakeIterator{steps: []iteratorStep{
		messageStep(&claudecode.ResultMessage{SessionID: "session-1", Result: &result}),
	}}}}
	host := connectHost(t, client)
	events, emit := collectEvents()
	if err := host.RunTurn(context.Background(), "prompt", emit); err != nil {
		t.Fatalf("RunTurn() error = %v", err)
	}
	want := []Event{
		{Kind: EventSessionStarted, SessionID: "session-1", Source: SessionIDFromResult},
		{Kind: EventText, SessionID: "session-1", Text: "fallback"},
		{Kind: EventTurnCompleted, SessionID: "session-1"},
	}
	if !reflect.DeepEqual(*events, want) {
		t.Fatalf("events = %#v, want %#v", *events, want)
	}
}

func TestHostDoesNotDuplicateResultText(t *testing.T) {
	result := "same"
	client := &fakeClient{iterators: []claudecode.MessageIterator{&fakeIterator{steps: []iteratorStep{
		messageStep(&claudecode.SystemMessage{Subtype: "init", Data: map[string]any{"session_id": "session-1"}}),
		messageStep(&claudecode.AssistantMessage{Content: []claudecode.ContentBlock{&claudecode.TextBlock{Text: "same"}}}),
		messageStep(&claudecode.ResultMessage{SessionID: "session-1", Result: &result}),
	}}}}
	host := connectHost(t, client)
	events, emit := collectEvents()
	if err := host.RunTurn(context.Background(), "prompt", emit); err != nil {
		t.Fatalf("RunTurn() error = %v", err)
	}
	textCount := 0
	for _, event := range *events {
		if event.Kind == EventText {
			textCount++
		}
	}
	if textCount != 1 {
		t.Fatalf("text event count = %d, events = %#v", textCount, *events)
	}
}

func TestHostRejectsChangedSessionID(t *testing.T) {
	client := &fakeClient{iterators: []claudecode.MessageIterator{&fakeIterator{steps: []iteratorStep{
		messageStep(&claudecode.SystemMessage{Subtype: "init", Data: map[string]any{"session_id": "session-1"}}),
		messageStep(&claudecode.ResultMessage{SessionID: "session-2"}),
	}}}}
	host := connectHost(t, client)
	err := host.RunTurn(context.Background(), "prompt", nil)
	if ScopeOf(err) != ScopeSession || !strings.Contains(err.Error(), "session-1") || !strings.Contains(err.Error(), "session-2") {
		t.Fatalf("RunTurn() error = %v", err)
	}
}

func TestHostRequiresResultSessionID(t *testing.T) {
	t.Run("first turn after init", func(t *testing.T) {
		client := &fakeClient{iterators: []claudecode.MessageIterator{&fakeIterator{steps: []iteratorStep{
			messageStep(&claudecode.SystemMessage{Subtype: "init", Data: map[string]any{"session_id": "session-1"}}),
			messageStep(&claudecode.ResultMessage{}),
		}}}}
		host := connectHost(t, client)
		err := host.RunTurn(context.Background(), "prompt", nil)
		if err == nil || ScopeOf(err) != ScopeSession || !strings.Contains(err.Error(), "ResultMessage 未提供 session ID") {
			t.Fatalf("RunTurn() error = %v", err)
		}
	})

	t.Run("later turn", func(t *testing.T) {
		client := &fakeClient{iterators: []claudecode.MessageIterator{
			&fakeIterator{steps: []iteratorStep{
				messageStep(&claudecode.SystemMessage{Subtype: "init", Data: map[string]any{"session_id": "session-1"}}),
				messageStep(&claudecode.ResultMessage{SessionID: "session-1"}),
			}},
			&fakeIterator{steps: []iteratorStep{messageStep(&claudecode.ResultMessage{})}},
		}}
		host := connectHost(t, client)
		if err := host.RunTurn(context.Background(), "first", nil); err != nil {
			t.Fatalf("first RunTurn() error = %v", err)
		}
		err := host.RunTurn(context.Background(), "second", nil)
		if err == nil || ScopeOf(err) != ScopeSession || !strings.Contains(err.Error(), "ResultMessage 未提供 session ID") {
			t.Fatalf("second RunTurn() error = %v", err)
		}
	})
}

func TestHostResultErrorIsTurnScoped(t *testing.T) {
	result := "denied"
	client := &fakeClient{iterators: []claudecode.MessageIterator{
		&fakeIterator{steps: []iteratorStep{
			messageStep(&claudecode.SystemMessage{Subtype: "init", Data: map[string]any{"session_id": "session-1"}}),
			messageStep(&claudecode.ResultMessage{SessionID: "session-1", IsError: true, Result: &result, Errors: []string{"policy blocked"}}),
		}},
		&fakeIterator{steps: []iteratorStep{messageStep(&claudecode.ResultMessage{SessionID: "session-1"})}},
	}}
	host := connectHost(t, client)
	events, emit := collectEvents()
	err := host.RunTurn(context.Background(), "first", emit)
	if ScopeOf(err) != ScopeTurn {
		t.Fatalf("ScopeOf(error) = %q, error = %v", ScopeOf(err), err)
	}
	if !strings.Contains(err.Error(), "denied") || !strings.Contains(err.Error(), "policy blocked") {
		t.Fatalf("error must contain Result and Errors, got %v", err)
	}
	if (*events)[len(*events)-1].Kind != EventTurnFailed {
		t.Fatalf("final event = %#v", (*events)[len(*events)-1])
	}
	if err := host.RunTurn(context.Background(), "second", nil); err != nil {
		t.Fatalf("second RunTurn() error = %v", err)
	}
}

func TestHostEOFBeforeResultIsSessionScoped(t *testing.T) {
	client := &fakeClient{iterators: []claudecode.MessageIterator{&fakeIterator{}}}
	host := connectHost(t, client)
	err := host.RunTurn(context.Background(), "prompt", nil)
	if ScopeOf(err) != ScopeSession || !strings.Contains(err.Error(), "未收到 ResultMessage") {
		t.Fatalf("RunTurn() error = %v", err)
	}
}

func TestHostIteratorErrorIsSessionScoped(t *testing.T) {
	root := errors.New("stream broke")
	client := &fakeClient{iterators: []claudecode.MessageIterator{&fakeIterator{steps: []iteratorStep{{err: root}}}}}
	host := connectHost(t, client)
	err := host.RunTurn(context.Background(), "prompt", nil)
	if ScopeOf(err) != ScopeSession || !errors.Is(err, root) {
		t.Fatalf("RunTurn() error = %v", err)
	}
}

func TestHostConnectQueryAndCloseErrors(t *testing.T) {
	t.Run("connect", func(t *testing.T) {
		root := errors.New("connect failed")
		host := NewHost(withClient(&fakeClient{connectErr: root}))
		err := host.Connect(context.Background())
		if ScopeOf(err) != ScopeSession || !errors.Is(err, root) {
			t.Fatalf("Connect() error = %v", err)
		}
	})

	t.Run("query", func(t *testing.T) {
		root := errors.New("query failed")
		client := &fakeClient{queryErr: root}
		host := connectHost(t, client)
		err := host.RunTurn(context.Background(), "prompt", nil)
		if ScopeOf(err) != ScopeSession || !errors.Is(err, root) {
			t.Fatalf("RunTurn() error = %v", err)
		}
	})

	t.Run("close is idempotent", func(t *testing.T) {
		root := errors.New("disconnect failed")
		client := &fakeClient{disconnectErr: root}
		host := connectHost(t, client)
		if err := host.Close(); !errors.Is(err, root) {
			t.Fatalf("Close() error = %v", err)
		}
		if err := host.Close(); err != nil {
			t.Fatalf("second Close() error = %v", err)
		}
		if client.disconnects != 1 {
			t.Fatalf("disconnects = %d", client.disconnects)
		}
	})
}

func TestHostRequiresConnectAndNonEmptyPrompt(t *testing.T) {
	client := &fakeClient{}
	host := NewHost(withClient(client))
	if err := host.RunTurn(context.Background(), "prompt", nil); ScopeOf(err) != ScopeSession {
		t.Fatalf("unconnected RunTurn() error = %v", err)
	}
	if err := host.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if err := host.RunTurn(context.Background(), "  ", nil); ScopeOf(err) != ScopeTurn {
		t.Fatalf("empty RunTurn() error = %v", err)
	}
	if len(client.queries) != 0 {
		t.Fatalf("queries = %#v", client.queries)
	}
}

func TestHostAssistantErrorFailsTurn(t *testing.T) {
	serverError := claudecode.AssistantMessageErrorServer
	t.Run("server error is turn scoped", func(t *testing.T) {
		iterator := &fakeIterator{steps: []iteratorStep{
			messageStep(&claudecode.SystemMessage{Subtype: "init", Data: map[string]any{"session_id": "session-1"}}),
			messageStep(&claudecode.AssistantMessage{Error: &serverError, Content: []claudecode.ContentBlock{&claudecode.TextBlock{Text: "overloaded"}}}),
			// 错误 assistant 之后的 Result 不应被消费：错误即终止当前流。
			messageStep(&claudecode.ResultMessage{SessionID: "session-1"}),
		}}
		client := &fakeClient{iterators: []claudecode.MessageIterator{iterator}}
		host := connectHost(t, client)
		events, emit := collectEvents()
		err := host.RunTurn(context.Background(), "prompt", emit)
		if ScopeOf(err) != ScopeTurn || !strings.Contains(err.Error(), "server_error") || !strings.Contains(err.Error(), "overloaded") {
			t.Fatalf("RunTurn() error = %v", err)
		}
		// iterator 已消费到错误消息即停；剩余 step 未读。
		if len(iterator.steps) != 1 {
			t.Fatalf("stream must stop at error assistant, remaining steps = %d", len(iterator.steps))
		}
		last := (*events)[len(*events)-1]
		if last.Kind != EventTurnFailed || last.SessionID != "session-1" {
			t.Fatalf("final event = %#v", last)
		}
	})

	authError := claudecode.AssistantMessageErrorAuthFailed
	t.Run("auth error is session scoped", func(t *testing.T) {
		client := &fakeClient{iterators: []claudecode.MessageIterator{&fakeIterator{steps: []iteratorStep{
			messageStep(&claudecode.SystemMessage{Subtype: "init", Data: map[string]any{"session_id": "session-1"}}),
			messageStep(&claudecode.AssistantMessage{Error: &authError}),
		}}}}
		host := connectHost(t, client)
		err := host.RunTurn(context.Background(), "prompt", nil)
		if ScopeOf(err) != ScopeSession || !strings.Contains(err.Error(), "authentication_failed") {
			t.Fatalf("RunTurn() error = %v", err)
		}
	})
}

func TestDenyToolUse(t *testing.T) {
	result, err := denyToolUse(
		context.Background(),
		"Bash",
		map[string]any{"command": "true"},
		claudecode.ToolPermissionContext{},
	)
	if err != nil {
		t.Fatalf("denyToolUse() error = %v", err)
	}
	deny, ok := result.(claudecode.PermissionResultDeny)
	if !ok {
		t.Fatalf("denyToolUse() result type = %T", result)
	}
	if deny.Behavior != "deny" || strings.TrimSpace(deny.Message) == "" || deny.Interrupt {
		t.Fatalf("deny result = %#v", deny)
	}
}

// ---- A5.2 预分配 session ID（设计 §4/§5）----

// newHostWithReservedID 构造带预分配 ID 的 Host（新会话模式）。
func newHostWithReservedID(t *testing.T, client *fakeClient, sessionID string) *Host {
	t.Helper()
	host, err := NewHostWithReservedSessionID(sessionID, withClient(client))
	if err != nil {
		t.Fatalf("NewHostWithReservedSessionID() error = %v", err)
	}
	return host
}

func TestHostReserveSessionIDForNewConversation(t *testing.T) {
	sessionID := "9c3d7e2a-1b4f-4c5d-8e6a-2f7b9d0c4a11"
	client := &fakeClient{iterators: []claudecode.MessageIterator{&fakeIterator{steps: []iteratorStep{
		messageStep(&claudecode.SystemMessage{Subtype: "init", Data: map[string]any{"session_id": sessionID}}),
		messageStep(&claudecode.AssistantMessage{Content: []claudecode.ContentBlock{&claudecode.TextBlock{Text: "ok"}}}),
		messageStep(&claudecode.ResultMessage{SessionID: sessionID}),
	}}}}
	host := newHostWithReservedID(t, client, sessionID)

	if err := host.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	// Connect 后预留 ID 已可读，但尚未确认（设计 §5.1）。
	if host.SessionID() != sessionID {
		t.Fatalf("SessionID() after Connect = %q, want reserved %q", host.SessionID(), sessionID)
	}
	if host.SessionConfirmed() {
		t.Fatal("SessionConfirmed() must be false before the first init/Result")
	}

	events, emit := collectEvents()
	if err := host.RunTurn(context.Background(), "prompt", emit); err != nil {
		t.Fatalf("RunTurn() error = %v", err)
	}
	if !host.SessionConfirmed() {
		t.Fatal("SessionConfirmed() must be true after init confirmed the reserved ID")
	}
	// session_started 仍只发一次，且 ID 与预留一致。
	started := 0
	for _, event := range *events {
		if event.Kind == EventSessionStarted {
			started++
			if event.SessionID != sessionID {
				t.Fatalf("session_started ID = %q, want %q", event.SessionID, sessionID)
			}
		}
	}
	if started != 1 {
		t.Fatalf("session_started count = %d, want 1", started)
	}
}

func TestHostReservedSessionIDMismatchFailsSession(t *testing.T) {
	client := &fakeClient{iterators: []claudecode.MessageIterator{&fakeIterator{steps: []iteratorStep{
		messageStep(&claudecode.SystemMessage{Subtype: "init", Data: map[string]any{"session_id": "session-other"}}),
	}}}}
	host := newHostWithReservedID(t, client, "9c3d7e2a-1b4f-4c5d-8e6a-2f7b9d0c4a11")
	if err := host.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	err := host.RunTurn(context.Background(), "prompt", nil)
	if err == nil || ScopeOf(err) != ScopeSession {
		t.Fatalf("RunTurn() error = %v, want session-scoped mismatch failure", err)
	}
	if host.SessionConfirmed() {
		t.Fatal("SessionConfirmed() must remain false on mismatch")
	}
}

func TestHostReserveRejectsInvalidInput(t *testing.T) {
	if _, err := NewHostWithReservedSessionID("not-a-uuid", withClient(&fakeClient{})); err == nil {
		t.Fatal("NewHostWithReservedSessionID() accepted invalid UUID")
	}
	if _, err := NewHostWithReservedSessionID("", withClient(&fakeClient{})); err == nil {
		t.Fatal("NewHostWithReservedSessionID() accepted empty session ID")
	}
}

func TestHostNewConversationGeneratesUUID(t *testing.T) {
	// NewHost 新会话模式：内部生成 UUID v4 并预分配（设计 §3.1）。
	host := NewHost(withClient(&fakeClient{}))
	if err := host.PrepareNewSession(); err != nil {
		t.Fatalf("PrepareNewSession() error = %v", err)
	}
	id := host.SessionID()
	if id == "" {
		t.Fatal("PrepareNewSession() did not reserve a session ID")
	}
	if len(id) != 36 || id[14] != '4' {
		t.Fatalf("reserved ID %q is not a UUID v4", id)
	}
	// 第二次调用拒绝：一个 Host 只允许一次预留。
	if err := host.PrepareNewSession(); err == nil {
		t.Fatal("second PrepareNewSession() must fail")
	}
}

// TestSessionSDKOptionsSeparateNewAndResume 校验设计 §4 的互斥要求：新建模式
// 只配置 WithSessionID，恢复模式只配置 WithResume，两者不得同时出现。
func TestSessionSDKOptionsSeparateNewAndResume(t *testing.T) {
	sessionID := "9c3d7e2a-1b4f-4c5d-8e6a-2f7b9d0c4a11"

	newSession := claudecode.NewOptions(sessionSDKOptions(sessionModeNew, sessionID)...)
	if newSession.SessionID == nil || *newSession.SessionID != sessionID {
		t.Fatalf("new session SessionID = %v, want %q", newSession.SessionID, sessionID)
	}
	if newSession.Resume != nil {
		t.Fatalf("new session must not set Resume, got %q", *newSession.Resume)
	}
	if err := newSession.Validate(); err != nil {
		t.Fatalf("new session Validate() error = %v", err)
	}

	resumed := claudecode.NewOptions(sessionSDKOptions(sessionModeResume, sessionID)...)
	if resumed.Resume == nil || *resumed.Resume != sessionID {
		t.Fatalf("resume Resume = %v, want %q", resumed.Resume, sessionID)
	}
	if resumed.SessionID != nil {
		t.Fatalf("resume must not set SessionID, got %q", *resumed.SessionID)
	}
	if err := resumed.Validate(); err != nil {
		t.Fatalf("resume Validate() error = %v", err)
	}
}

// TestHostResumeReservesExistingID 覆盖设计 §5.2：恢复会话在 Connect 前即持有
// 已有 ID，首个 init/Result 必须返回同一 ID。
func TestHostResumeReservesExistingID(t *testing.T) {
	sessionID := "9c3d7e2a-1b4f-4c5d-8e6a-2f7b9d0c4a11"
	client := &fakeClient{iterators: []claudecode.MessageIterator{&fakeIterator{steps: []iteratorStep{
		messageStep(&claudecode.SystemMessage{Subtype: "init", Data: map[string]any{"session_id": sessionID}}),
		messageStep(&claudecode.ResultMessage{SessionID: sessionID}),
	}}}}
	host, err := NewHostForResume(sessionID, withClient(client))
	if err != nil {
		t.Fatalf("NewHostForResume() error = %v", err)
	}
	if host.SessionID() != sessionID {
		t.Fatalf("SessionID() = %q, want %q", host.SessionID(), sessionID)
	}
	if host.SessionConfirmed() {
		t.Fatal("resume host must not be confirmed before the first init/Result")
	}
	if err := host.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if err := host.RunTurn(context.Background(), "prompt", nil); err != nil {
		t.Fatalf("RunTurn() error = %v", err)
	}
	if !host.SessionConfirmed() {
		t.Fatal("resume host must confirm the existing session ID")
	}
}

// TestHostResumeRejectsNewIDFromClaude 覆盖设计 §11：Resume 实际返回新 ID 时
// 按 Session 级失败处理，禁止静默切换。
func TestHostResumeRejectsNewIDFromClaude(t *testing.T) {
	client := &fakeClient{iterators: []claudecode.MessageIterator{&fakeIterator{steps: []iteratorStep{
		messageStep(&claudecode.SystemMessage{Subtype: "init", Data: map[string]any{"session_id": "other-session"}}),
	}}}}
	host, err := NewHostForResume("9c3d7e2a-1b4f-4c5d-8e6a-2f7b9d0c4a11", withClient(client))
	if err != nil {
		t.Fatalf("NewHostForResume() error = %v", err)
	}
	if err := host.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	turnErr := host.RunTurn(context.Background(), "prompt", nil)
	if turnErr == nil || ScopeOf(turnErr) != ScopeSession {
		t.Fatalf("RunTurn() error = %v, want session-scoped mismatch failure", turnErr)
	}
	if host.SessionConfirmed() {
		t.Fatal("resume host must stay unconfirmed on ID mismatch")
	}
}

func TestHostResumeRejectsInvalidID(t *testing.T) {
	if _, err := NewHostForResume("not-a-uuid", withClient(&fakeClient{})); err == nil {
		t.Fatal("NewHostForResume() accepted invalid UUID")
	}
	if _, err := NewHostForResume("", withClient(&fakeClient{})); err == nil {
		t.Fatal("NewHostForResume() accepted empty session ID")
	}
}

func TestHostUnreservedKeepsLegacyBehavior(t *testing.T) {
	// 未预留 ID 的 Host 保持 A2 行为：SessionID 首轮 init 前为空。
	client := &fakeClient{iterators: []claudecode.MessageIterator{&fakeIterator{steps: []iteratorStep{
		messageStep(&claudecode.SystemMessage{Subtype: "init", Data: map[string]any{"session_id": "session-1"}}),
		messageStep(&claudecode.ResultMessage{SessionID: "session-1"}),
	}}}}
	host := connectHost(t, client)
	if host.SessionID() != "" || host.SessionConfirmed() {
		t.Fatalf("legacy host must have empty/unconfirmed session before first turn")
	}
	if err := host.RunTurn(context.Background(), "prompt", nil); err != nil {
		t.Fatalf("RunTurn() error = %v", err)
	}
	if host.SessionID() != "session-1" || !host.SessionConfirmed() {
		t.Fatal("legacy host must confirm session ID from init")
	}
}
