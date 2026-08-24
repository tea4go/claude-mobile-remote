package transport

import (
	"errors"
	"testing"
	"time"
)

func TestCommandGateEnqueuesResultBeforeEarlyEvents(t *testing.T) {
	outbox := newOutbox(64, 1024*1024)
	gate := newCommandGate(outbox, 32, 256*1024)
	if err := gate.Begin("command-1"); err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if err := gate.EnqueueEvent([]byte("event-1")); err != nil {
		t.Fatalf("EnqueueEvent(first) error = %v", err)
	}
	if err := gate.EnqueueEvent([]byte("event-2")); err != nil {
		t.Fatalf("EnqueueEvent(second) error = %v", err)
	}
	if err := gate.Finish([]byte("result")); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}
	for _, want := range []string{"result", "event-1", "event-2"} {
		got, ok := outbox.Next()
		if !ok || string(got) != want {
			t.Fatalf("Next() = %q, %v, want %q", got, ok, want)
		}
	}
}

func TestOutboxRejectsCountAndByteOverflow(t *testing.T) {
	countLimited := newOutbox(2, 1024)
	if err := countLimited.Enqueue([]byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := countLimited.Enqueue([]byte("two")); err != nil {
		t.Fatal(err)
	}
	if err := countLimited.Enqueue([]byte("three")); !errors.Is(err, errOutboxOverflow) {
		t.Fatalf("count overflow error = %v, want errOutboxOverflow", err)
	}

	byteLimited := newOutbox(10, 3)
	if err := byteLimited.Enqueue([]byte("ab")); err != nil {
		t.Fatal(err)
	}
	if err := byteLimited.Enqueue([]byte("cd")); !errors.Is(err, errOutboxOverflow) {
		t.Fatalf("byte overflow error = %v, want errOutboxOverflow", err)
	}
}

func TestCommandGateRejectsEarlyCountAndByteOverflow(t *testing.T) {
	countGate := newCommandGate(newOutbox(64, 1024), 1, 1024)
	if err := countGate.Begin("command-1"); err != nil {
		t.Fatal(err)
	}
	if err := countGate.EnqueueEvent([]byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := countGate.EnqueueEvent([]byte("two")); !errors.Is(err, errEarlyBufferOverflow) {
		t.Fatalf("early count overflow error = %v, want errEarlyBufferOverflow", err)
	}

	byteGate := newCommandGate(newOutbox(64, 1024), 10, 3)
	if err := byteGate.Begin("command-2"); err != nil {
		t.Fatal(err)
	}
	if err := byteGate.EnqueueEvent([]byte("ab")); err != nil {
		t.Fatal(err)
	}
	if err := byteGate.EnqueueEvent([]byte("cd")); !errors.Is(err, errEarlyBufferOverflow) {
		t.Fatalf("early byte overflow error = %v, want errEarlyBufferOverflow", err)
	}
}

func TestOutboxCloseWakesBlockedNext(t *testing.T) {
	outbox := newOutbox(1, 1)
	done := make(chan bool, 1)
	go func() {
		_, ok := outbox.Next()
		done <- ok
	}()
	outbox.Close()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("Next() reported an item after Close")
		}
	case <-time.After(time.Second):
		t.Fatal("Close() did not wake Next()")
	}
}

func TestCommandGateRejectsSecondInFlightCommandAndResets(t *testing.T) {
	gate := newCommandGate(newOutbox(4, 1024), 4, 1024)
	if err := gate.Begin("command-1"); err != nil {
		t.Fatalf("Begin(first) error = %v", err)
	}
	if err := gate.Begin("command-2"); !errors.Is(err, errCommandInFlight) {
		t.Fatalf("Begin(second) error = %v, want errCommandInFlight", err)
	}
	if err := gate.Abort([]byte("rejected")); err != nil {
		t.Fatalf("Abort() error = %v", err)
	}
	if err := gate.Begin("command-2"); err != nil {
		t.Fatalf("Begin(after reset) error = %v", err)
	}
}

func TestOutboxCopiesEnqueuedBytes(t *testing.T) {
	outbox := newOutbox(1, 16)
	data := []byte("one")
	if err := outbox.Enqueue(data); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	data[0] = 'X'
	got, ok := outbox.Next()
	if !ok || string(got) != "one" {
		t.Fatalf("Next() = %q, %v", got, ok)
	}
}
