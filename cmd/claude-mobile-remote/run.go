package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	projectsession "github.com/tea4go/claude-mobile-remote/internal/session"
)

type sessionController interface {
	Start(context.Context) error
	Submit(context.Context, projectsession.Input) (string, error)
	Subscribe(context.Context, int) (projectsession.Snapshot, <-chan projectsession.Event, error)
	WaitIdle(context.Context) (projectsession.Snapshot, error)
	Stop(context.Context) error
}

type scannedLine struct {
	text string
	err  error
}

func scanLines(ctx context.Context, input io.Reader) (<-chan scannedLine, <-chan struct{}) {
	lines := make(chan scannedLine)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer close(lines)
		scanner := bufio.NewScanner(input)
		scanner.Buffer(make([]byte, 64*1024), projectsession.MaxPromptBytes+2)
		for scanner.Scan() {
			text := scanner.Text()
			if len([]byte(strings.TrimSuffix(text, "\r"))) > projectsession.MaxPromptBytes {
				select {
				case lines <- scannedLine{err: errors.New("input token too long")}:
				case <-ctx.Done():
				}
				return
			}
			select {
			case lines <- scannedLine{text: text}:
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
	return lines, done
}

func closeScannerInput(input io.Reader) {
	if closer, ok := input.(io.Closer); ok {
		_ = closer.Close()
	}
}

func eventErrorMessage(event projectsession.Event) string {
	if event.Error == nil {
		return "unknown error"
	}
	if event.Error.Message != "" {
		return event.Error.Message
	}
	if event.Error.Code != "" {
		return event.Error.Code
	}
	return "unknown error"
}

func snapshotErrorMessage(snapshot projectsession.Snapshot) string {
	if snapshot.SessionError == nil {
		return "unknown error"
	}
	if snapshot.SessionError.Message != "" {
		return snapshot.SessionError.Message
	}
	if snapshot.SessionError.Code != "" {
		return snapshot.SessionError.Code
	}
	return "unknown error"
}

func run(ctx context.Context, input io.Reader, output, errorOutput io.Writer, controller sessionController) (exitCode int) {
	defer func() {
		if err := controller.Stop(context.Background()); err != nil {
			fmt.Fprintf(errorOutput, "关闭错误: %v\n", err)
			if exitCode == 0 {
				exitCode = 1
			}
		}
	}()

	if err := controller.Start(ctx); err != nil {
		if ctx.Err() != nil {
			return 0
		}
		fmt.Fprintf(errorOutput, "启动错误: %v\n", err)
		return 1
	}
	if ctx.Err() != nil {
		return 0
	}

	subscriptionContext, cancelSubscription := context.WithCancel(ctx)
	defer cancelSubscription()
	_, events, err := controller.Subscribe(subscriptionContext, 64)
	if err != nil {
		if ctx.Err() != nil {
			return 0
		}
		fmt.Fprintf(errorOutput, "订阅错误: %v\n", err)
		return 1
	}
	if ctx.Err() != nil {
		return 0
	}

	scanCtx, cancelScan := context.WithCancel(ctx)
	defer cancelScan()
	lines, scannerDone := scanLines(scanCtx, input)
	pending := make(map[string]struct{})
	emittedText := make(map[string]bool)
	deferredTurnFailures := make([]projectsession.Event, 0)
	eof := false
	flushDeferredTurnFailures := func() {
		for _, event := range deferredTurnFailures {
			fmt.Fprintf(errorOutput, "Turn 错误: %s\n", eventErrorMessage(event))
		}
		deferredTurnFailures = deferredTurnFailures[:0]
	}
	finishEOF := func() int {
		snapshot, waitErr := controller.WaitIdle(ctx)
		if ctx.Err() != nil {
			return 0
		}
		if waitErr != nil {
			fmt.Fprintf(errorOutput, "结算错误: %v\n", waitErr)
			return 1
		}
		if snapshot.SessionStatus == projectsession.SessionFailed {
			fmt.Fprintf(errorOutput, "Session 错误: %s\n", snapshotErrorMessage(snapshot))
			return 1
		}
		flushDeferredTurnFailures()
		return 0
	}

	for {
		select {
		case <-ctx.Done():
			cancelScan()
			closeScannerInput(input)
			return 0
		case line, ok := <-lines:
			if ctx.Err() != nil {
				cancelScan()
				closeScannerInput(input)
				return 0
			}
			if !ok {
				lines = nil
				eof = true
				if len(pending) == 0 {
					return finishEOF()
				}
				continue
			}
			if line.err != nil {
				if ctx.Err() != nil {
					cancelScan()
					closeScannerInput(input)
					return 0
				}
				fmt.Fprintf(errorOutput, "读取输入错误: %v\n", line.err)
				return 1
			}

			prompt := strings.TrimSpace(line.text)
			if prompt == "" {
				continue
			}
			if prompt == ":quit" {
				cancelScan()
				closeScannerInput(input)
				return 0
			}

			requestID, submitErr := controller.Submit(ctx, projectsession.Input{
				Source: projectsession.InputTerminal,
				Text:   prompt,
			})
			if submitErr != nil {
				if ctx.Err() != nil {
					cancelScan()
					closeScannerInput(input)
					return 0
				}
				fmt.Fprintf(errorOutput, "输入拒绝: %v\n", submitErr)
				if errors.Is(submitErr, projectsession.ErrSessionUnavailable) {
					cancelScan()
					closeScannerInput(input)
					return 1
				}
				continue
			}
			pending[requestID] = struct{}{}
		case event, ok := <-events:
			if ctx.Err() != nil {
				cancelScan()
				closeScannerInput(input)
				return 0
			}
			if !eof {
				select {
				case <-scannerDone:
					eof = true
					lines = nil
				default:
				}
			}
			if !ok {
				cancelScan()
				if !eof {
					closeScannerInput(input)
				}
				if ctx.Err() != nil {
					return 0
				}
				if len(pending) > 0 {
					fmt.Fprintf(errorOutput, "事件流错误: 已关闭，仍有 %d 个请求未完成\n", len(pending))
					return 1
				}
				return 0
			}

			switch event.Kind {
			case projectsession.EventSessionStarted:
				fmt.Fprintf(output, "session: %s\n", event.SessionID)
			case projectsession.EventTurnStarted:
				flushDeferredTurnFailures()
			case projectsession.EventTextDelta:
				fmt.Fprint(output, event.Text)
				if event.Text != "" {
					emittedText[event.RequestID] = true
				}
			case projectsession.EventTurnCompleted:
				delete(pending, event.RequestID)
				fmt.Fprintln(output)
				delete(emittedText, event.RequestID)
				if eof && len(pending) == 0 {
					return finishEOF()
				}
			case projectsession.EventTurnFailed:
				delete(pending, event.RequestID)
				if emittedText[event.RequestID] {
					fmt.Fprintln(output)
				}
				delete(emittedText, event.RequestID)
				deferredTurnFailures = append(deferredTurnFailures, event)
				if len(pending) == 0 {
					snapshot, waitErr := controller.WaitIdle(ctx)
					if ctx.Err() != nil {
						return 0
					}
					if waitErr != nil {
						fmt.Fprintf(errorOutput, "结算错误: %v\n", waitErr)
						return 1
					}
					if snapshot.SessionStatus == projectsession.SessionFailed {
						deferredTurnFailures = deferredTurnFailures[:0]
						fmt.Fprintf(errorOutput, "Session 错误: %s\n", snapshotErrorMessage(snapshot))
						return 1
					}
					flushDeferredTurnFailures()
					if eof {
						return 0
					}
				}
			case projectsession.EventSessionFailed:
				cancelScan()
				if !eof {
					closeScannerInput(input)
				}
				if ctx.Err() != nil {
					return 0
				}
				hasPartialLine := false
				for _, hasText := range emittedText {
					if hasText {
						hasPartialLine = true
						break
					}
				}
				if hasPartialLine {
					fmt.Fprintln(output)
				}
				clear(emittedText)
				deferredTurnFailures = deferredTurnFailures[:0]
				fmt.Fprintf(errorOutput, "Session 错误: %s\n", eventErrorMessage(event))
				return 1
			case projectsession.EventSessionStopped:
				cancelScan()
				if !eof {
					closeScannerInput(input)
				}
				return 0
			}
		}
	}
}
