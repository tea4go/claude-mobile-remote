// Package claude 封装 claude-agent-sdk-go 的最小托管生命周期。
package claude

import (
	"context"
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
	client      sdkClient
	connected   bool
	closed      bool
	sessionID   string
	startedSent bool
}

// NewHost 创建使用 SDK 默认权限模式的 Host。A2 对进入回调的工具请求统一拒绝。
func NewHost(options ...Option) *Host {
	host := &Host{client: claudecode.NewClient(claudecode.WithCanUseTool(denyToolUse))}
	for _, option := range options {
		option(host)
	}
	return host
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
func (h *Host) SessionID() string { return h.sessionID }

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
