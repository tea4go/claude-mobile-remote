package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	projectclaude "github.com/tea4go/claude-mobile-remote/internal/claude"
	"github.com/tea4go/claude-mobile-remote/internal/pairing"
	projectsession "github.com/tea4go/claude-mobile-remote/internal/session"
	"github.com/tea4go/claude-mobile-remote/internal/version"
	remoteserver "github.com/tea4go/claude-mobile-remote/remote"
)

type runtimeBuilder func(cliConfig) (sessionController, remoteService, error)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	exitCode := runManagedMain(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
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
	if config.pcID == "" {
		var err error
		if config.fakeHostScenario == "" {
			config.pcID, err = defaultPCID()
		} else {
			config.pcID, err = randomPCID()
		}
		if err != nil {
			return nil, nil, err
		}
	}
	host, reservedID, err := buildHost(config)
	if err != nil {
		return nil, nil, err
	}
	controller := projectsession.NewController(host, projectsession.WithReservedSessionID(reservedID))

	// 桩模式的 Session ID 是固定常量，多个 --fake-host 进程会共用同一个凭证文件，
	// 因此只有真实 Session 才启用跨重启恢复。
	var pairingFile string
	if config.fakeHostScenario == "" && reservedID != "" {
		pairingFile, err = defaultPairingFile(reservedID)
		if err != nil {
			return nil, nil, err
		}
		// 已过期的凭证不再能授权任何客户端，启动时顺手清理，避免目录只增不减。
		_ = pairing.PruneExpired(filepath.Dir(pairingFile), time.Now())
	}

	remoteServer, err := remoteserver.New(remoteserver.Config{
		SessionID:              reservedID,
		Controller:             controller,
		ListenAddress:          config.listen,
		AdvertiseURL:           config.advertiseURL,
		PairTokenTTL:           config.pairTokenTTL,
		PairingPersistenceFile: pairingFile,
		PCID:                   config.pcID,
		PCName:                 cliHostName(),
		AllowedOrigins:         append([]string(nil), config.allowedOrigins...),
	})
	if err != nil {
		return nil, nil, err
	}
	return controller, remoteServer, nil
}

func defaultPairingFile(sessionID string) (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(configDir, "claude-mobile-remote", "pairings", sessionID+".json"), nil
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
		resumed, err := projectclaude.NewHostForResume(config.resumeSessionID, hostOptions(config)...)
		if err != nil {
			return nil, "", err
		}
		return resumed, config.resumeSessionID, nil
	}
	host := projectclaude.NewHost(hostOptions(config)...)
	if err := host.PrepareNewSession(); err != nil {
		return nil, "", err
	}
	return host, host.SessionID(), nil
}

func hostOptions(config cliConfig) []projectclaude.Option {
	options := make([]projectclaude.Option, 0, 4)
	if config.cwd != "" {
		options = append(options, projectclaude.WithCwd(config.cwd))
	}
	if config.permissionMode != "" {
		options = append(options, projectclaude.WithPermissionMode(config.permissionMode))
	}
	if config.model != "" {
		options = append(options, projectclaude.WithModel(config.model))
	}
	if config.maxTurns > 0 {
		options = append(options, projectclaude.WithMaxTurns(config.maxTurns))
	}
	return options
}
