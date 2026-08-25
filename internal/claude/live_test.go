//go:build integration

package claude

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
