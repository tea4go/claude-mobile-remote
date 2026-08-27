package main

import (
	"errors"
	"flag"
	"io"
	"net"
	"net/url"
	"strings"
	"time"

	projectclaude "github.com/tea4go/claude-mobile-remote/internal/claude"
)

var defaultAllowedOrigins = []string{
	"http://127.0.0.1:5173",
	"http://localhost:5173",
}

type cliConfig struct {
	showVersion     bool
	listen          string
	advertiseURL    string
	pairTokenTTL    time.Duration
	allowedOrigins  []string
	resumeSessionID string
	// fakeHostScenario 为空表示使用真实 Claude Host；非空时改用脚本化桩 Host
	// （B2 Task 4），用于把交互验证与真实 Claude 额度解耦。
	fakeHostScenario projectclaude.FakeScenario
}

type stringListFlag []string

func (values *stringListFlag) String() string {
	return strings.Join(*values, ",")
}

func (values *stringListFlag) Set(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("origin must not be empty")
	}
	*values = append(*values, value)
	return nil
}

func parseConfig(arguments []string) (cliConfig, error) {
	config := cliConfig{
		listen:       "127.0.0.1:8765",
		advertiseURL: "ws://127.0.0.1:8765/ws",
		pairTokenTTL: 12 * time.Hour,
	}
	var extraOrigins stringListFlag
	var fakeHost string
	flags := flag.NewFlagSet("claude-mobile-remote", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.BoolVar(&config.showVersion, "version", false, "打印版本并退出")
	flags.StringVar(&config.listen, "listen", config.listen, "WebSocket 监听地址")
	flags.StringVar(&config.advertiseURL, "advertise-url", config.advertiseURL, "客户端可访问的 WebSocket URL")
	flags.DurationVar(&config.pairTokenTTL, "pair-token-ttl", config.pairTokenTTL, "配对凭证有效期")
	flags.Var(&extraOrigins, "allowed-origin", "额外允许的浏览器 Origin，可重复")
	flags.StringVar(&config.resumeSessionID, "resume", "", "恢复已有会话的 session ID（UUID）")
	flags.StringVar(&fakeHost, "fake-host", "", "使用脚本化假 Host 而非真实 Claude："+fakeHostScenarioList())
	if err := flags.Parse(arguments); err != nil {
		return cliConfig{}, err
	}
	if flags.NArg() != 0 {
		return cliConfig{}, errors.New("unexpected positional arguments")
	}
	if isFlagSet(flags, "fake-host") {
		scenario, err := projectclaude.ParseFakeScenario(fakeHost)
		if err != nil {
			return cliConfig{}, err
		}
		// 桩 Host 没有可恢复的历史；静默忽略 --resume 会让用户误以为接上了旧上下文。
		if config.resumeSessionID != "" {
			return cliConfig{}, errors.New("--fake-host cannot be combined with --resume")
		}
		config.fakeHostScenario = scenario
	}
	if config.resumeSessionID != "" && !isUUID(config.resumeSessionID) {
		return cliConfig{}, errors.New("invalid resume session ID: must be a UUID")
	}
	if _, _, err := net.SplitHostPort(config.listen); err != nil {
		return cliConfig{}, errors.New("invalid listen address")
	}
	if err := validateAdvertiseURL(config.advertiseURL); err != nil {
		return cliConfig{}, err
	}
	if config.pairTokenTTL <= 0 {
		return cliConfig{}, errors.New("pair token TTL must be positive")
	}
	config.allowedOrigins = append([]string(nil), defaultAllowedOrigins...)
	for _, origin := range extraOrigins {
		if err := validateOrigin(origin); err != nil {
			return cliConfig{}, err
		}
		config.allowedOrigins = append(config.allowedOrigins, origin)
	}
	return config, nil
}

// isFlagSet 报告 flag 是否被显式给出。它把 `--fake-host ""` 与完全不给区分开：
// 前者是拼错的开关，必须报错，而不是静默回退到真实 Host。
func isFlagSet(flags *flag.FlagSet, name string) bool {
	present := false
	flags.Visit(func(visited *flag.Flag) {
		if visited.Name == name {
			present = true
		}
	})
	return present
}

func fakeHostScenarioList() string {
	names := make([]string, 0, len(projectclaude.FakeScenarios()))
	for _, scenario := range projectclaude.FakeScenarios() {
		names = append(names, string(scenario))
	}
	return strings.Join(names, "|")
}

func validateAdvertiseURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "ws" && parsed.Scheme != "wss") || parsed.Host == "" || parsed.Path == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("invalid advertised WebSocket URL")
	}
	return nil
}

func validateOrigin(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("invalid allowed origin")
	}
	return nil
}

// isUUID 校验 8-4-4-4-12 十六进制 UUID（A5.2 resume 参数）。
func isUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, char := range value {
		switch i {
		case 8, 13, 18, 23:
			if char != '-' {
				return false
			}
		default:
			isHex := (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')
			if !isHex {
				return false
			}
		}
	}
	return true
}
