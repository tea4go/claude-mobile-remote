package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	projectsession "github.com/tea4go/claude-mobile-remote/internal/session"
	"github.com/tea4go/claude-mobile-remote/internal/version"
)

type managedEvent struct {
	sessionID string
	event     projectsession.Event
	closed    bool
}

func runManagedMain(ctx context.Context, arguments []string, input io.Reader, output, errorOutput io.Writer) int {
	config, err := parseConfig(arguments)
	if err != nil {
		fmt.Fprintf(errorOutput, "配置错误: %v\n", err)
		return 2
	}
	if config.showVersion {
		fmt.Fprintln(output, version.Version)
		return 0
	}
	manager, err := newManagedCLI(config)
	if err != nil {
		fmt.Fprintf(errorOutput, "初始化错误: %v\n", err)
		return 1
	}
	return runManaged(ctx, input, output, errorOutput, manager)
}

func runManaged(ctx context.Context, input io.Reader, output, errorOutput io.Writer, manager *managedCLI) (exitCode int) {
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := manager.Close(closeCtx); err != nil {
			fmt.Fprintf(errorOutput, "关闭错误: %v\n", err)
			if exitCode == 0 {
				exitCode = 1
			}
		}
	}()
	first, err := manager.Start(ctx)
	if err != nil {
		fmt.Fprintf(errorOutput, "启动错误: %v\n", err)
		return 1
	}
	if err := writePairingOutput(output, first); err != nil {
		fmt.Fprintln(errorOutput, "配对信息输出错误")
		return 1
	}
	fmt.Fprintln(output, "管理命令: :sessions, :use <ID>, :new, :stop [ID], :pair [ID], :quit")

	selected := first.SessionID
	pending := make(map[string]string)
	touched := make(map[string]struct{})
	eof := false
	hadFailure := false
	finishEOF := func() int {
		if ctx.Err() != nil {
			return 0
		}
		for id := range touched {
			controller, _, ok := manager.Session(id)
			if !ok {
				hadFailure = true
				continue
			}
			snapshot, err := controller.WaitIdle(ctx)
			if ctx.Err() != nil {
				return 0
			}
			if err != nil || snapshot.SessionStatus == projectsession.SessionFailed {
				hadFailure = true
			}
		}
		if hadFailure {
			return 1
		}
		return 0
	}
	attached := make(map[string]struct{})
	events := make(chan managedEvent, 128)
	attach := func(sessionID string) {
		if _, exists := attached[sessionID]; exists {
			return
		}
		controller, _, ok := manager.Session(sessionID)
		if !ok {
			return
		}
		subscriptionCtx, cancel := context.WithCancel(ctx)
		_, source, err := controller.Subscribe(subscriptionCtx, 128)
		if err != nil {
			cancel()
			return
		}
		attached[sessionID] = struct{}{}
		go func() {
			defer cancel()
			for event := range source {
				select {
				case events <- managedEvent{sessionID: sessionID, event: event}:
				case <-ctx.Done():
					return
				}
			}
			select {
			case events <- managedEvent{sessionID: sessionID, closed: true}:
			case <-ctx.Done():
			}
		}()
	}
	attach(selected)
	scanCtx, cancelScan := context.WithCancel(ctx)
	defer cancelScan()
	defer closeScannerInput(input)
	lines, _ := scanLines(scanCtx, input)
	for {
		select {
		case <-ctx.Done():
			cancelScan()
			closeScannerInput(input)
			return 0
		case notice := <-events:
			if notice.closed {
				for requestID, id := range pending {
					if id == notice.sessionID {
						delete(pending, requestID)
						hadFailure = true
					}
				}
				if eof && len(pending) == 0 {
					return finishEOF()
				}
				continue
			}
			switch notice.event.Kind {
			case projectsession.EventTurnCompleted, projectsession.EventTurnFailed:
				delete(pending, notice.event.RequestID)
			}
			if notice.sessionID != selected {
				if eof && len(pending) == 0 {
					return finishEOF()
				}
				continue
			}
			switch notice.event.Kind {
			case projectsession.EventSessionStarted:
				fmt.Fprintf(output, "session: %s\n", notice.sessionID)
			case projectsession.EventTextDelta:
				fmt.Fprint(output, notice.event.Text)
			case projectsession.EventTurnCompleted:
				fmt.Fprintln(output)
			case projectsession.EventTurnFailed, projectsession.EventSessionFailed:
				hadFailure = true
				fmt.Fprintf(errorOutput, "%s 错误: %s\n", notice.sessionID, eventErrorMessage(notice.event))
			case projectsession.EventSessionStopped:
				fmt.Fprintf(output, "session stopped: %s\n", notice.sessionID)
			}
			if eof && len(pending) == 0 {
				return finishEOF()
			}
		case payload := <-manager.Pairings():
			attach(payload.SessionID)
			if err := writePairingOutput(output, payload); err != nil {
				fmt.Fprintln(errorOutput, "配对信息输出错误")
				return 1
			}
		case line, ok := <-lines:
			if !ok {
				lines = nil
				eof = true
				if len(pending) == 0 {
					return finishEOF()
				}
				continue
			}
			if line.err != nil {
				fmt.Fprintf(errorOutput, "读取输入错误: %v\n", line.err)
				return 1
			}
			prompt := strings.TrimSpace(line.text)
			if prompt == "" {
				continue
			}
			if strings.HasPrefix(prompt, "::") {
				prompt = prompt[1:]
			} else if strings.HasPrefix(prompt, ":") {
				if prompt == ":quit" {
					return 0
				}
				if err := runManagedCommand(ctx, manager, prompt, &selected, attach, output); err != nil {
					fmt.Fprintf(errorOutput, "管理命令错误: %v\n", err)
				}
				continue
			}
			controller, _, ok := manager.Session(selected)
			if !ok {
				fmt.Fprintln(errorOutput, "当前会话不可用")
				continue
			}
			if requestID, err := controller.Submit(ctx, projectsession.Input{Source: projectsession.InputTerminal, Text: prompt}); err != nil {
				fmt.Fprintf(errorOutput, "输入拒绝: %v\n", err)
			} else {
				pending[requestID] = selected
				touched[selected] = struct{}{}
			}
		}
	}
}

func runManagedCommand(ctx context.Context, manager *managedCLI, command string, selected *string, attach func(string), output io.Writer) error {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return nil
	}
	switch fields[0] {
	case ":sessions":
		if len(fields) != 1 {
			return fmt.Errorf("usage: :sessions")
		}
		for _, id := range manager.SessionIDs() {
			controller, _, _ := manager.Session(id)
			snapshot, err := controller.Snapshot(ctx)
			if err != nil {
				return err
			}
			marker := " "
			if id == *selected {
				marker = "*"
			}
			fmt.Fprintf(output, "%s %s %s\n", marker, id, snapshot.SessionStatus)
		}
	case ":use":
		if len(fields) != 2 {
			return fmt.Errorf("usage: :use <session ID or unique prefix>")
		}
		id, err := resolveManagedID(manager, fields[1])
		if err != nil {
			return err
		}
		*selected = id
		attach(id)
		fmt.Fprintf(output, "selected: %s\n", id)
	case ":new":
		if len(fields) != 1 {
			return fmt.Errorf("usage: :new")
		}
		commandID, err := randomSessionID()
		if err != nil {
			return err
		}
		payload, err := manager.CreateSession(ctx, *selected, commandID)
		if err != nil {
			return err
		}
		*selected = payload.SessionID
		attach(payload.SessionID)
		fmt.Fprintf(output, "selected: %s\n", payload.SessionID)
	case ":stop", ":pair":
		if len(fields) > 2 {
			return fmt.Errorf("usage: %s [session ID or unique prefix]", fields[0])
		}
		id := *selected
		if len(fields) == 2 {
			var err error
			id, err = resolveManagedID(manager, fields[1])
			if err != nil {
				return err
			}
		}
		if fields[0] == ":stop" {
			if err := manager.StopSession(ctx, id); err != nil {
				return err
			}
			fmt.Fprintf(output, "stopped: %s\n", id)
		} else {
			_, payload, ok := manager.Session(id)
			if !ok {
				return fmt.Errorf("session not found")
			}
			return writePairingOutput(output, payload)
		}
	default:
		return fmt.Errorf("unknown command; use :sessions, :use, :new, :stop, :pair or :quit")
	}
	return nil
}

func resolveManagedID(manager *managedCLI, query string) (string, error) {
	matched := ""
	for _, id := range manager.SessionIDs() {
		if strings.HasPrefix(id, query) {
			if matched != "" {
				return "", fmt.Errorf("session ID prefix is ambiguous")
			}
			matched = id
		}
	}
	if matched == "" {
		return "", fmt.Errorf("session not found")
	}
	return matched, nil
}
