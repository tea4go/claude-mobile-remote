package main

import (
	"errors"
	"flag"
	"io"
	"net"
	"net/url"
	"strings"
	"time"
)

var defaultAllowedOrigins = []string{
	"http://127.0.0.1:5173",
	"http://localhost:5173",
}

type cliConfig struct {
	showVersion    bool
	listen         string
	advertiseURL   string
	pairTokenTTL   time.Duration
	allowedOrigins []string
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
		pairTokenTTL: 10 * time.Minute,
	}
	var extraOrigins stringListFlag
	flags := flag.NewFlagSet("claude-mobile-remote", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.BoolVar(&config.showVersion, "version", false, "打印版本并退出")
	flags.StringVar(&config.listen, "listen", config.listen, "WebSocket 监听地址")
	flags.StringVar(&config.advertiseURL, "advertise-url", config.advertiseURL, "客户端可访问的 WebSocket URL")
	flags.DurationVar(&config.pairTokenTTL, "pair-token-ttl", config.pairTokenTTL, "配对凭证有效期")
	flags.Var(&extraOrigins, "allowed-origin", "额外允许的浏览器 Origin，可重复")
	if err := flags.Parse(arguments); err != nil {
		return cliConfig{}, err
	}
	if flags.NArg() != 0 {
		return cliConfig{}, errors.New("unexpected positional arguments")
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
