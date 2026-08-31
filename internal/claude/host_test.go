package claude

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	claudecode "github.com/tea4go/claude-agent-sdk-go"
)

// hostTestTimeout 给所有旁路通道等待设上界，死锁表现为超时失败而非挂起。
const hostTestTimeout = time.Second

type fakeClient struct {
	connectErr    error
	queryErr      error
	disconnectErr error
	interruptErr  error
	queries       []string
	iterators     []claudecode.MessageIterator
	connectCalls  int
	disconnects   int

	mu         sync.Mutex
	interrupts int
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

func (f *fakeClient) Interrupt(context.Context) error {
	f.mu.Lock()
	f.interrupts++
	f.mu.Unlock()
	return f.interruptErr
}

func (f *fakeClient) interruptCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.interrupts
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

func connectHost(t *testing.T, client *fakeClient, options ...Option) *Host {
	t.Helper()
	host := NewHost(append([]Option{withClient(client)}, options...)...)
	if err := host.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	return host
}

// brokerHost 构造一个只用于权限回调的 Host：不跑 Turn，因此等待决议只能被
// 决议本身、Close 或 action 超时终止。
func brokerHost(t *testing.T, options ...Option) *Host {
	t.Helper()
	return connectHost(t, &fakeClient{}, append([]Option{WithActionTimeout(hostTestTimeout)}, options...)...)
}

// resolveNextAction 在后台取出下一个 action 请求、回传其展示内容并投递决议。
func resolveNextAction(host *Host, decision ActionDecision) <-chan ActionSpec {
	specs := make(chan ActionSpec, 1)
	go func() {
		select {
		case request := <-host.Actions():
			specs <- request.Spec()
			request.Resolve(decision)
		case <-time.After(hostTestTimeout):
		}
	}()
	return specs
}

func awaitSpec(t *testing.T, specs <-chan ActionSpec) ActionSpec {
	t.Helper()
	select {
	case spec := <-specs:
		return spec
	case <-time.After(hostTestTimeout):
		t.Fatal("timed out waiting for the action spec")
		return ActionSpec{}
	}
}

// callTool 调用权限回调并断言它不返回 error：B2 约定所有拒绝都通过 Deny 结果表达，
// 回调本身只在 SDK 契约被破坏时才返回 error。
func callTool(
	t *testing.T,
	host *Host,
	toolName string,
	input map[string]any,
	permissionContext claudecode.ToolPermissionContext,
) claudecode.PermissionResult {
	t.Helper()
	result, err := host.canUseTool(context.Background(), toolName, input, permissionContext)
	if err != nil {
		t.Fatalf("canUseTool() error = %v", err)
	}
	return result
}

func requireAllow(t *testing.T, result claudecode.PermissionResult) claudecode.PermissionResultAllow {
	t.Helper()
	allow, ok := result.(claudecode.PermissionResultAllow)
	if !ok {
		t.Fatalf("canUseTool() = %T, want PermissionResultAllow", result)
	}
	if len(allow.UpdatedInput) == 0 {
		t.Fatal("allow must carry a non-empty UpdatedInput (CLI schema requirement)")
	}
	return allow
}

func requireDeny(t *testing.T, result claudecode.PermissionResult) claudecode.PermissionResultDeny {
	t.Helper()
	deny, ok := result.(claudecode.PermissionResultDeny)
	if !ok {
		t.Fatalf("canUseTool() = %T, want PermissionResultDeny", result)
	}
	if deny.Message == "" {
		t.Fatal("deny must carry a safe explanation")
	}
	return deny
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

	host := NewHost()
	newSession := claudecode.NewOptions(host.sessionSDKOptions(sessionModeNew, sessionID)...)
	if newSession.SessionID == nil || *newSession.SessionID != sessionID {
		t.Fatalf("new session SessionID = %v, want %q", newSession.SessionID, sessionID)
	}
	if newSession.Resume != nil {
		t.Fatalf("new session must not set Resume, got %q", *newSession.Resume)
	}
	if err := newSession.Validate(); err != nil {
		t.Fatalf("new session Validate() error = %v", err)
	}

	resumed := claudecode.NewOptions(host.sessionSDKOptions(sessionModeResume, sessionID)...)
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

func TestSessionSDKOptionsCarryWorkingDirectoryForNewAndResume(t *testing.T) {
	sessionID := "9c3d7e2a-1b4f-4c5d-8e6a-2f7b9d0c4a11"
	dir := t.TempDir()
	host := NewHost(WithCwd(dir))

	for _, test := range []struct {
		name string
		mode sessionMode
	}{
		{name: "new", mode: sessionModeNew},
		{name: "resume", mode: sessionModeResume},
	} {
		t.Run(test.name, func(t *testing.T) {
			options := claudecode.NewOptions(host.sessionSDKOptions(test.mode, sessionID)...)
			if options.Cwd == nil || *options.Cwd != dir {
				t.Fatalf("Cwd = %v, want %q", options.Cwd, dir)
			}
		})
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

func TestPermissionCallbackAllowsMCPToolWithoutAction(t *testing.T) {
	host := brokerHost(t)
	input := map[string]any{"path": "/tmp/report.md"}

	allow := requireAllow(t, callTool(t, host, "mcp__filesystem__read_file", input, claudecode.ToolPermissionContext{}))
	if !reflect.DeepEqual(map[string]any(allow.UpdatedInput), input) {
		t.Fatalf("UpdatedInput = %v, want the untouched original input", allow.UpdatedInput)
	}
	if len(allow.UpdatedPermissions) != 0 {
		t.Fatalf("UpdatedPermissions = %v, want none for an auto-allowed MCP tool", allow.UpdatedPermissions)
	}
	select {
	case request := <-host.Actions():
		t.Fatalf("MCP tool produced a pending action: %+v", request.Spec())
	default:
	}
}

func TestPermissionCallbackAllowOnceKeepsOriginalInput(t *testing.T) {
	host := brokerHost(t)
	suggestions := []claudecode.PermissionUpdate{{Type: "addRules"}}
	specs := resolveNextAction(host, ActionDecision{Kind: DecisionAllowOnce})

	input := map[string]any{"command": "ls -la /tmp"}
	allow := requireAllow(t, callTool(t, host, "Bash", input,
		claudecode.ToolPermissionContext{Suggestions: suggestions}))
	if len(allow.UpdatedPermissions) != 0 {
		t.Fatalf("UpdatedPermissions = %v, want none for allow_once", allow.UpdatedPermissions)
	}
	if !reflect.DeepEqual(map[string]any(allow.UpdatedInput), input) {
		t.Fatalf("UpdatedInput = %v, want the untouched original input", allow.UpdatedInput)
	}

	spec := awaitSpec(t, specs)
	if spec.Kind != ActionToolApproval || spec.ToolName != "Bash" {
		t.Fatalf("spec = %+v, want a Bash tool approval", spec)
	}
	if spec.Preview != "ls -la /tmp" {
		t.Fatalf("Preview = %q, want the whitelisted Bash preview", spec.Preview)
	}
	if !spec.AllowSessionAvailable {
		t.Fatal("AllowSessionAvailable must be true when the SDK offers suggestions")
	}
	if len(spec.Questions) != 0 {
		t.Fatalf("Questions = %v, want none for a tool approval", spec.Questions)
	}
}

func TestPermissionCallbackAllowSessionScopesSuggestions(t *testing.T) {
	host := brokerHost(t)
	suggestions := []claudecode.PermissionUpdate{{Type: "addRules"}, {Type: "setMode"}}
	specs := resolveNextAction(host, ActionDecision{Kind: DecisionAllowSession})

	allow := requireAllow(t, callTool(t, host, "Bash", map[string]any{"command": "ls"},
		claudecode.ToolPermissionContext{Suggestions: suggestions}))
	if len(allow.UpdatedPermissions) != len(suggestions) {
		t.Fatalf("UpdatedPermissions has %d entries, want the %d SDK suggestions", len(allow.UpdatedPermissions), len(suggestions))
	}
	for index, update := range allow.UpdatedPermissions {
		if update.Type != suggestions[index].Type {
			t.Fatalf("UpdatedPermissions[%d].Type = %q, want the SDK suggestion %q", index, update.Type, suggestions[index].Type)
		}
		if update.Destination == nil || *update.Destination != "session" {
			t.Fatalf("UpdatedPermissions[%d].Destination = %v, want session scope", index, update.Destination)
		}
	}
	for index, suggestion := range suggestions {
		if suggestion.Destination != nil {
			t.Fatalf("suggestions[%d] was mutated in place", index)
		}
	}
	awaitSpec(t, specs)
}

func TestPermissionCallbackAllowSessionWithoutSuggestionsDenies(t *testing.T) {
	host := brokerHost(t)
	specs := resolveNextAction(host, ActionDecision{Kind: DecisionAllowSession})

	// SDK 未给出 Suggestions 时不得自行拼接权限规则，该决议必须退化为拒绝。
	requireDeny(t, callTool(t, host, "Bash", map[string]any{"command": "ls"},
		claudecode.ToolPermissionContext{}))
	if spec := awaitSpec(t, specs); spec.AllowSessionAvailable {
		t.Fatal("AllowSessionAvailable must be false without SDK suggestions")
	}
}

func TestPermissionCallbackDenyCarriesReasonAndInterrupt(t *testing.T) {
	host := brokerHost(t)
	specs := resolveNextAction(host, ActionDecision{Kind: DecisionDeny, Reason: "denied by the user", Interrupt: true})

	deny := requireDeny(t, callTool(t, host, "Write", map[string]any{"file_path": "/etc/hosts"},
		claudecode.ToolPermissionContext{}))
	if deny.Message != "denied by the user" {
		t.Fatalf("deny message = %q, want the decision reason", deny.Message)
	}
	if !deny.Interrupt {
		t.Fatal("deny must forward the Interrupt flag used to unblock cancellation")
	}
	if spec := awaitSpec(t, specs); spec.Preview != "/etc/hosts" {
		t.Fatalf("Preview = %q, want the whitelisted Write preview", spec.Preview)
	}
}

func TestPermissionCallbackAskUserQuestionFlattensAnswers(t *testing.T) {
	host := brokerHost(t)
	input := map[string]any{"questions": []any{
		map[string]any{
			"header":   "Framework",
			"question": "Which framework?",
			"options": []any{
				map[string]any{"label": "React", "description": "web"},
				map[string]any{"label": "Vue"},
			},
		},
		map[string]any{
			"header":      "Targets",
			"question":    "Which targets?",
			"multiSelect": true,
			"options": []any{
				map[string]any{"label": "iOS"},
				map[string]any{"label": "Android"},
			},
		},
	}}
	specs := resolveNextAction(host, ActionDecision{
		Kind:    DecisionAnswer,
		Answers: [][]string{{"React"}, {"iOS", "Android"}},
	})

	allow := requireAllow(t, callTool(t, host, askUserQuestionToolName, input,
		claudecode.ToolPermissionContext{Suggestions: []claudecode.PermissionUpdate{{Type: "addRules"}}}))
	if len(allow.UpdatedPermissions) != 0 {
		t.Fatalf("UpdatedPermissions = %v, want none for an answered question", allow.UpdatedPermissions)
	}
	if _, kept := allow.UpdatedInput["questions"]; !kept {
		t.Fatal("UpdatedInput must keep the original input keys")
	}
	wantAnswers := map[string]any{
		"Which framework?": "React",
		"Which targets?":   "iOS, Android",
	}
	if !reflect.DeepEqual(allow.UpdatedInput["answers"], wantAnswers) {
		t.Fatalf("UpdatedInput[answers] = %v, want %v", allow.UpdatedInput["answers"], wantAnswers)
	}

	spec := awaitSpec(t, specs)
	if spec.Kind != ActionAskUserQuestion || spec.ToolName != askUserQuestionToolName {
		t.Fatalf("spec = %+v, want an ask_user_question action", spec)
	}
	if spec.Preview != "" {
		t.Fatalf("Preview = %q, want no preview for a question", spec.Preview)
	}
	if spec.AllowSessionAvailable {
		t.Fatal("AllowSessionAvailable is meaningless for a question and must stay false")
	}
	wantQuestions := []Question{
		{
			Header:    "Framework",
			Question:  "Which framework?",
			InputType: QuestionChoice,
			Options:   []QuestionOption{{Label: "React", Description: "web"}, {Label: "Vue"}},
		},
		{
			Header:      "Targets",
			Question:    "Which targets?",
			InputType:   QuestionChoice,
			MultiSelect: true,
			Options:     []QuestionOption{{Label: "iOS"}, {Label: "Android"}},
		},
	}
	if !reflect.DeepEqual(spec.Questions, wantQuestions) {
		t.Fatalf("Questions = %+v, want %+v", spec.Questions, wantQuestions)
	}
}

func TestPermissionCallbackAskUserQuestionWithoutOptionsIsFreeText(t *testing.T) {
	host := brokerHost(t)
	input := map[string]any{"questions": []any{
		map[string]any{"header": "Name", "question": "What should the release be called?"},
	}}
	specs := resolveNextAction(host, ActionDecision{Kind: DecisionAnswer, Answers: [][]string{{"Aurora"}}})

	allow := requireAllow(t, callTool(t, host, askUserQuestionToolName, input,
		claudecode.ToolPermissionContext{}))
	wantAnswers := map[string]any{"What should the release be called?": "Aurora"}
	if !reflect.DeepEqual(allow.UpdatedInput["answers"], wantAnswers) {
		t.Fatalf("UpdatedInput[answers] = %v, want %v", allow.UpdatedInput["answers"], wantAnswers)
	}
	spec := awaitSpec(t, specs)
	if len(spec.Questions) != 1 || spec.Questions[0].InputType != QuestionText {
		t.Fatalf("Questions = %+v, want a single free-text question", spec.Questions)
	}
}

func TestPermissionCallbackAnswerKeyFallsBackToPosition(t *testing.T) {
	host := brokerHost(t)
	input := map[string]any{"questions": []any{
		map[string]any{"header": "Only a header", "options": []any{map[string]any{"label": "yes"}}},
	}}
	specs := resolveNextAction(host, ActionDecision{Kind: DecisionAnswer, Answers: [][]string{{"yes"}}})

	allow := requireAllow(t, callTool(t, host, askUserQuestionToolName, input,
		claudecode.ToolPermissionContext{}))
	wantAnswers := map[string]any{"Only a header": "yes"}
	if !reflect.DeepEqual(allow.UpdatedInput["answers"], wantAnswers) {
		t.Fatalf("UpdatedInput[answers] = %v, want the header as key", allow.UpdatedInput["answers"])
	}
	awaitSpec(t, specs)
}

func TestPermissionCallbackRejectsMalformedQuestions(t *testing.T) {
	host := brokerHost(t)
	input := map[string]any{"questions": []any{map[string]any{"options": []any{map[string]any{"label": "yes"}}}}}
	specs := resolveNextAction(host, ActionDecision{Kind: DecisionAnswer, Answers: [][]string{{"yes"}}})

	// 缺少 header 和 question 的提问无法安全展示，Adapter 只上报空问题列表，
	// 由 Controller 按 unsupported 处理。
	requireAllow(t, callTool(t, host, askUserQuestionToolName, input,
		claudecode.ToolPermissionContext{}))
	if spec := awaitSpec(t, specs); len(spec.Questions) != 0 {
		t.Fatalf("Questions = %+v, want none for a malformed question", spec.Questions)
	}
}

func TestPermissionCallbackTimesOutWithoutInterrupt(t *testing.T) {
	client := &fakeClient{}
	host := connectHost(t, client, WithActionTimeout(20*time.Millisecond))

	// 没有任何消费者读取旁路通道：上行 select 必须被超时分支解开。
	deny := requireDeny(t, callTool(t, host, "Bash", map[string]any{"command": "ls"},
		claudecode.ToolPermissionContext{}))
	if deny.Interrupt {
		t.Fatal("action timeout must use Deny{Interrupt:false} so the Turn continues")
	}
	if client.interruptCalls() != 0 {
		t.Fatalf("Interrupt called %d times, want 0 while a callback is blocked", client.interruptCalls())
	}
}

func TestPermissionCallbackUnblocksOnClose(t *testing.T) {
	host := brokerHost(t, WithActionTimeout(time.Minute))
	results := make(chan claudecode.PermissionResult, 1)
	go func() {
		result, _ := host.canUseTool(context.Background(), "Bash", map[string]any{"command": "ls"},
			claudecode.ToolPermissionContext{})
		results <- result
	}()

	select {
	case <-host.Actions():
	case <-time.After(hostTestTimeout):
		t.Fatal("timed out waiting for the action request")
	}
	if err := host.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	select {
	case result := <-results:
		requireDeny(t, result)
	case <-time.After(hostTestTimeout):
		t.Fatal("Close did not unblock the pending permission callback")
	}
}

func TestPermissionCallbackConcurrentRequestsStayIndependent(t *testing.T) {
	host := brokerHost(t, WithActionTimeout(time.Minute))
	type outcome struct {
		tool   string
		result claudecode.PermissionResult
	}
	outcomes := make(chan outcome, 2)
	for _, tool := range []string{"Bash", "Write"} {
		go func(tool string) {
			result, _ := host.canUseTool(context.Background(), tool, map[string]any{"command": tool, "file_path": tool},
				claudecode.ToolPermissionContext{})
			outcomes <- outcome{tool: tool, result: result}
		}(tool)
	}

	// 两个并发回调各自独立等待：Adapter 不做并发判定，也不互相覆盖。
	decisions := map[string]ActionDecision{
		"Bash":  {Kind: DecisionAllowOnce},
		"Write": {Kind: DecisionDeny, Reason: "one action at a time"},
	}
	for range decisions {
		select {
		case request := <-host.Actions():
			spec := request.Spec()
			if !request.Resolve(decisions[spec.ToolName]) {
				t.Fatalf("Resolve(%s) refused the first decision", spec.ToolName)
			}
		case <-time.After(hostTestTimeout):
			t.Fatal("timed out waiting for concurrent action requests")
		}
	}
	for range decisions {
		select {
		case got := <-outcomes:
			switch got.tool {
			case "Bash":
				requireAllow(t, got.result)
			default:
				requireDeny(t, got.result)
			}
		case <-time.After(hostTestTimeout):
			t.Fatal("timed out waiting for concurrent callback results")
		}
	}
}

func TestActionRequestResolvesOnlyOnce(t *testing.T) {
	request := NewActionRequest(ActionSpec{Kind: ActionToolApproval, ToolName: "Bash"})
	if !request.Resolve(ActionDecision{Kind: DecisionAllowOnce}) {
		t.Fatal("Resolve() refused the first decision")
	}
	if request.Resolve(ActionDecision{Kind: DecisionDeny, Reason: "late"}) {
		t.Fatal("Resolve() accepted a second decision")
	}
	select {
	case decision := <-request.Decision():
		if decision.Kind != DecisionAllowOnce {
			t.Fatalf("decision = %+v, want the first one", decision)
		}
	default:
		t.Fatal("Decision() carried no decision")
	}
	select {
	case decision := <-request.Decision():
		t.Fatalf("Decision() delivered twice: %+v", decision)
	default:
	}
}

func TestActionRequestSpecIsACopy(t *testing.T) {
	request := NewActionRequest(ActionSpec{
		Kind:      ActionAskUserQuestion,
		ToolName:  askUserQuestionToolName,
		Questions: []Question{{Header: "H", Question: "Q", InputType: QuestionChoice, Options: []QuestionOption{{Label: "yes"}}}},
	})
	spec := request.Spec()
	spec.Questions[0].Options[0].Label = "mutated"
	if request.Spec().Questions[0].Options[0].Label != "yes" {
		t.Fatal("Spec() must hand out a deep copy")
	}
}

func TestHostInterruptDelegatesToClient(t *testing.T) {
	client := &fakeClient{}
	host := connectHost(t, client)
	if err := host.Interrupt(context.Background()); err != nil {
		t.Fatalf("Interrupt() error = %v", err)
	}
	if client.interruptCalls() != 1 {
		t.Fatalf("Interrupt calls = %d, want 1", client.interruptCalls())
	}
	if err := host.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := host.Interrupt(context.Background()); err == nil {
		t.Fatal("Interrupt() must fail after Close")
	}
	if client.interruptCalls() != 1 {
		t.Fatalf("Interrupt calls = %d, want the closed Host to skip the SDK", client.interruptCalls())
	}
}

func TestHostRunTurnEmitsStructuredBlocks(t *testing.T) {
	iterator := &fakeIterator{steps: []iteratorStep{
		messageStep(&claudecode.SystemMessage{Subtype: "init", Data: map[string]any{"session_id": "session-1"}}),
		messageStep(&claudecode.AssistantMessage{Content: []claudecode.ContentBlock{
			&claudecode.ThinkingBlock{Thinking: "weighing options"},
			&claudecode.TextBlock{Text: "hello"},
			&claudecode.ToolUseBlock{ToolUseID: "toolu_1", Name: "Read", Input: map[string]any{"file_path": "/tmp/a"}},
		}}),
		messageStep(&claudecode.SystemMessage{Subtype: "compact_boundary"}),
		messageStep(&claudecode.ResultMessage{SessionID: "session-1"}),
	}}
	host := connectHost(t, &fakeClient{iterators: []claudecode.MessageIterator{iterator}})

	events, emit := collectEvents()
	if err := host.RunTurn(context.Background(), "prompt", emit); err != nil {
		t.Fatalf("RunTurn() error = %v", err)
	}

	var kinds []EventKind
	for _, event := range *events {
		kinds = append(kinds, event.Kind)
	}
	// 非 init 的 SystemMessage（compact_boundary）被静默忽略，不产生事件。
	wantKinds := []EventKind{
		EventSessionStarted,
		EventContentBlock,
		EventText,
		EventContentBlock,
		EventTurnCompleted,
	}
	if !reflect.DeepEqual(kinds, wantKinds) {
		t.Fatalf("event kinds = %v, want %v", kinds, wantKinds)
	}
	thinking := (*events)[1]
	if thinking.Block == nil || thinking.Block.Kind != BlockThinking || thinking.Block.Thinking != "weighing options" {
		t.Fatalf("thinking event = %+v, want the mapped thinking block", thinking)
	}
	toolUse := (*events)[3]
	if toolUse.Block == nil || toolUse.Block.Kind != BlockToolUse || toolUse.Block.ToolName != "Read" ||
		toolUse.Block.ToolClass != ToolBuiltin || toolUse.Block.ToolUseID != "toolu_1" {
		t.Fatalf("tool_use event = %+v, want the mapped tool_use block", toolUse)
	}
	for _, event := range *events {
		if event.SessionID != "session-1" {
			t.Fatalf("event %q carries session %q, want session-1", event.Kind, event.SessionID)
		}
	}
}
