package pairing

import (
	"strings"
	"testing"
	"time"

	"github.com/skip2/go-qrcode"
	"github.com/tea4go/claude-mobile-remote/protocol"
)

func TestRenderTerminalQR(t *testing.T) {
	payload := protocol.Pairing{
		Version:   protocol.PayloadVersion,
		WSURL:     "ws://192.0.2.10:8765/ws",
		SessionID: "session-1",
		PairToken: "token-fixture",
		ExpiresAt: time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC),
	}

	qr, err := RenderTerminalQR(payload)
	if err != nil {
		t.Fatalf("RenderTerminalQR() error = %v", err)
	}
	if strings.TrimSpace(qr) == "" {
		t.Fatal("RenderTerminalQR() returned empty output")
	}
	if lines := strings.Count(qr, "\n"); lines < 5 {
		t.Fatalf("QR output has %d lines, want at least 5", lines)
	}
	if strings.Contains(qr, payload.PairToken) {
		t.Fatal("QR output exposed the human-readable pair token")
	}

	content := protocol.EncodePairingFragment(payload)
	legacy, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		t.Fatalf("qrcode.New() error = %v", err)
	}
	legacyQR := legacy.ToString(false)
	if got, want := maxLineWidth(qr), maxLineWidth(legacyQR); got >= want {
		t.Fatalf("QR width = %d, want less than legacy width %d", got, want)
	}
	if got, want := strings.Count(qr, "\n"), strings.Count(legacyQR, "\n"); got >= want {
		t.Fatalf("QR height = %d, want less than legacy height %d", got, want)
	}

	compact, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		t.Fatalf("qrcode.New() compact error = %v", err)
	}
	compact.DisableBorder = true
	if got, want := qr, compact.ToSmallString(false); got != want {
		t.Fatalf("QR rendering does not use compact borderless form")
	}
}

func maxLineWidth(value string) int {
	max := 0
	for _, line := range strings.Split(value, "\n") {
		if len([]rune(line)) > max {
			max = len([]rune(line))
		}
	}
	return max
}
