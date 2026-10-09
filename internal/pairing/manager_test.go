package pairing

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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

func TestManagerPCNameOnIssueAndRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pairing.json")
	options := []Option{WithPersistenceFile(path), WithPCID("0123456789abcdef0123456789abcdef"), WithPCName("zhanglingsiMac-mini.local"), WithRandom(bytes.NewReader(bytes.Repeat([]byte{0x42}, 32)))}
	first, err := NewManager("ws://127.0.0.1:8765/ws", time.Hour, options...)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := first.Issue("session-1")
	if err != nil {
		t.Fatal(err)
	}
	if payload.PCName != "zhanglingsiMac-mini.local" {
		t.Fatalf("PCName = %q", payload.PCName)
	}
	second, err := NewManager("ws://127.0.0.1:8765/ws", time.Hour, options...)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := second.Issue("session-1")
	if err != nil {
		t.Fatal(err)
	}
	if restored.PCName != payload.PCName || restored.PairToken != payload.PairToken {
		t.Fatal("restored pairing lost PC name or token")
	}
	if _, err := NewManager("ws://127.0.0.1:8765/ws", time.Hour, WithPCName("bad\nname")); err == nil {
		t.Fatal("control character in PC name accepted")
	}
}

func TestManagerRestoresUnexpiredCredentialAfterRestart(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "pairing.json")
	clock := func() time.Time { return now }

	first, err := NewManager("ws://127.0.0.1:8765/ws", time.Hour, WithClock(clock), WithPersistenceFile(path), WithRandom(bytes.NewReader(bytes.Repeat([]byte{0x41}, 32))))
	if err != nil {
		t.Fatalf("NewManager(first) error = %v", err)
	}
	initial, err := first.Issue("session-1")
	if err != nil {
		t.Fatalf("Issue(first) error = %v", err)
	}

	second, err := NewManager("ws://127.0.0.1:8765/ws", time.Hour, WithClock(clock), WithPersistenceFile(path), WithRandom(bytes.NewReader(bytes.Repeat([]byte{0x99}, 32))))
	if err != nil {
		t.Fatalf("NewManager(second) error = %v", err)
	}
	restored, err := second.Issue("session-1")
	if err != nil {
		t.Fatalf("Issue(second) error = %v", err)
	}
	if restored.PairToken != initial.PairToken || !restored.ExpiresAt.Equal(initial.ExpiresAt) {
		t.Fatalf("Issue(second) did not restore credential: got token changed or expiry changed")
	}
	if _, err := second.Authenticate("session-1", initial.PairToken); err != nil {
		t.Fatalf("Authenticate(restored) error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(pairing file) error = %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("pairing file mode = %o, want 600", info.Mode().Perm())
	}
}

func TestManagerPCIDSurvivesCredentialRestoreAndRotation(t *testing.T) {
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "pairing.json")
	pcID := "0123456789abcdef0123456789abcdef"
	first, err := NewManager("ws://127.0.0.1:8765/ws", time.Hour, WithClock(func() time.Time { return now }), WithPersistenceFile(path), WithPCID(pcID))
	if err != nil {
		t.Fatal(err)
	}
	issued, err := first.Issue("session-1")
	if err != nil {
		t.Fatal(err)
	}
	if issued.PCID != pcID {
		t.Fatal("issued PC ID missing")
	}
	second, err := NewManager("ws://127.0.0.1:8765/ws", time.Hour, WithClock(func() time.Time { return now }), WithPersistenceFile(path), WithPCID(pcID))
	if err != nil {
		t.Fatal(err)
	}
	restored, err := second.Issue("session-1")
	if err != nil {
		t.Fatal(err)
	}
	if restored.PCID != pcID || restored.PairToken != issued.PairToken {
		t.Fatal("restored pairing changed PC ID or token")
	}
	now = now.Add(2 * time.Hour)
	rotated, ok, err := second.RotateIfExpired()
	if err != nil || !ok || rotated.PCID != pcID {
		t.Fatalf("rotated PC ID = %q, ok = %v, err = %v", rotated.PCID, ok, err)
	}
}

func TestManagerReplacesExpiredOrDifferentSessionCredential(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "pairing.json")
	clock := func() time.Time { return now }
	random := append(bytes.Repeat([]byte{0x51}, 32), bytes.Repeat([]byte{0x52}, 32)...)
	manager, err := NewManager("ws://127.0.0.1:8765/ws", time.Hour, WithClock(clock), WithPersistenceFile(path), WithRandom(bytes.NewReader(random)))
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	initial, err := manager.Issue("session-1")
	if err != nil {
		t.Fatalf("Issue(initial) error = %v", err)
	}
	now = now.Add(2 * time.Hour)
	expired, err := manager.Issue("session-1")
	if err != nil {
		t.Fatalf("Issue(expired) error = %v", err)
	}
	if expired.PairToken == initial.PairToken {
		t.Fatal("expired credential was restored")
	}

	other, err := NewManager("ws://127.0.0.1:8765/ws", time.Hour, WithClock(clock), WithPersistenceFile(path), WithRandom(bytes.NewReader(bytes.Repeat([]byte{0x71}, 32))))
	if err != nil {
		t.Fatalf("NewManager(other) error = %v", err)
	}
	otherPayload, err := other.Issue("session-2")
	if err != nil {
		t.Fatalf("Issue(other) error = %v", err)
	}
	if otherPayload.PairToken == expired.PairToken {
		t.Fatal("different session reused persisted credential")
	}
}

func TestManagerReplacesDamagedPersistenceFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pairing.json")
	if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	manager, err := NewManager(
		"ws://127.0.0.1:8765/ws",
		time.Hour,
		WithPersistenceFile(path),
		WithRandom(bytes.NewReader(bytes.Repeat([]byte{0x61}, 32))),
	)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if _, err := manager.Issue("session-1"); err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var stored persistedPayload
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("replacement file is not valid JSON: %v", err)
	}
	if stored.SessionID != "session-1" || stored.PairToken == "" {
		t.Fatalf("replacement record = %+v", stored)
	}
}

func TestManagerDegradesWhenPersistenceFileUnusable(t *testing.T) {
	// A directory in place of the record makes both the read and the atomic
	// replace fail. Persistence is a convenience, so the Session must still start.
	path := filepath.Join(t.TempDir(), "pairing.json")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	manager, err := NewManager(
		"ws://127.0.0.1:8765/ws",
		time.Hour,
		WithPersistenceFile(path),
		WithRandom(bytes.NewReader(bytes.Repeat([]byte{0x63}, 32))),
	)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	payload, err := manager.Issue("session-1")
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if payload.PairToken == "" {
		t.Fatal("Issue() returned an empty credential")
	}
	if _, err := manager.Authenticate("session-1", payload.PairToken); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("failed save left %d entries behind, want only the blocking directory", len(entries))
	}
}

func TestManagerRestoreUsesCurrentAdvertiseURL(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), "pairing.json")
	clock := func() time.Time { return now }

	first, err := NewManager("ws://127.0.0.1:8765/ws", time.Hour, WithClock(clock), WithPersistenceFile(path), WithRandom(bytes.NewReader(bytes.Repeat([]byte{0x81}, 32))))
	if err != nil {
		t.Fatalf("NewManager(first) error = %v", err)
	}
	initial, err := first.Issue("session-1")
	if err != nil {
		t.Fatalf("Issue(first) error = %v", err)
	}

	// A restart on another address must keep the credential but advertise the
	// endpoint the current process actually listens on.
	second, err := NewManager("ws://192.168.1.20:9000/ws", time.Hour, WithClock(clock), WithPersistenceFile(path), WithRandom(bytes.NewReader(bytes.Repeat([]byte{0x82}, 32))))
	if err != nil {
		t.Fatalf("NewManager(second) error = %v", err)
	}
	restored, err := second.Issue("session-1")
	if err != nil {
		t.Fatalf("Issue(second) error = %v", err)
	}
	if restored.PairToken != initial.PairToken {
		t.Fatal("Issue(second) did not restore the persisted credential")
	}
	if restored.WSURL != "ws://192.168.1.20:9000/ws" {
		t.Fatalf("restored WSURL = %q, want the current advertise URL", restored.WSURL)
	}
}

func TestPruneExpiredRemovesOnlyExpiredRecords(t *testing.T) {
	directory := t.TempDir()
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	writeRecord := func(name string, expiresAt time.Time) string {
		path := filepath.Join(directory, name)
		data, err := json.Marshal(persistedPayload{SessionID: name, PairToken: "token", ExpiresAt: expiresAt})
		if err != nil {
			t.Fatalf("Marshal(%s) error = %v", name, err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
		return path
	}
	expired := writeRecord("expired.json", now.Add(-time.Minute))
	valid := writeRecord("valid.json", now.Add(time.Hour))
	damaged := filepath.Join(directory, "damaged.json")
	if err := os.WriteFile(damaged, []byte("not-json"), 0o600); err != nil {
		t.Fatalf("WriteFile(damaged) error = %v", err)
	}
	unrelated := filepath.Join(directory, "notes.txt")
	if err := os.WriteFile(unrelated, []byte("keep"), 0o600); err != nil {
		t.Fatalf("WriteFile(unrelated) error = %v", err)
	}

	if err := PruneExpired(directory, now); err != nil {
		t.Fatalf("PruneExpired() error = %v", err)
	}
	if _, err := os.Stat(expired); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expired record was kept: err = %v", err)
	}
	for _, path := range []string{valid, damaged, unrelated} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("PruneExpired removed %s: %v", filepath.Base(path), err)
		}
	}
	if err := PruneExpired(filepath.Join(directory, "missing"), now); err != nil {
		t.Fatalf("PruneExpired(missing directory) error = %v", err)
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
