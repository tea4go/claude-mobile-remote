//go:build integration

package claude

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLiveTwoTurnSession(t *testing.T) {
	if os.Getenv("CLAUDE_MOBILE_REMOTE_LIVE") != "1" {
		t.Skip("set CLAUDE_MOBILE_REMOTE_LIVE=1 to run the live Claude test")
	}

	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		t.Fatalf("generate code word: %v", err)
	}
	codeWord := hex.EncodeToString(randomBytes)

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
