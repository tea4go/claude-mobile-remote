// Package claude 封装 claude-agent-sdk-go 的最小托管生命周期。
package claude

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"

	claudecode "github.com/tea4go/claude-agent-sdk-go"
)

type sdkClient interface {
	Connect(context.Context, ...claudecode.StreamMessage) error
	Query(context.Context, string) error
	ReceiveResponse(context.Context) claudecode.MessageIterator
	Disconnect() error
}

// Option 配置 Host。A2 只使用默认生产 Client；测试在包内注入 fake。
type Option func(*Host)

func withClient(client sdkClient) Option {
	return func(host *Host) { host.client = client }
}

func denyToolUse(
	context.Context,
	string,
	map[string]any,
	claudecode.ToolPermissionContext,
) (claudecode.PermissionResult, error) {
	return claudecode.NewPermissionResultDeny("A2 尚未实现审批"), nil
}

// Host 持有一个长生命周期 SDK Client。RunTurn 必须由上层串行调用。
type Host struct {
	client       sdkClient
	connected    bool
	closed       bool
	sessionID    string
	sessionIDSet bool
	startedSent  bool
}

// NewHost 创建使用 SDK 默认权限模式的 Host。A2 对进入回调的工具请求统一拒绝。
func NewHost(options ...Option) *Host {
	host := &Host{client: claudecode.NewClient(claudecode.WithCanUseTool(denyToolUse))}
	for _, option := range options {
		option(host)
	}
	return host
}

// sessionMode 区分预分配 ID 的两种来源（A5.2 设计 §4）：新建会话把 Service
// 生成的 UUID 指定给 Claude，恢复会话沿用已有 ID。两种模式的 SDK 选项互斥。
type sessionMode int

const (
	sessionModeNew sessionMode = iota
	sessionModeResume
)

// sessionSDKOptions 按模式构造 SDK 选项：新建只映射 --session-id，恢复只
// 映射 --resume（设计 §5）。SDK 侧 Validate 会拒绝两者并用。
func sessionSDKOptions(mode sessionMode, sessionID string) []claudecode.Option {
	options := []claudecode.Option{claudecode.WithCanUseTool(denyToolUse)}
	if mode == sessionModeResume {
		return append(options, claudecode.WithResume(sessionID))
	}
	return append(options, claudecode.WithSessionID(sessionID))
}

// NewHostWithReservedSessionID 创建以既有 UUID 预分配新 Claude session 的
// Host（A5.2）。ID 通过 SDK WithSessionID 映射到 Claude CLI --session-id；
// Host 从 Connect 前持有预期 ID，首个 init/Result 必须返回相同 ID。
func NewHostWithReservedSessionID(sessionID string, options ...Option) (*Host, error) {
	return newReservedHost(sessionModeNew, sessionID, options...)
}

// NewHostForResume 创建恢复已有会话的 Host（A5.2 设计 §5.2）。已有 ID 通过
// SDK WithResume 映射到 Claude CLI --resume，不传 --session-id；Claude 返回
// 其他 ID 时按 Session 级失败处理。
func NewHostForResume(sessionID string, options ...Option) (*Host, error) {
	return newReservedHost(sessionModeResume, sessionID, options...)
}

func newReservedHost(mode sessionMode, sessionID string, options ...Option) (*Host, error) {
	if !isValidUUID(sessionID) {
		return nil, fmt.Errorf("预分配 session ID 必须是合法 UUID: %q", sessionID)
	}
	host := NewHost(append([]Option{withReservedSessionID(mode, sessionID)}, options...)...)
	return host, nil
}

func withReservedSessionID(mode sessionMode, sessionID string) Option {
	return func(host *Host) {
		host.sessionID = sessionID
		host.sessionIDSet = true
		host.client = claudecode.NewClient(sessionSDKOptions(mode, sessionID)...)
	}
}

// PrepareNewSession 为新会话生成 UUID v4 并预分配（A5.2 设计 §3.1）。
// 只能调用一次；Connect 前调用。
func (h *Host) PrepareNewSession() error {
	if h.sessionIDSet || h.connected || h.closed {
		return wrapError(ScopeSession, "预分配 session ID", errors.New("session ID 已确定或 Host 已启动"))
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return wrapError(ScopeSession, "生成 session UUID", err)
	}
	raw[6] = (raw[6] & 0x0f) | 0x40 // version 4
	raw[8] = (raw[8] & 0x3f) | 0x80 // variant 10
	sessionID := fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16])
	withReservedSessionID(sessionModeNew, sessionID)(h)
	return nil
}

func isValidUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, char := range value {
		switch i {
		case 8, 13, 18, 23:
			if char != '-' {
				return false
			}
		default:
			isHex := (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')
			if !isHex {
				return false
			}
		}
	}
	return true
}

// Connect 启动并连接由 SDK 托管的 Claude CLI。
func (h *Host) Connect(ctx context.Context) error {
	if h.connected || h.closed {
		return wrapError(ScopeSession, "连接 Claude CLI", errors.New("Host 已连接或已关闭"))
	}
	if err := h.client.Connect(ctx); err != nil {
		return wrapError(ScopeSession, "连接 Claude CLI", err)
	}
	h.connected = true
	return nil
}

// SessionID 返回 SDK 提供的真实 session ID；首轮 init/Result 前为空。
// A5.2 预分配后从 Connect 前即返回预留 ID。
func (h *Host) SessionID() string { return h.sessionID }

// SessionConfirmed 报告 Claude init/Result 是否已确认预留 ID（A5.2 §4）。
// 未预留 ID 的 Host 在首个非空 init/Result 接受后同样视为已确认。
func (h *Host) SessionConfirmed() bool { return h.startedSent }

// RunTurn 在当前 Client 上执行一个文本 Turn，并同步发出 A2 内部事件。
func (h *Host) RunTurn(ctx context.Context, prompt string, emit func(Event)) error {
	if !h.connected || h.closed {
		return wrapError(ScopeSession, "执行 Claude Turn", errors.New("Host 未连接"))
	}
	if strings.TrimSpace(prompt) == "" {
		return wrapError(ScopeTurn, "执行 Claude Turn", errors.New("prompt 不能为空"))
	}
	if emit == nil {
		emit = func(Event) {}
	}
	if err := h.client.Query(ctx, prompt); err != nil {
		return wrapError(ScopeSession, "发送 Claude Query", err)
	}

	iterator := h.client.ReceiveResponse(ctx)
	defer iterator.Close()
	emittedText := false
	for {
		message, err := iterator.Next(ctx)
		if err != nil {
			if errors.Is(err, claudecode.ErrNoMoreMessages) {
				return wrapError(ScopeSession, "接收 Claude 响应", errors.New("流结束但未收到 ResultMessage"))
			}
			return wrapError(ScopeSession, "接收 Claude 响应", err)
		}

		parsed := parseMessage(message)
		if parsed.result != nil && parsed.sessionID == "" {
			return wrapError(ScopeSession, "完成 Claude Turn", errors.New("ResultMessage 未提供 session ID"))
		}
		if parsed.assistantErr != nil {
			// 带 Error 的 AssistantMessage 终止当前流（SDK 语义，与 WhaleTerm
			// 验证行为一致）：server/rate_limit 类瞬时错误只结束本 Turn，
			// 其余（认证、计费等）视为 Session 级不可恢复。
			scope := ScopeTurn
			if !isTransientAssistantError(parsed.assistantErr.errorType) {
				scope = ScopeSession
			}
			message := parsed.assistantErr.text
			if message == "" {
				message = "Claude API Error: " + parsed.assistantErr.errorType
			} else {
				message = fmt.Sprintf("Claude API Error (%s): %s", parsed.assistantErr.errorType, message)
			}
			sessionID := h.sessionID
			if sessionID == "" {
				sessionID = parsed.sessionID
			}
			emit(Event{Kind: EventTurnFailed, SessionID: sessionID, Text: message})
			return wrapError(scope, "Claude Turn 失败", errors.New(message))
		}
		if parsed.sessionID != "" {
			if err := h.acceptSessionID(parsed.sessionID, parsed.source, emit); err != nil {
				return err
			}
		}
		for _, text := range parsed.text {
			emittedText = true
			emit(Event{Kind: EventText, SessionID: h.sessionID, Text: text})
		}
		if parsed.result == nil {
			continue
		}
		if h.sessionID == "" {
			return wrapError(ScopeSession, "完成 Claude Turn", errors.New("ResultMessage 未提供 session ID"))
		}
		if parsed.result.isError {
			details := make([]string, 0, 1+len(parsed.result.errors))
			if parsed.result.text != "" {
				details = append(details, parsed.result.text)
			}
			details = append(details, parsed.result.errors...)
			if len(details) == 0 {
				details = append(details, "Claude 返回未知错误")
			}
			message := strings.Join(details, "; ")
			emit(Event{Kind: EventTurnFailed, SessionID: h.sessionID, Text: message})
			return wrapError(ScopeTurn, "Claude Turn 失败", errors.New(message))
		}
		if !emittedText && parsed.result.text != "" {
			emit(Event{Kind: EventText, SessionID: h.sessionID, Text: parsed.result.text})
		}
		emit(Event{Kind: EventTurnCompleted, SessionID: h.sessionID})
		return nil
	}
}

// isTransientAssistantError 判断 AssistantMessage 错误类型是否只影响当前 Turn。
// server_error 与 rate_limit 通常可重试；认证、计费、无效请求等无法通过
// 重试恢复，保守视为 Session 级失败。类型集合来源：WhaleTerm 验证行为。
func isTransientAssistantError(errorType string) bool {
	return errorType == string(claudecode.AssistantMessageErrorServer) ||
		errorType == string(claudecode.AssistantMessageErrorRateLimit)
}

func (h *Host) acceptSessionID(value string, source SessionIDSource, emit func(Event)) error {
	if h.sessionID != "" && h.sessionID != value {
		return wrapError(ScopeSession, "校验 Claude session ID", fmt.Errorf("session ID 从 %q 变为 %q", h.sessionID, value))
	}
	if h.sessionID == "" {
		h.sessionID = value
	}
	if !h.startedSent {
		h.startedSent = true
		emit(Event{Kind: EventSessionStarted, SessionID: value, Source: source})
	}
	return nil
}

// Close 幂等关闭 SDK Client。失败不会导致重复 Disconnect。
func (h *Host) Close() error {
	if h.closed {
		return nil
	}
	h.closed = true
	if !h.connected {
		return nil
	}
	h.connected = false
	if err := h.client.Disconnect(); err != nil {
		return fmt.Errorf("关闭 Claude CLI: %w", err)
	}
	return nil
}
