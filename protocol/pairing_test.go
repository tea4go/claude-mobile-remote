package protocol

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestEncodePairingFragmentUsesMobileWebContract(t *testing.T) {
	payload := Pairing{
		Version:   PayloadVersion,
		WSURL:     "ws://192.0.2.10:8765/ws",
		SessionID: "session-1",
		PairToken: "token-fixture",
		ExpiresAt: time.Date(2026, 8, 31, 12, 0, 0, 123000000, time.UTC),
	}

	fragment := EncodePairingFragment(payload)
	if !strings.HasPrefix(fragment, "#pair=") {
		t.Fatalf("fragment = %q, want #pair= prefix", fragment)
	}
	encoded := strings.TrimPrefix(fragment, "#pair=")
	if strings.ContainsAny(encoded, "=+/\n\r") {
		t.Fatalf("fragment encoding contains non-Base64URL characters: %q", encoded)
	}

	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode fragment: %v", err)
	}
	var got Pairing
	if err := json.Unmarshal(decoded, &got); err != nil {
		t.Fatalf("decode pairing JSON: %v", err)
	}
	if got.Version != payload.Version || got.WSURL != payload.WSURL || got.SessionID != payload.SessionID || got.PairToken != payload.PairToken || !got.ExpiresAt.Equal(payload.ExpiresAt) {
		t.Fatalf("decoded payload = %+v, want %+v", got, payload)
	}
}
