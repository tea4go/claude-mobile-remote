package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	projectclaude "github.com/tea4go/claude-mobile-remote/internal/claude"
)

type fakeTurn struct {
	events []projectclaude.Event
	err    error
}

type fakeHost struct {
	connectErr   error
	closeErr     error
	turns        map[string]fakeTurn
	prompts      []string
	connectCalls int
	closeCalls   int
	connected    chan struct{}
}

func (f *fakeHost) Connect(context.Context) error {
	f.connectCalls++
	if f.connected != nil {
		close(f.connected)
	}
	return f.connectErr
}

func (f *fakeHost) RunTurn(_ context.Context, prompt string, emit func(projectclaude.Event)) error {
	f.prompts = append(f.prompts, prompt)
	turn := f.turns[prompt]
	for _, event := range turn.events {
		emit(event)
	}
	return turn.err
}

func (f *fakeHost) SessionID() string { return "session-1" }

func (f *fakeHost) Close() error {
	f.closeCalls++
	return f.closeErr
}

func TestRunProcessesLinesUntilQuit(t *testing.T) {
	host := &fakeHost{turns: map[string]fakeTurn{
		"first": {events: []projectclaude.Event{
			{Kind: projectclaude.EventSessionStarted, SessionID: "session-1"},
			{Kind: projectclaude.EventText, Text: "one"},
		}},
		"second": {events: []projectclaude.Event{{Kind: projectclaude.EventText, Text: "two"}}},
	}}
	var stdout, stderr bytes.Buffer
	exitCode := run(context.Background(), strings.NewReader("\nfirst\nsecond\n:quit\nignored\n"), &stdout, &stderr, host)

	if exitCode != 0 || stderr.Len() != 0 {
		t.Fatalf("exitCode = %d, stderr = %q", exitCode, stderr.String())
	}
	if !reflect.DeepEqual(host.prompts, []string{"first", "second"}) {
		t.Fatalf("prompts = %#v", host.prompts)
	}
	if host.connectCalls != 1 || host.closeCalls != 1 {
		t.Fatalf("connect/close calls = %d/%d", host.connectCalls, host.closeCalls)
	}
	if stdout.String() != "session: session-1\none\ntwo\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunContinuesAfterTurnError(t *testing.T) {
	host := &fakeHost{turns: map[string]fakeTurn{
		"first": {
			events: []projectclaude.Event{{Kind: projectclaude.EventText, Text: "partial"}},
			err:    errors.New("recoverable"),
		},
		"second": {events: []projectclaude.Event{{Kind: projectclaude.EventText, Text: "ok"}}},
	}}
	firstTurn := host.turns["first"]
	firstTurn.err = projectclaudeTestError(projectclaude.ScopeTurn, firstTurn.err)
	host.turns["first"] = firstTurn
	var stdout, stderr bytes.Buffer
	exitCode := run(context.Background(), strings.NewReader("first\nsecond\n"), &stdout, &stderr, host)

	if exitCode != 0 || !strings.Contains(stderr.String(), "recoverable") {
		t.Fatalf("exitCode = %d, stderr = %q", exitCode, stderr.String())
	}
	if !reflect.DeepEqual(host.prompts, []string{"first", "second"}) {
		t.Fatalf("prompts = %#v", host.prompts)
	}
	if stdout.String() != "partial\nok\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunStopsOnConnectOrSessionError(t *testing.T) {
	t.Run("connect", func(t *testing.T) {
		host := &fakeHost{connectErr: errors.New("connect failed")}
		var stdout, stderr bytes.Buffer
		if got := run(context.Background(), strings.NewReader("ignored\n"), &stdout, &stderr, host); got != 1 {
			t.Fatalf("exitCode = %d", got)
		}
		if len(host.prompts) != 0 || host.closeCalls != 1 || !strings.Contains(stderr.String(), "connect failed") {
			t.Fatalf("host = %#v, stderr = %q", host, stderr.String())
		}
	})

	t.Run("session", func(t *testing.T) {
		host := &fakeHost{turns: map[string]fakeTurn{
			"first":  {err: errors.New("stream failed")},
			"second": {},
		}}
		var stdout, stderr bytes.Buffer
		if got := run(context.Background(), strings.NewReader("first\nsecond\n"), &stdout, &stderr, host); got != 1 {
			t.Fatalf("exitCode = %d", got)
		}
		if !reflect.DeepEqual(host.prompts, []string{"first"}) {
			t.Fatalf("prompts = %#v", host.prompts)
		}
	})
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func TestRunReportsScannerAndCloseErrors(t *testing.T) {
	t.Run("scanner", func(t *testing.T) {
		host := &fakeHost{}
		var stdout, stderr bytes.Buffer
		exitCode := run(context.Background(), failingReader{err: errors.New("read failed")}, &stdout, &stderr, host)
		if exitCode != 1 || !strings.Contains(stderr.String(), "读取输入") {
			t.Fatalf("exitCode = %d, stderr = %q", exitCode, stderr.String())
		}
	})

	t.Run("close after success", func(t *testing.T) {
		host := &fakeHost{closeErr: errors.New("close failed")}
		var stdout, stderr bytes.Buffer
		exitCode := run(context.Background(), strings.NewReader(""), &stdout, &stderr, host)
		if exitCode != 1 || !strings.Contains(stderr.String(), "close failed") {
			t.Fatalf("exitCode = %d, stderr = %q", exitCode, stderr.String())
		}
	})

	t.Run("close after primary error", func(t *testing.T) {
		host := &fakeHost{connectErr: errors.New("primary"), closeErr: errors.New("close failed")}
		var stdout, stderr bytes.Buffer
		exitCode := run(context.Background(), strings.NewReader(""), &stdout, &stderr, host)
		if exitCode != 1 || !strings.Contains(stderr.String(), "primary") || !strings.Contains(stderr.String(), "close failed") {
			t.Fatalf("exitCode = %d, stderr = %q", exitCode, stderr.String())
		}
	})
}

func TestRunEOFAndEmptyLinesExitNormally(t *testing.T) {
	host := &fakeHost{}
	exitCode := run(context.Background(), strings.NewReader("\n  \n"), io.Discard, io.Discard, host)
	if exitCode != 0 || len(host.prompts) != 0 {
		t.Fatalf("exitCode = %d, prompts = %#v", exitCode, host.prompts)
	}
}

func TestRunAcceptsPromptLargerThanScannerDefault(t *testing.T) {
	prompt := strings.Repeat("x", 70*1024)
	host := &fakeHost{}
	exitCode := run(context.Background(), strings.NewReader(prompt+"\n:quit\n"), io.Discard, io.Discard, host)
	if exitCode != 0 || !reflect.DeepEqual(host.prompts, []string{prompt}) {
		t.Fatalf("exitCode = %d, prompt count = %d", exitCode, len(host.prompts))
	}
}

type blockingReadCloser struct {
	closed chan struct{}
	once   sync.Once
}

func newBlockingReadCloser() *blockingReadCloser {
	return &blockingReadCloser{closed: make(chan struct{})}
}

func (r *blockingReadCloser) Read([]byte) (int, error) {
	<-r.closed
	return 0, io.EOF
}

func (r *blockingReadCloser) Close() error {
	r.once.Do(func() { close(r.closed) })
	return nil
}

func TestRunCancellationWhileIdleClosesHost(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reader := newBlockingReadCloser()
	connected := make(chan struct{})
	host := &fakeHost{connected: connected}
	done := make(chan int, 1)
	go func() {
		done <- run(ctx, reader, io.Discard, io.Discard, host)
	}()

	<-connected
	cancel()
	select {
	case exitCode := <-done:
		if exitCode != 0 || host.closeCalls != 1 {
			t.Fatalf("exitCode = %d, closeCalls = %d", exitCode, host.closeCalls)
		}
	case <-time.After(500 * time.Millisecond):
		_ = reader.Close()
		<-done
		t.Fatal("run did not stop after context cancellation while idle")
	}
}

type testScopedError struct {
	scope projectclaude.ErrorScope
	err   error
}

func (e testScopedError) Error() string                   { return e.err.Error() }
func (e testScopedError) Unwrap() error                   { return e.err }
func (e testScopedError) Scope() projectclaude.ErrorScope { return e.scope }

func projectclaudeTestError(scope projectclaude.ErrorScope, err error) error {
	return testScopedError{scope: scope, err: err}
}
