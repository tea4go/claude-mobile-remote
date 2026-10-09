package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	projectclaude "github.com/tea4go/claude-mobile-remote/internal/claude"
	cmrremote "github.com/tea4go/claude-mobile-remote/remote"
)

var defaultAllowedOrigins = []string{
	"http://127.0.0.1:5173",
	"http://localhost:5173",
}

type cliConfig struct {
	pcID            string
	showVersion     bool
	profile         string
	listen          string
	advertiseURL    string
	advertiseHost   string
	pairTokenTTL    time.Duration
	allowedOrigins  []string
	resumeSessionID string
	cwd             string
	permissionMode  string
	model           string
	maxTurns        int
	maxSessions     int
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
		profile:      "local",
		listen:       "127.0.0.1:8765",
		advertiseURL: "ws://127.0.0.1:8765/ws",
		// 配对凭证可跨重启恢复，但 expiresAt 是三端共识的绝对时间，服务端
		// 单方面续期会被客户端的本地过期校验拦下；因此用较长的默认有效期
		// 覆盖「隔几天接着用」，仍可用 --pair-token-ttl 收紧。
		pairTokenTTL: cmrremote.DefaultPairTokenTTL,
		maxSessions:  4,
	}
	config.allowedOrigins = append([]string(nil), defaultAllowedOrigins...)
	var extraOrigins stringListFlag
	var fakeHost string
	flags := flag.NewFlagSet("claude-mobile-remote", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.BoolVar(&config.showVersion, "version", false, "打印版本并退出")
	flags.StringVar(&config.profile, "profile", config.profile, "远程环境: local|emulator|device|debug")
	flags.StringVar(&config.listen, "listen", config.listen, "WebSocket 监听地址")
	flags.StringVar(&config.advertiseURL, "advertise-url", config.advertiseURL, "客户端可访问的 WebSocket URL")
	flags.StringVar(&config.advertiseHost, "advertise-host", "", "device profile 使用的客户端可达 IPv4（高级参数）")
	flags.DurationVar(&config.pairTokenTTL, "pair-token-ttl", config.pairTokenTTL, "配对凭证有效期")
	flags.Var(&extraOrigins, "allowed-origin", "额外允许的浏览器 Origin，可重复")
	flags.StringVar(&config.resumeSessionID, "resume", "", "恢复已有会话的 session ID（UUID）")
	flags.StringVar(&config.cwd, "cwd", "", "Claude 会话的工作目录")
	flags.StringVar(&config.permissionMode, "permission-mode", "", "Claude 权限模式: default|acceptEdits|plan|bypassPermissions")
	flags.StringVar(&config.model, "model", "", "Claude 使用的模型")
	flags.IntVar(&config.maxTurns, "max-turns", 0, "Claude 单次连接允许的最大 Turn 数")
	flags.IntVar(&config.maxSessions, "max-sessions", config.maxSessions, "同时运行的 Claude 会话上限")
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
	if err := applyProfile(&config, flags); err != nil {
		return cliConfig{}, err
	}
	if config.resumeSessionID != "" && !isUUID(config.resumeSessionID) {
		return cliConfig{}, errors.New("invalid resume session ID: must be a UUID")
	}
	if isFlagSet(flags, "cwd") {
		if strings.TrimSpace(config.cwd) == "" {
			return cliConfig{}, errors.New("cwd must not be empty")
		}
		absolute, err := filepath.Abs(config.cwd)
		if err != nil {
			return cliConfig{}, fmt.Errorf("invalid cwd: %w", err)
		}
		info, err := os.Stat(absolute)
		if err != nil {
			return cliConfig{}, fmt.Errorf("invalid cwd: %w", err)
		}
		if !info.IsDir() {
			return cliConfig{}, errors.New("cwd must be a directory")
		}
		config.cwd = absolute
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
	if config.permissionMode != "" && !validPermissionModes[config.permissionMode] {
		return cliConfig{}, errors.New("invalid permission mode")
	}
	if config.advertiseHost != "" {
		ip := net.ParseIP(config.advertiseHost)
		if ip == nil || ip.To4() == nil {
			return cliConfig{}, errors.New("advertise host must be an IPv4 address")
		}
	}
	if isFlagSet(flags, "max-turns") && config.maxTurns <= 0 {
		return cliConfig{}, errors.New("max turns must be positive")
	}
	if config.maxSessions <= 0 || config.maxSessions > 32 {
		return cliConfig{}, errors.New("max sessions must be between 1 and 32")
	}
	for _, origin := range extraOrigins {
		if err := validateOrigin(origin); err != nil {
			return cliConfig{}, err
		}
		config.allowedOrigins = append(config.allowedOrigins, origin)
	}
	return config, nil
}

var validPermissionModes = map[string]bool{
	"default":           true,
	"acceptEdits":       true,
	"plan":              true,
	"bypassPermissions": true,
}

func applyProfile(config *cliConfig, flags *flag.FlagSet) error {
	switch config.profile {
	case "local", "debug":
	case "emulator":
		if !isFlagSet(flags, "listen") {
			config.listen = "0.0.0.0:8765"
		}
		if !isFlagSet(flags, "advertise-url") {
			port, err := listenPort(config.listen)
			if err != nil {
				return err
			}
			config.advertiseURL = "ws://10.0.2.2:" + port + "/ws"
		}
		if !contains(config.allowedOrigins, "http://localhost") {
			config.allowedOrigins = append(config.allowedOrigins, "http://localhost")
		}
		// live reload 下壳内页面从 dev server 加载，Origin 为 http://10.0.2.2:5173
		// （10.0.2.2 是模拟器内部 NAT 别名，只指向宿主回环，仅模拟器内可达）。
		// 自定义 Vite 端口时 profile 无法得知端口，仍需显式 --allowed-origin。
		if !contains(config.allowedOrigins, "http://10.0.2.2:5173") {
			config.allowedOrigins = append(config.allowedOrigins, "http://10.0.2.2:5173")
		}
	case "device":
		if !isFlagSet(flags, "listen") {
			config.listen = "0.0.0.0:8765"
		}
		if !isFlagSet(flags, "advertise-url") {
			host := config.advertiseHost
			if host == "" {
				var err error
				host, err = firstAdvertiseHost()
				if err != nil {
					return err
				}
			}
			config.advertiseHost = host
			port, err := listenPort(config.listen)
			if err != nil {
				return err
			}
			config.advertiseURL = "ws://" + net.JoinHostPort(host, port) + "/ws"
		}
		if !contains(config.allowedOrigins, "http://localhost") {
			config.allowedOrigins = append(config.allowedOrigins, "http://localhost")
		}
	default:
		return errors.New("invalid profile: must be local, emulator, device, or debug")
	}
	return nil
}

func listenPort(address string) (string, error) {
	_, port, err := net.SplitHostPort(address)
	if err != nil || port == "" {
		return "", errors.New("invalid listen address")
	}
	return port, nil
}

func firstAdvertiseHost() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("detect device address: %w", err)
	}
	for _, networkInterface := range interfaces {
		if networkInterface.Flags&net.FlagUp == 0 || networkInterface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := networkInterface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			var ip net.IP
			switch value := address.(type) {
			case *net.IPNet:
				ip = value.IP
			case *net.IPAddr:
				ip = value.IP
			}
			if ipv4 := ip.To4(); ipv4 != nil && !ipv4.IsLoopback() {
				return ipv4.String(), nil
			}
		}
	}
	return "", errors.New("cannot detect a reachable IPv4 address; use --advertise-host")
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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
