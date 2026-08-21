package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	projectclaude "github.com/tea4go/claude-mobile-remote/internal/claude"
)

type turnHost interface {
	Connect(context.Context) error
	RunTurn(context.Context, string, func(projectclaude.Event)) error
	SessionID() string
	Close() error
}

const maxPromptBytes = 1024 * 1024

type scannedLine struct {
	text string
	err  error
}

func scanLines(ctx context.Context, input io.Reader) <-chan scannedLine {
	lines := make(chan scannedLine)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(input)
		scanner.Buffer(make([]byte, 64*1024), maxPromptBytes)
		for scanner.Scan() {
			select {
			case lines <- scannedLine{text: scanner.Text()}:
			case <-ctx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil {
			select {
			case lines <- scannedLine{err: err}:
			case <-ctx.Done():
			}
		}
	}()
	return lines
}

func run(ctx context.Context, input io.Reader, output, errorOutput io.Writer, host turnHost) (exitCode int) {
	defer func() {
		if err := host.Close(); err != nil {
			fmt.Fprintf(errorOutput, "关闭错误: %v\n", err)
			if exitCode == 0 {
				exitCode = 1
			}
		}
	}()

	if err := host.Connect(ctx); err != nil {
		fmt.Fprintf(errorOutput, "启动错误: %v\n", err)
		return 1
	}

	scanCtx, cancelScan := context.WithCancel(ctx)
	defer cancelScan()
	lines := scanLines(scanCtx, input)
	for {
		var line scannedLine
		var ok bool
		select {
		case <-ctx.Done():
			if closer, canClose := input.(io.Closer); canClose {
				_ = closer.Close()
			}
			return 0
		case line, ok = <-lines:
			if !ok {
				return 0
			}
		}
		if line.err != nil {
			fmt.Fprintf(errorOutput, "读取输入错误: %v\n", line.err)
			return 1
		}

		prompt := strings.TrimSpace(line.text)
		if prompt == "" {
			continue
		}
		if prompt == ":quit" {
			return 0
		}

		turnCtx, cancelTurn := context.WithCancel(ctx)
		turnEmittedText := false
		err := host.RunTurn(turnCtx, prompt, func(event projectclaude.Event) {
			switch event.Kind {
			case projectclaude.EventSessionStarted:
				fmt.Fprintf(output, "session: %s\n", event.SessionID)
			case projectclaude.EventText:
				turnEmittedText = true
				fmt.Fprint(output, event.Text)
			}
		})
		cancelTurn()
		if ctx.Err() != nil {
			if turnEmittedText {
				fmt.Fprintln(output)
			}
			return 0
		}
		if err == nil {
			fmt.Fprintln(output)
			continue
		}
		if turnEmittedText {
			fmt.Fprintln(output)
		}
		fmt.Fprintf(errorOutput, "Turn 错误: %v\n", err)
		if projectclaude.ScopeOf(err) == projectclaude.ScopeSession {
			return 1
		}
	}
}
