package pairing

import (
	"bytes"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestManagerIssueAndAuthenticate(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	manager, err := NewManager(
		"ws://100.64.0.10:8765/ws",
		10*time.Minute,
		WithClock(func() time.Time { return now }),
		WithRandom(bytes.NewReader(bytes.Repeat([]byte{0x2a}, 32))),
	)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	payload, err := manager.Issue("session-1")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if payload.Version != 1 || payload.WSURL != "ws://100.64.0.10:8765/ws" || payload.SessionID != "session-1" {
		t.Fatalf("unexpected payload metadata: version=%d wsURL=%q sessionID=%q", payload.Version, payload.WSURL, payload.SessionID)
	}
	if payload.PairToken == "" {
		t.Fatal("Issue() returned an empty pair token")
	}
	if !payload.ExpiresAt.Equal(now.Add(10 * time.Minute)) {
		t.Fatalf("expiresAt = %v, want %v", payload.ExpiresAt, now.Add(10*time.Minute))
	}

	lease, err := manager.Authenticate("session-1", payload.PairToken)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	defer lease.Close()

	if _, err := manager.Authenticate("session-2", payload.PairToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("cross-session Authenticate() error = %v, want ErrUnauthorized", err)
	}
	if _, err := manager.Authenticate("session-1", "invalid"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("invalid-token Authenticate() error = %v, want ErrUnauthorized", err)
	}
}

func TestPayloadMarshalUsesSharedJSONFields(t *testing.T) {
	payload := Payload{
		Version:   1,
		WSURL:     "ws://pc/ws",
		SessionID: "session-1",
		PairToken: "fixture-credential",
		ExpiresAt: time.Date(2026, 8, 24, 12, 10, 0, 0, time.UTC),
	}
	data, err := payload.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	for _, name := range []string{"version", "wsUrl", "sessionId", "pairToken", "expiresAt"} {
		if _, ok := fields[name]; !ok {
			t.Fatalf("Marshal() omitted %q", name)
		}
	}
}

func TestManagerExpiryRotationAndRevocation(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	random := append(bytes.Repeat([]byte{0x11}, 32), bytes.Repeat([]byte{0x22}, 32)...)
	manager, err := NewManager(
		"ws://100.64.0.10:8765/ws",
		10*time.Minute,
		WithClock(func() time.Time { return now }),
		WithRandom(bytes.NewReader(random)),
	)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	first, err := manager.Issue("session-1")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	firstLease, err := manager.Authenticate("session-1", first.PairToken)
	if err != nil {
		t.Fatalf("Authenticate(first) error = %v", err)
	}

	now = now.Add(11 * time.Minute)
	if _, err := manager.Authenticate("session-1", first.PairToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expired Authenticate() error = %v, want ErrUnauthorized", err)
	}
	select {
	case <-firstLease.Done():
		t.Fatal("natural expiry closed an established lease")
	default:
	}

	second, rotated, err := manager.RotateIfExpired()
	if err != nil {
		t.Fatalf("RotateIfExpired() error = %v", err)
	}
	if !rotated {
		t.Fatal("RotateIfExpired() did not rotate an expired credential")
	}
	if second.PairToken == "" || second.PairToken == first.PairToken {
		t.Fatal("rotation did not return a new credential")
	}
	secondLease, err := manager.Authenticate("session-1", second.PairToken)
	if err != nil {
		t.Fatalf("Authenticate(second) error = %v", err)
	}

	manager.Revoke()
	for name, lease := range map[string]*Lease{"first": firstLease, "second": secondLease} {
		select {
		case <-lease.Done():
		case <-time.After(time.Second):
			t.Fatalf("Revoke() did not close %s lease", name)
		}
	}
	if _, err := manager.Authenticate("session-1", second.PairToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("revoked Authenticate() error = %v, want ErrUnauthorized", err)
	}
}

func TestManagerInvalidatePreservesEstablishedLease(t *testing.T) {
	manager, err := NewManager(
		"ws://100.64.0.10:8765/ws",
		10*time.Minute,
		WithRandom(bytes.NewReader(bytes.Repeat([]byte{0x33}, 32))),
	)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	payload, err := manager.Issue("session-1")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	lease, err := manager.Authenticate("session-1", payload.PairToken)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	defer lease.Close()

	manager.Invalidate()
	if _, err := manager.Authenticate("session-1", payload.PairToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("invalidated Authenticate() error = %v, want ErrUnauthorized", err)
	}
	select {
	case <-lease.Done():
		t.Fatal("Invalidate() closed an established lease")
	default:
	}
}

func TestManagerConcurrentAuthenticateAndRevoke(t *testing.T) {
	manager, err := NewManager(
		"ws://100.64.0.10:8765/ws",
		time.Hour,
		WithRandom(bytes.NewReader(bytes.Repeat([]byte{0x44}, 32))),
	)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	payload, err := manager.Issue("session-1")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	start := make(chan struct{})
	var wait sync.WaitGroup
	for range 32 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			lease, authErr := manager.Authenticate("session-1", payload.PairToken)
			if authErr == nil {
				lease.Close()
				return
			}
			if !errors.Is(authErr, ErrUnauthorized) {
				t.Errorf("Authenticate() error = %v", authErr)
			}
		}()
	}
	close(start)
	manager.Revoke()
	wait.Wait()
}
