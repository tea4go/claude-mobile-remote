package main

import (
	"bytes"
	"context"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRunMainVersionDoesNotBuildRuntime(t *testing.T) {
	called := false
	var output bytes.Buffer
	exitCode := runMain(context.Background(), []string{"-version"}, strings.NewReader(""), &output, io.Discard,
		func(cliConfig) (sessionController, remoteService, error) {
			called = true
			return nil, nil, nil
		})
	if exitCode != 0 || output.String() != "0.1.0-dev\n" {
		t.Fatalf("runMain() = %d, output %q", exitCode, output.String())
	}
	if called {
		t.Fatal("-version built runtime dependencies")
	}
}

func TestParseConfig(t *testing.T) {
	got, err := parseConfig(nil)
	if err != nil {
		t.Fatalf("parseConfig(defaults) error = %v", err)
	}
	if got.listen != "127.0.0.1:8765" || got.advertiseURL != "ws://127.0.0.1:8765/ws" || got.pairTokenTTL != 12*time.Hour {
		t.Fatalf("defaults = %#v", got)
	}
	defaultOrigins := []string{"http://127.0.0.1:5173", "http://localhost:5173"}
	if !reflect.DeepEqual(got.allowedOrigins, defaultOrigins) {
		t.Fatalf("default allowed origins = %#v, want %#v", got.allowedOrigins, defaultOrigins)
	}

	got, err = parseConfig([]string{
		"--listen", "0.0.0.0:8765",
		"--advertise-url", "ws://100.64.0.10:8765/ws",
		"--pair-token-ttl", "15m",
		"--allowed-origin", "capacitor://localhost",
		"--allowed-origin", "http://dev.local",
	})
	if err != nil {
		t.Fatalf("parseConfig(explicit) error = %v", err)
	}
	wantOrigins := append(defaultOrigins, "capacitor://localhost", "http://dev.local")
	if !reflect.DeepEqual(got.allowedOrigins, wantOrigins) {
		t.Fatalf("allowed origins = %#v, want %#v", got.allowedOrigins, wantOrigins)
	}
	if got.listen != "0.0.0.0:8765" || got.advertiseURL != "ws://100.64.0.10:8765/ws" || got.pairTokenTTL != 15*time.Minute {
		t.Fatalf("explicit config = %#v", got)
	}
}

func TestParseConfigRejectsInvalidNetworkValues(t *testing.T) {
	tests := [][]string{
		{"--listen", "missing-port"},
		{"--advertise-url", "http://pc:8765/ws"},
		{"--advertise-url", "ws://pc:8765"},
		{"--pair-token-ttl", "0s"},
		{"--allowed-origin", "not-a-url"},
	}
	for _, args := range tests {
		if _, err := parseConfig(args); err == nil {
			t.Fatalf("parseConfig(%q) accepted invalid values", args)
		}
	}
}
