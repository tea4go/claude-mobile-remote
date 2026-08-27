//go:build integration

package claude

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	claudecode "github.com/tea4go/claude-agent-sdk-go"
)

// liveCodeWord 生成一次性虚构口令，用于验证跨 Turn 上下文而不写入真实业务内容。
func liveCodeWord(t *testing.T) string {
	t.Helper()
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		t.Fatalf("generate code word: %v", err)
	}
	return hex.EncodeToString(randomBytes)
}

// liveTranscriptBusinessLines 统计指定 session 的 Claude transcript 中的业务消息
// 行数（user/assistant）。transcript 不存在时返回 0，用于验证 Connect 与
// Query 前不产生业务对话内容（A5.2 设计 §12.2）。
func liveTranscriptBusinessLines(t *testing.T, sessionID string) int {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(home, ".claude", "projects", "*", sessionID+".jsonl"))
	if err != nil {
		t.Fatalf("glob transcript: %v", err)
	}
	business := 0
	for _, match := range matches {
		content, err := os.ReadFile(match)
		if err != nil {
			t.Fatalf("read transcript: %v", err)
		}
		for _, line := range strings.Split(string(content), "\n") {
			if strings.Contains(line, `"type":"user"`) || strings.Contains(line, `"type":"assistant"`) {
				business++
			}
		}
	}
	return business
}

func requireLive(t *testing.T) {
	t.Helper()
	if os.Getenv("CLAUDE_MOBILE_REMOTE_LIVE") != "1" {
		t.Skip("set CLAUDE_MOBILE_REMOTE_LIVE=1 to run the live Claude test")
	}
}

func TestLiveTwoTurnSession(t *testing.T) {
	requireLive(t)
	codeWord := liveCodeWord(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	host := NewHost()
	if err := host.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer func() {
		if err := host.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	}()

	firstSource := SessionIDSource("")
	firstStartedBeforeResult := false
	firstCompleted := false
	firstPrompt := "Do not use any tools. Remember this code word for the next turn and reply with only the code word: " + codeWord
	if err := host.RunTurn(ctx, firstPrompt, func(event Event) {
		switch event.Kind {
		case EventSessionStarted:
			firstSource = event.Source
			firstStartedBeforeResult = !firstCompleted
		case EventTurnCompleted:
			firstCompleted = true
		}
	}); err != nil {
		t.Fatalf("first RunTurn() error = %v", err)
	}
	if firstSource != SessionIDFromInit {
		t.Fatalf("session source = %q, want init", firstSource)
	}
	if !firstStartedBeforeResult || !firstCompleted {
		t.Fatal("session_started must arrive before the first turn_completed event")
	}
	firstSessionID := host.SessionID()
	if firstSessionID == "" {
		t.Fatal("first turn session ID is empty")
	}

	var secondText strings.Builder
	secondPrompt := "Do not use any tools. Reply with only the code word I asked you to remember in the previous turn."
	if err := host.RunTurn(ctx, secondPrompt, func(event Event) {
		if event.Kind == EventText {
			secondText.WriteString(event.Text)
		}
	}); err != nil {
		t.Fatalf("second RunTurn() error = %v", err)
	}
	if host.SessionID() != firstSessionID {
		t.Fatal("session ID changed across two turns")
	}
	if !strings.Contains(secondText.String(), codeWord) {
		t.Fatal("second turn did not preserve first-turn context")
	}

	t.Log("session_id_non_empty=true session_id_consistent=true turns=2")
}

// TestLiveReservedSessionIDPreTurn 是 A5.2 的真实 SDK 门禁（设计 §12.2）：
// 预分配 UUID 经 --session-id 交给 Claude，Connect 不产生业务 Turn，首条
// 真实 Query 的 init/Result 返回同一 ID，第二轮沿用该 ID 与上下文。
func TestLiveReservedSessionIDPreTurn(t *testing.T) {
	requireLive(t)
	codeWord := liveCodeWord(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	host := NewHost()
	if err := host.PrepareNewSession(); err != nil {
		t.Fatalf("PrepareNewSession() error = %v", err)
	}
	reservedID := host.SessionID()
	if reservedID == "" {
		t.Fatal("PrepareNewSession() did not reserve a session ID")
	}
	if host.SessionConfirmed() {
		t.Fatal("reserved session must not be confirmed before Connect")
	}

	if err := host.Connect(ctx); err != nil {
		t.Fatalf("Connect(--session-id %s) error = %v", reservedID, err)
	}
	closed := false
	defer func() {
		if closed {
			return
		}
		if err := host.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	}()

	// Connect 后预留 ID 可读但未确认，且不得凭空产生业务对话内容。
	if host.SessionID() != reservedID {
		t.Fatalf("SessionID() after Connect = %q, want %q", host.SessionID(), reservedID)
	}
	if host.SessionConfirmed() {
		t.Fatal("session must not be confirmed before the first query")
	}
	if business := liveTranscriptBusinessLines(t, reservedID); business != 0 {
		t.Fatalf("transcript business lines before first query = %d, want 0", business)
	}

	var confirmedID string
	var confirmedSource SessionIDSource
	firstPrompt := "Do not use any tools. Remember this code word for the next turn and reply with only the code word: " + codeWord
	firstErr := host.RunTurn(ctx, firstPrompt, func(event Event) {
		if event.Kind == EventSessionStarted {
			confirmedID = event.SessionID
			confirmedSource = event.Source
		}
	})
	// 无论 Turn 是否完成，init 已确认的 ID 都必须等于预留 ID。
	if confirmedID != "" && confirmedID != reservedID {
		t.Fatalf("Claude confirmed session ID = %q, want reserved %q", confirmedID, reservedID)
	}
	if firstErr != nil {
		if confirmedID == reservedID {
			// 账号额度/限流等环境原因阻断首轮时，已取得“--session-id 被接受且
			// init 返回预留 ID”的证据；剩余上下文断言必须在恢复后重跑，
			// 不能把跳过当作真实门禁通过。
			t.Skipf("reserved ID confirmed by init but the first turn did not complete: %v", firstErr)
		}
		t.Fatalf("first RunTurn() error = %v", firstErr)
	}
	if confirmedID != reservedID {
		t.Fatalf("Claude confirmed session ID = %q, want reserved %q", confirmedID, reservedID)
	}
	if confirmedSource != SessionIDFromInit {
		t.Fatalf("session source = %q, want init", confirmedSource)
	}
	if !host.SessionConfirmed() {
		t.Fatal("session must be confirmed after the first turn")
	}

	var secondText strings.Builder
	secondPrompt := "Do not use any tools. Reply with only the code word I asked you to remember in the previous turn."
	if err := host.RunTurn(ctx, secondPrompt, func(event Event) {
		if event.Kind == EventText {
			secondText.WriteString(event.Text)
		}
	}); err != nil {
		t.Fatalf("second RunTurn() error = %v", err)
	}
	if host.SessionID() != reservedID {
		t.Fatalf("session ID changed across turns: %q", host.SessionID())
	}
	if !strings.Contains(secondText.String(), codeWord) {
		t.Fatal("second turn did not preserve first-turn context")
	}

	if err := host.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	closed = true

	// Resume 必须返回请求的同一 ID，且在首轮前即可读取（设计 §5.2）。
	resumeHost, err := NewHostForResume(reservedID)
	if err != nil {
		t.Fatalf("NewHostForResume() error = %v", err)
	}
	if err := resumeHost.Connect(ctx); err != nil {
		t.Fatalf("Connect(--resume %s) error = %v", reservedID, err)
	}
	defer func() {
		if err := resumeHost.Close(); err != nil {
			t.Errorf("resume Close() error = %v", err)
		}
	}()
	if resumeHost.SessionID() != reservedID || resumeHost.SessionConfirmed() {
		t.Fatalf("resume host pre-turn state = (%q, confirmed=%v)", resumeHost.SessionID(), resumeHost.SessionConfirmed())
	}
	var resumeText strings.Builder
	if err := resumeHost.RunTurn(ctx, secondPrompt, func(event Event) {
		if event.Kind == EventText {
			resumeText.WriteString(event.Text)
		}
	}); err != nil {
		t.Fatalf("resume RunTurn() error = %v", err)
	}
	if resumeHost.SessionID() != reservedID {
		t.Fatalf("resume session ID = %q, want %q", resumeHost.SessionID(), reservedID)
	}
	if !strings.Contains(resumeText.String(), codeWord) {
		t.Fatal("resume turn did not preserve the original session context")
	}

	t.Log("reserved_id_confirmed=true pre_turn_transcript_clean=true turns=2 resume_same_id=true")
}

// TestLiveReservedSessionIDNoQueryLeavesNoTranscript 验证设计 §11：首条 Query
// 前退出不产生业务 Turn，未使用的预留 UUID 自然作废。
func TestLiveReservedSessionIDNoQueryLeavesNoTranscript(t *testing.T) {
	requireLive(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	host := NewHost()
	if err := host.PrepareNewSession(); err != nil {
		t.Fatalf("PrepareNewSession() error = %v", err)
	}
	reservedID := host.SessionID()
	if err := host.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if err := host.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if business := liveTranscriptBusinessLines(t, reservedID); business != 0 {
		t.Fatalf("transcript business lines after query-less session = %d, want 0", business)
	}

	t.Log("connect_close_without_query_transcript_clean=true")
}

// askUserAnswerKey 按 B2 设计 §3.1 固定规则推导 answers map 的 key：
// 依次取 question、header、question_<序号>（序号从 1 开始）。
func askUserAnswerKey(question map[string]any, index int) string {
	if text, ok := question["question"].(string); ok && text != "" {
		return text
	}
	if header, ok := question["header"].(string); ok && header != "" {
		return header
	}
	return fmt.Sprintf("question_%d", index+1)
}

// liveToolResultText 提取 ToolResultBlock 的正文，仅用于本地实证观察 CLI 是否
// 报 schema 错误；正文不进入协议、日志或任何持久化路径。
func liveToolResultText(content any) string {
	switch value := content.(type) {
	case string:
		return value
	case []any:
		var builder strings.Builder
		for _, item := range value {
			if entry, ok := item.(map[string]any); ok {
				if text, ok := entry["text"].(string); ok {
					builder.WriteString(text)
				}
			}
		}
		return builder.String()
	default:
		return fmt.Sprintf("%v", content)
	}
}

// TestLiveAskUserQuestionFreeText 是 B2 Task 2 Step 1 的自由文本实证（设计
// §3.1「自由文本待实证」）：真实 CLI 触发一次 AskUserQuestion 后，在
// UpdatedInput["answers"] 中回传一个不属于任何 options[].label 的字符串，观察
// CLI 是否接受、是否报 schema 错误。本测试只输出观察结论，不改变默认配置：
// 是否启用 inputType=text 由 ServiceConfig.AskUserTextEnabled 显式控制。
func TestLiveAskUserQuestionFreeText(t *testing.T) {
	requireLive(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 自由文本刻意使用不可能出现在 options 中的一次性口令，便于判定 CLI 是否
	// 真的把该字符串当作答案传给了模型。
	freeText := "freetext-" + liveCodeWord(t)

	var (
		mu          sync.Mutex
		askedTools  []string
		askedKey    string
		askedLabels []string
	)
	callback := func(
		_ context.Context,
		toolName string,
		input map[string]any,
		_ claudecode.ToolPermissionContext,
	) (claudecode.PermissionResult, error) {
		mu.Lock()
		askedTools = append(askedTools, toolName)
		mu.Unlock()
		if toolName != "AskUserQuestion" {
			return claudecode.NewPermissionResultDeny("本实证只允许 AskUserQuestion"), nil
		}

		questions, _ := input["questions"].([]any)
		if len(questions) == 0 {
			return claudecode.NewPermissionResultDeny("AskUserQuestion 未提供 questions"), nil
		}
		first, _ := questions[0].(map[string]any)
		key := askUserAnswerKey(first, 0)
		var labels []string
		if options, ok := first["options"].([]any); ok {
			for _, option := range options {
				if entry, ok := option.(map[string]any); ok {
					if label, ok := entry["label"].(string); ok {
						labels = append(labels, label)
					}
				}
			}
		}
		mu.Lock()
		askedKey = key
		askedLabels = labels
		mu.Unlock()

		// 按设计 §3.1 的形状回传：原始 input 加上 answers 键；答案刻意不取
		// 任何 option.label。
		updated := make(map[string]any, len(input)+1)
		for name, value := range input {
			updated[name] = value
		}
		updated["answers"] = map[string]any{key: freeText}
		allow := claudecode.NewPermissionResultAllow()
		allow.UpdatedInput = updated
		return allow, nil
	}

	client := claudecode.NewClient(claudecode.WithCanUseTool(callback))
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer func() {
		if err := client.Disconnect(); err != nil {
			t.Errorf("Disconnect() error = %v", err)
		}
	}()

	prompt := "Use the AskUserQuestion tool exactly once to ask me which color I prefer, " +
		"offering exactly three options labelled Red, Green and Blue. Do not use any other tool. " +
		"After I answer, reply with only the answer text you received, verbatim."
	if err := client.Query(ctx, prompt); err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	iterator := client.ReceiveResponse(ctx)
	defer iterator.Close()

	toolNames := make(map[string]string)
	resultIsError := ""
	resultText := ""
	var assistantText strings.Builder
	// 与 Host.RunTurn 一致：收到 ResultMessage 即结束本 Turn 的读取，
	// 迭代器在 Result 之后不会返回 ErrNoMoreMessages 而是继续阻塞。
	for done := false; !done; {
		message, err := iterator.Next(ctx)
		if err != nil {
			if errors.Is(err, claudecode.ErrNoMoreMessages) {
				break
			}
			t.Fatalf("iterator.Next() error = %v", err)
		}
		switch msg := message.(type) {
		case *claudecode.AssistantMessage:
			if msg.ParentToolUseID != nil {
				continue
			}
			for _, block := range msg.Content {
				switch typed := block.(type) {
				case *claudecode.TextBlock:
					assistantText.WriteString(typed.Text)
				case *claudecode.ToolUseBlock:
					toolNames[typed.ToolUseID] = typed.Name
				}
			}
		case *claudecode.UserMessage:
			blocks, ok := msg.Content.([]claudecode.ContentBlock)
			if !ok {
				continue
			}
			for _, block := range blocks {
				typed, ok := block.(*claudecode.ToolResultBlock)
				if !ok || toolNames[typed.ToolUseID] != "AskUserQuestion" {
					continue
				}
				resultIsError = "absent"
				if typed.IsError != nil {
					resultIsError = fmt.Sprintf("%t", *typed.IsError)
				}
				resultText = liveToolResultText(typed.Content)
			}
		case *claudecode.ResultMessage:
			if msg.IsError {
				t.Logf("free_text_probe result_message_is_error=true errors=%v", msg.Errors)
			}
			done = true
		}
	}

	mu.Lock()
	tools := append([]string(nil), askedTools...)
	key := askedKey
	labels := append([]string(nil), askedLabels...)
	mu.Unlock()

	if key == "" {
		t.Fatalf("AskUserQuestion 未被触发，实证结论不成立；进入回调的工具=%v", tools)
	}

	// CLI 是否接受非 label 字符串，由三项观察共同判定：工具结果未被标为
	// schema 错误、结果正文未提示非法答案、模型后续文本能引用该自由文本。
	echoed := strings.Contains(assistantText.String(), freeText)
	resultEchoed := strings.Contains(resultText, freeText)
	t.Logf("free_text_probe answer_key_kind=%s option_labels=%v tool_result_is_error=%s "+
		"tool_result_echoed_free_text=%t assistant_echoed_free_text=%t",
		liveAnswerKeyKind(key, labels), labels, resultIsError, resultEchoed, echoed)
	t.Logf("free_text_probe tool_result_text_len=%d assistant_text_len=%d",
		len(resultText), assistantText.Len())
	if !resultEchoed && !echoed {
		t.Logf("free_text_probe verdict=rejected_or_ignored tool_result_excerpt=%q",
			liveExcerpt(resultText))
		return
	}
	t.Log("free_text_probe verdict=accepted")
}

// liveAnswerKeyKind 只报告 key 的来源类别，避免把问题原文写进测试输出。
func liveAnswerKeyKind(key string, labels []string) string {
	if strings.HasPrefix(key, "question_") {
		return "fallback_index"
	}
	for _, label := range labels {
		if key == label {
			return "unexpected_label"
		}
	}
	return "question_or_header"
}

// liveExcerpt 截取诊断片段，避免超长输出。
func liveExcerpt(text string) string {
	const limit = 200
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "…"
}
