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
	host, reservedID, err := buildHost(config)
	if err != nil {
		return nil, nil, err
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

// buildHost 选择 Claude 接入实现，并返回会话的预留 ID。
// A5.2 §5：新会话预生成 UUID v4 并经 --session-id 指定为真实 ID；
// resume 直接使用已有 ID。两者都在创建 Host 前确定。
func buildHost(config cliConfig) (projectsession.Host, string, error) {
	if config.fakeHostScenario != "" {
		// 桩模式只替换 Claude 接入层：会话、通道、传输和配对全部走同一套代码路径。
		fake, err := projectclaude.NewFakeHost(config.fakeHostScenario)
		if err != nil {
			return nil, "", err
		}
		return fake, fake.SessionID(), nil
	}
	if config.resumeSessionID != "" {
		resumed, err := projectclaude.NewHostForResume(config.resumeSessionID)
		if err != nil {
			return nil, "", err
		}
		return resumed, config.resumeSessionID, nil
	}
	host := projectclaude.NewHost()
	if err := host.PrepareNewSession(); err != nil {
		return nil, "", err
	}
	return host, host.SessionID(), nil
}
