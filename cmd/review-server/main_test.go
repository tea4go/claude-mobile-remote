package main

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tea4go/claude-mobile-remote/internal/transport"
	"github.com/tea4go/claude-mobile-remote/protocol"
)

func TestRejectUnsafeConfiguration(t *testing.T) {
	for _, c := range []struct{ listen, url string }{{"0.0.0.0:1234", "wss://example.test/ws"}, {"127.0.0.1:1234", "ws://example.test/ws"}, {"127.0.0.1:1234", "wss://example.test/ws?token=secret"}} {
		if validate(c.listen, c.url, time.Hour) == nil {
			t.Fatal("unsafe configuration accepted")
		}
	}
}

func TestServiceAuthenticationAndRestart(t *testing.T) {
	dir := t.TempDir()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	var first protocol.Pairing
	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- serve(ctx, addr, "wss://example.test/ws", dir, time.Hour) }()
		var conn *websocket.Conn
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			conn, _, err = websocket.DefaultDialer.Dial("ws://"+addr+"/ws", nil)
			if err == nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if err != nil {
			cancel()
			t.Fatal("listener failed")
		}
		b, err := os.ReadFile(filepath.Join(dir, "pairing.json"))
		if err != nil {
			t.Fatal(err)
		}
		var p protocol.Pairing
		if err = json.Unmarshal(b, &p); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = p
		} else if p.PairToken != first.PairToken || !p.ExpiresAt.Equal(first.ExpiresAt) {
			t.Fatal("credential changed on restart")
		}
		for _, name := range []string{"pairing.json", "pairing.png", "credential.json"} {
			info, err := os.Stat(filepath.Join(dir, name))
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatal("private file permissions")
			}
		}
		send := func(kind transport.MessageType, payload any) {
			e, err := transport.NewEnvelope(kind, demoID, "", "", 0, payload)
			if err != nil {
				t.Fatal(err)
			}
			if err = conn.WriteJSON(e); err != nil {
				t.Fatal(err)
			}
		}
		read := func(kind transport.MessageType) {
			_ = conn.SetReadDeadline(time.Now().Add(time.Second))
			var e transport.Envelope
			if err := conn.ReadJSON(&e); err != nil {
				t.Fatal(err)
			}
			if e.Type != kind {
				t.Fatalf("unexpected response type %s", e.Type)
			}
		}
		send(transport.TypeHello, transport.HelloPayload{PairToken: p.PairToken})
		read(transport.TypeHelloAck)
		send(transport.TypeSubscribe, struct{}{})
		read(transport.TypeSubscribed)
		read(transport.TypeSnapshot)
		_ = conn.Close()
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("shutdown timeout")
		}
	}
}
