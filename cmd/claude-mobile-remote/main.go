package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	projectclaude "github.com/tea4go/claude-mobile-remote/internal/claude"
	projectsession "github.com/tea4go/claude-mobile-remote/internal/session"
	"github.com/tea4go/claude-mobile-remote/internal/version"
)

func main() {
	showVersion := flag.Bool("version", false, "打印版本并退出")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.Version)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	controller := projectsession.NewController(projectclaude.NewHost())
	exitCode := run(ctx, os.Stdin, os.Stdout, os.Stderr, controller)
	stop()
	os.Exit(exitCode)
}
