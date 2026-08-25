package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/tea4go/claude-mobile-remote/internal/channel"
	projectclaude "github.com/tea4go/claude-mobile-remote/internal/claude"
	"github.com/tea4go/claude-mobile-remote/internal/pairing"
	projectsession "github.com/tea4go/claude-mobile-remote/internal/session"
	"github.com/tea4go/claude-mobile-remote/internal/transport"
	"github.com/tea4go/claude-mobile-remote/internal/version"
)

type runtimeBuilder func(cliConfig) (sessionController, remoteService, error)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	exitCode := runMain(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, buildRuntime)
	os.Exit(exitCode)
}

func runMain(ctx context.Context, arguments []string, input io.Reader, output, errorOutput io.Writer, builder runtimeBuilder) int {
	config, err := parseConfig(arguments)
	if err != nil {
		fmt.Fprintf(errorOutput, "配置错误: %v\n", err)
		return 2
	}
	if config.showVersion {
		fmt.Fprintln(output, version.Version)
		return 0
	}
	controller, remote, err := builder(config)
	if err != nil {
		fmt.Fprintf(errorOutput, "初始化错误: %v\n", err)
		return 1
	}
	return runWithRemote(ctx, input, output, errorOutput, controller, remote)
}

func buildRuntime(config cliConfig) (sessionController, remoteService, error) {
	// A5.2 §5：新会话预生成 UUID v4 并经 --session-id 指定为真实 ID；
	// resume 直接使用已有 ID。两者都在创建 Host 前确定。
	var host *projectclaude.Host
	var reservedID string
	if config.resumeSessionID != "" {
		resumed, err := projectclaude.NewHostForResume(config.resumeSessionID)
		if err != nil {
			return nil, nil, err
		}
		host = resumed
		reservedID = config.resumeSessionID
	} else {
		host = projectclaude.NewHost()
		if err := host.PrepareNewSession(); err != nil {
			return nil, nil, err
		}
		reservedID = host.SessionID()
	}
	controller := projectsession.NewController(host, projectsession.WithReservedSessionID(reservedID))
	manager, err := pairing.NewManager(config.advertiseURL, config.pairTokenTTL)
	if err != nil {
		return nil, nil, err
	}
	registry := channel.NewRegistry()
	serverConfig := transport.DefaultConfig()
	serverConfig.ListenAddress = config.listen
	serverConfig.AllowedOrigins = append([]string(nil), config.allowedOrigins...)
	server, err := transport.NewServer(serverConfig, manager, registry)
	if err != nil {
		return nil, nil, err
	}
	remote := newRemoteRuntime(manager, registry, server, controller)
	return controller, remote, nil
}
