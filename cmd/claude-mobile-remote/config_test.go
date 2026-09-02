package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	projectclaude "github.com/tea4go/claude-mobile-remote/internal/claude"
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
	if got.listen != "127.0.0.1:8765" || got.advertiseURL != "ws://127.0.0.1:8765/ws" || got.pairTokenTTL != 30*24*time.Hour {
		t.Fatalf("defaults = %#v", got)
	}
	if got.profile != "local" {
		t.Fatalf("default profile = %q, want local", got.profile)
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

func TestParseConfigProfiles(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		profile     string
		listen      string
		advertise   string
		origins     []string
		advertiseIP string
	}{
		{name: "emulator", args: []string{"--profile", "emulator"}, profile: "emulator", listen: "0.0.0.0:8765", advertise: "ws://10.0.2.2:8765/ws", origins: []string{"http://localhost", "http://10.0.2.2:5173"}},
		{name: "emulator custom port", args: []string{"--profile", "emulator", "--listen", "0.0.0.0:9000"}, profile: "emulator", listen: "0.0.0.0:9000", advertise: "ws://10.0.2.2:9000/ws", origins: []string{"http://localhost", "http://10.0.2.2:5173"}},
		{name: "device", args: []string{"--profile", "device", "--advertise-host", "10.88.16.45"}, profile: "device", listen: "0.0.0.0:8765", advertise: "ws://10.88.16.45:8765/ws", origins: []string{"http://localhost"}, advertiseIP: "10.88.16.45"},
		{name: "debug", args: []string{"--profile", "debug", "--listen", "127.0.0.1:9000", "--advertise-url", "ws://debug.local:9000/ws", "--allowed-origin", "http://debug.local"}, profile: "debug", listen: "127.0.0.1:9000", advertise: "ws://debug.local:9000/ws", origins: []string{"http://debug.local"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseConfig(test.args)
			if err != nil {
				t.Fatalf("parseConfig() error = %v", err)
			}
			if got.profile != test.profile || got.listen != test.listen || got.advertiseURL != test.advertise {
				t.Fatalf("profile config = %#v", got)
			}
			for _, origin := range test.origins {
				if !contains(got.allowedOrigins, origin) {
					t.Fatalf("allowed origins = %#v, want %q", got.allowedOrigins, origin)
				}
			}
			if test.advertiseIP != "" && got.advertiseHost != test.advertiseIP {
				t.Fatalf("advertise host = %q, want %q", got.advertiseHost, test.advertiseIP)
			}
		})
	}
}

func TestParseConfigClaudeOptions(t *testing.T) {
	got, err := parseConfig([]string{"--permission-mode", "acceptEdits", "--model", "claude-sonnet", "--max-turns", "3"})
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}
	if got.permissionMode != "acceptEdits" || got.model != "claude-sonnet" || got.maxTurns != 3 {
		t.Fatalf("Claude options = %#v", got)
	}
}

func TestParseConfigRejectsInvalidProfileAndPermissionMode(t *testing.T) {
	for _, args := range [][]string{
		{"--profile", "unknown"},
		{"--permission-mode", "unknown"},
		{"--max-turns", "0"},
		{"--advertise-host", "not-an-ip"},
	} {
		if _, err := parseConfig(args); err == nil {
			t.Fatalf("parseConfig(%q) accepted invalid values", args)
		}
	}
}

func TestParseConfigCwd(t *testing.T) {
	dir := t.TempDir()
	got, err := parseConfig([]string{"--cwd", dir})
	if err != nil {
		t.Fatalf("parseConfig(--cwd) error = %v", err)
	}
	if got.cwd != dir {
		t.Fatalf("cwd = %q, want %q", got.cwd, dir)
	}

	file := dir + "/file"
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	for _, value := range []string{"", dir + "/missing", file} {
		if _, err := parseConfig([]string{"--cwd", value}); err == nil {
			t.Fatalf("parseConfig(--cwd %q) error = nil, want rejection", value)
		}
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

func TestParseConfigFakeHost(t *testing.T) {
	defaults, err := parseConfig(nil)
	if err != nil {
		t.Fatalf("parseConfig(defaults) error = %v", err)
	}
	if defaults.fakeHostScenario != "" {
		t.Fatalf("default fakeHostScenario = %q, want empty so the real Host stays the default", defaults.fakeHostScenario)
	}

	stubbed, err := parseConfig([]string{"--fake-host", "interactive"})
	if err != nil {
		t.Fatalf("parseConfig(--fake-host) error = %v", err)
	}
	if stubbed.fakeHostScenario != projectclaude.FakeScenarioInteractive {
		t.Fatalf("fakeHostScenario = %q, want %q", stubbed.fakeHostScenario, projectclaude.FakeScenarioInteractive)
	}
	// 桩模式只替换 Host：监听、配对 TTL 和 Origin 白名单必须与真实模式逐字一致。
	stubbed.fakeHostScenario = ""
	if !reflect.DeepEqual(stubbed, defaults) {
		t.Fatalf("--fake-host changed unrelated config: %#v, want %#v", stubbed, defaults)
	}

	for _, args := range [][]string{
		{"--fake-host", "unknown"},
		{"--fake-host", ""},
		{"--fake-host", "Interactive"},
		// 桩不能恢复真实会话；静默忽略 --resume 会让用户误以为接上了旧上下文。
		{"--fake-host", "interactive", "--resume", "11111111-2222-4333-8444-555555555555"},
	} {
		if _, err := parseConfig(args); err == nil {
			t.Fatalf("parseConfig(%q) error = nil, want a rejection", args)
		}
	}
}

func TestBuildRuntimeFakeHostNeedsNoRealClaude(t *testing.T) {
	// 清空 PATH 让任何 Claude 子进程都无法启动：装配仍必须成功。
	t.Setenv("PATH", "")
	config, err := parseConfig([]string{"--fake-host", "interactive", "--listen", "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}
	controller, remote, err := buildRuntime(config)
	if err != nil {
		t.Fatalf("buildRuntime() error = %v", err)
	}
	if controller == nil || remote == nil {
		t.Fatal("buildRuntime() returned a nil dependency")
	}
	if got := controller.ReservedSessionID(); got != projectclaude.FakeSessionID {
		t.Fatalf("ReservedSessionID() = %q, want the fake session ID %q", got, projectclaude.FakeSessionID)
	}
}
