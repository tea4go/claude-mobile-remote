package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tea4go/claude-mobile-remote/internal/pairing"
	"github.com/tea4go/claude-mobile-remote/internal/transport"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(runClient(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func runClient(ctx context.Context, arguments []string, input io.Reader, output, errorOutput io.Writer) int {
	payload, err := parsePairing(arguments)
	if err != nil {
		fmt.Fprintln(errorOutput, "配对信息无效")
		return 2
	}
	connection, _, err := websocket.DefaultDialer.DialContext(ctx, payload.WSURL, nil)
	if err != nil {
		fmt.Fprintf(errorOutput, "连接错误: %v\n", err)
		return 1
	}
	defer connection.Close()

	if err := writeEnvelope(connection, transport.TypeHello, payload.SessionID, "", transport.HelloPayload{PairToken: payload.PairToken}); err != nil {
		fmt.Fprintln(errorOutput, "鉴权消息发送失败")
		return 1
	}
	if envelope, err := readExpected(connection, transport.TypeHelloAck); err != nil || envelope.SessionID != payload.SessionID {
		fmt.Fprintln(errorOutput, "鉴权失败")
		return 1
	}
	fmt.Fprintln(output, "authenticated")
	if err := writeEnvelope(connection, transport.TypeSubscribe, payload.SessionID, "", struct{}{}); err != nil {
		fmt.Fprintln(errorOutput, "订阅消息发送失败")
		return 1
	}
	if _, err := readExpected(connection, transport.TypeSubscribed); err != nil {
		fmt.Fprintln(errorOutput, "订阅失败")
		return 1
	}
	fmt.Fprintf(output, "subscribed session=%s\n", payload.SessionID)
	snapshot, err := readExpected(connection, transport.TypeSnapshot)
	if err != nil {
		fmt.Fprintln(errorOutput, "snapshot 接收失败")
		return 1
	}
	var snapshotPayload transport.SnapshotPayload
	if err := transport.DecodePayload(snapshot, &snapshotPayload); err != nil {
		fmt.Fprintln(errorOutput, "snapshot 无效")
		return 1
	}
	fmt.Fprintf(output, "snapshot seq=%d state=%s\n", snapshot.Seq, snapshotPayload.Snapshot.SessionStatus)

	clientContext, cancel := context.WithCancel(ctx)
	defer cancel()
	outbound := make(chan transport.Envelope, 16)
	writeErrors := make(chan error, 1)
	go clientWriteLoop(clientContext, connection, outbound, writeErrors)
	readResults := make(chan clientReadResult, 16)
	go clientReadLoop(clientContext, connection, readResults)
	lines := make(chan string)
	go scanClientLines(clientContext, input, lines)
	pending := make(map[string]struct{})

	for {
		select {
		case <-ctx.Done():
			return 0
		case err := <-writeErrors:
			fmt.Fprintf(errorOutput, "发送错误: %v\n", err)
			return 1
		case line, ok := <-lines:
			if !ok {
				lines = nil
				continue
			}
			text := strings.TrimSpace(line)
			if text == "" {
				continue
			}
			if text == ":quit" {
				return 0
			}
			commandID, err := randomCommandID()
			if err != nil {
				fmt.Fprintln(errorOutput, "命令 ID 生成失败")
				return 1
			}
			envelope, err := transport.NewEnvelope(transport.TypeCommand, payload.SessionID, commandID, "", 0, transport.CommandPayload{Kind: transport.CommandSendText, Text: text})
			if err != nil {
				fmt.Fprintln(errorOutput, "命令编码失败")
				return 1
			}
			pending[commandID] = struct{}{}
			select {
			case outbound <- envelope:
			case <-ctx.Done():
				return 0
			}
		case readResult := <-readResults:
			if readResult.err != nil {
				if websocket.IsCloseError(readResult.err, websocket.CloseNormalClosure) && len(pending) == 0 {
					return 0
				}
				fmt.Fprintln(errorOutput, "连接已断开")
				return 1
			}
			envelope := readResult.envelope
			switch envelope.Type {
			case transport.TypeCommandResult:
				var result transport.CommandResultPayload
				if transport.DecodePayload(envelope, &result) != nil {
					fmt.Fprintln(errorOutput, "命令结果无效")
					return 1
				}
				delete(pending, envelope.CommandID)
				if result.Accepted {
					fmt.Fprintf(output, "command accepted commandId=%s requestId=%s\n", envelope.CommandID, envelope.RequestID)
				} else {
					fmt.Fprintf(output, "command rejected commandId=%s code=%s\n", envelope.CommandID, result.Code)
				}
			case transport.TypeEvent:
				var event transport.EventPayload
				if transport.DecodePayload(envelope, &event) != nil {
					fmt.Fprintln(errorOutput, "事件无效")
					return 1
				}
				fmt.Fprintf(output, "event seq=%d type=%s requestId=%s\n", envelope.Seq, event.Event.Kind, envelope.RequestID)
			case transport.TypeSnapshot:
				fmt.Fprintf(output, "snapshot seq=%d\n", envelope.Seq)
			case transport.TypeError:
				var protocolError transport.ErrorPayload
				_ = transport.DecodePayload(envelope, &protocolError)
				fmt.Fprintf(errorOutput, "协议错误: %s\n", protocolError.Code)
				return 1
			}
		}
	}
}

type clientReadResult struct {
	envelope transport.Envelope
	err      error
}

func parsePairing(arguments []string) (pairing.Payload, error) {
	flags := flag.NewFlagSet("channel-test-client", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	raw := flags.String("pair", "", "配对 JSON")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 || *raw == "" {
		return pairing.Payload{}, errors.New("invalid arguments")
	}
	var payload pairing.Payload
	decoder := json.NewDecoder(bytes.NewBufferString(*raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return pairing.Payload{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return pairing.Payload{}, errors.New("trailing pairing data")
	}
	parsedURL, err := url.Parse(payload.WSURL)
	if err != nil || (parsedURL.Scheme != "ws" && parsedURL.Scheme != "wss") || parsedURL.Host == "" ||
		payload.Version != pairing.PayloadVersion || payload.SessionID == "" || payload.PairToken == "" || !time.Now().Before(payload.ExpiresAt) {
		return pairing.Payload{}, errors.New("invalid pairing payload")
	}
	return payload, nil
}

func writeEnvelope(connection *websocket.Conn, messageType transport.MessageType, sessionID, commandID string, payload any) error {
	envelope, err := transport.NewEnvelope(messageType, sessionID, commandID, "", 0, payload)
	if err != nil {
		return err
	}
	data, err := transport.EncodeEnvelope(envelope)
	if err != nil {
		return err
	}
	return connection.WriteMessage(websocket.TextMessage, data)
}

func readExpected(connection *websocket.Conn, expected transport.MessageType) (transport.Envelope, error) {
	_, data, err := connection.ReadMessage()
	if err != nil {
		return transport.Envelope{}, err
	}
	envelope, err := transport.DecodeEnvelope(data)
	if err != nil {
		return transport.Envelope{}, err
	}
	if err := transport.ValidateEnvelope(envelope); err != nil {
		return transport.Envelope{}, err
	}
	if envelope.Type != expected {
		return transport.Envelope{}, fmt.Errorf("unexpected message type %q", envelope.Type)
	}
	return envelope, nil
}

func clientWriteLoop(ctx context.Context, connection *websocket.Conn, outbound <-chan transport.Envelope, errorsChannel chan<- error) {
	for {
		select {
		case <-ctx.Done():
			return
		case envelope := <-outbound:
			data, err := transport.EncodeEnvelope(envelope)
			if err == nil {
				err = connection.WriteMessage(websocket.TextMessage, data)
			}
			if err != nil {
				select {
				case errorsChannel <- err:
				case <-ctx.Done():
				}
				return
			}
		}
	}
}

func clientReadLoop(ctx context.Context, connection *websocket.Conn, results chan<- clientReadResult) {
	for {
		_, data, err := connection.ReadMessage()
		var envelope transport.Envelope
		if err == nil {
			envelope, err = transport.DecodeEnvelope(data)
			if err == nil {
				err = transport.ValidateEnvelope(envelope)
			}
		}
		select {
		case results <- clientReadResult{envelope: envelope, err: err}:
		case <-ctx.Done():
			return
		}
		if err != nil {
			return
		}
	}
}

func scanClientLines(ctx context.Context, input io.Reader, lines chan<- string) {
	defer close(lines)
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		select {
		case lines <- scanner.Text():
		case <-ctx.Done():
			return
		}
	}
}

func randomCommandID() (string, error) {
	var raw [16]byte
	if _, err := io.ReadFull(rand.Reader, raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}
