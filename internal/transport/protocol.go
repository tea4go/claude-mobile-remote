package transport

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	projectsession "github.com/tea4go/claude-mobile-remote/internal/session"
)

const Version = 1

var ErrInvalidMessage = errors.New("invalid protocol message")

const (
	CodeInvalidJSON        = "invalid_json"
	CodeUnsupportedVersion = "unsupported_version"
	CodeInvalidMessage     = "invalid_message"
	CodeInvalidState       = "invalid_state"
	CodeUnauthorized       = "unauthorized"
	CodeMessageTooLarge    = "message_too_large"
	CodeInvalidCommand     = "invalid_command"
	CodeDuplicateCommandID = "duplicate_command_id"
	CodeSessionUnavailable = "session_unavailable"
	CodeQueueFull          = "queue_full"
	CodeInternal           = "internal"
)

// ProtocolError contains a stable client-facing code and a credential-safe message.
type ProtocolError struct {
	Code    string
	Message string
}

func (e *ProtocolError) Error() string {
	return e.Code + ": " + e.Message
}

func (e *ProtocolError) Unwrap() error {
	return ErrInvalidMessage
}

type MessageType string

const (
	TypeHello         MessageType = "hello"
	TypeHelloAck      MessageType = "hello_ack"
	TypeSubscribe     MessageType = "subscribe"
	TypeSubscribed    MessageType = "subscribed"
	TypeUnsubscribe   MessageType = "unsubscribe"
	TypeUnsubscribed  MessageType = "unsubscribed"
	TypeSnapshot      MessageType = "snapshot"
	TypeEvent         MessageType = "event"
	TypeCommand       MessageType = "command"
	TypeCommandResult MessageType = "command_result"
	TypeError         MessageType = "error"
)

type Envelope struct {
	Version   int             `json:"version"`
	Type      MessageType     `json:"type"`
	SessionID string          `json:"sessionId"`
	CommandID string          `json:"commandId"`
	RequestID string          `json:"requestId"`
	Seq       uint64          `json:"seq"`
	Payload   json.RawMessage `json:"payload"`
}

type CommandKind string

const CommandSendText CommandKind = "send_text"

type HelloPayload struct {
	PairToken string `json:"pairToken"`
}

type CommandPayload struct {
	Kind CommandKind `json:"kind"`
	Text string      `json:"text"`
}

type CommandResultPayload struct {
	Accepted bool   `json:"accepted"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type SnapshotPayload struct {
	ConnectionStatus string                  `json:"connectionStatus"`
	Snapshot         projectsession.Snapshot `json:"snapshot"`
}

type EventPayload struct {
	Event projectsession.Event `json:"event"`
}

func NewEnvelope(messageType MessageType, sessionID, commandID, requestID string, seq uint64, payload any) (Envelope, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("encode protocol payload: %w", err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &object); err != nil || object == nil {
		return Envelope{}, ErrInvalidMessage
	}
	return Envelope{Version: Version, Type: messageType, SessionID: sessionID, CommandID: commandID, RequestID: requestID, Seq: seq, Payload: encoded}, nil
}

func EncodeEnvelope(envelope Envelope) ([]byte, error) {
	return json.Marshal(envelope)
}

func DecodeEnvelope(data []byte) (Envelope, error) {
	var envelope Envelope
	if err := decodeStrict(data, &envelope); err != nil {
		return Envelope{}, &ProtocolError{Code: CodeInvalidJSON, Message: "invalid JSON message"}
	}
	return envelope, nil
}

// ValidateEnvelope enforces version-one field and payload invariants.
func ValidateEnvelope(envelope Envelope) error {
	if envelope.Version != Version {
		return protocolError(CodeUnsupportedVersion, "unsupported protocol version")
	}
	if envelope.Type == "" || envelope.SessionID == "" || len(envelope.Payload) == 0 {
		return protocolError(CodeInvalidMessage, "missing required message field")
	}

	switch envelope.Type {
	case TypeCommand:
		if envelope.CommandID == "" || envelope.RequestID != "" || envelope.Seq != 0 {
			return protocolError(CodeInvalidMessage, "invalid command correlation fields")
		}
		var payload CommandPayload
		if err := DecodePayload(envelope, &payload); err != nil || payload.Kind != CommandSendText {
			return protocolError(CodeInvalidMessage, "invalid command payload")
		}
	case TypeHello:
		if envelope.CommandID != "" || envelope.RequestID != "" || envelope.Seq != 0 {
			return protocolError(CodeInvalidMessage, "invalid hello correlation fields")
		}
		var payload HelloPayload
		if err := DecodePayload(envelope, &payload); err != nil || payload.PairToken == "" {
			return protocolError(CodeInvalidMessage, "invalid hello payload")
		}
	case TypeHelloAck, TypeSubscribe, TypeSubscribed, TypeUnsubscribe, TypeUnsubscribed:
		if envelope.CommandID != "" || envelope.RequestID != "" || envelope.Seq != 0 || !isEmptyPayload(envelope) {
			return protocolError(CodeInvalidMessage, "invalid message fields")
		}
	case TypeSnapshot:
		var payload SnapshotPayload
		if envelope.CommandID != "" || envelope.RequestID != "" || DecodePayload(envelope, &payload) != nil ||
			payload.Snapshot.SessionID != envelope.SessionID || payload.Snapshot.Seq != envelope.Seq {
			return protocolError(CodeInvalidMessage, "invalid snapshot message")
		}
	case TypeEvent:
		var payload EventPayload
		if envelope.CommandID != "" || DecodePayload(envelope, &payload) != nil || payload.Event.SessionID != envelope.SessionID ||
			payload.Event.RequestID != envelope.RequestID || payload.Event.Seq != envelope.Seq {
			return protocolError(CodeInvalidMessage, "invalid event message")
		}
	case TypeCommandResult:
		var payload CommandResultPayload
		if envelope.CommandID == "" || envelope.Seq != 0 || DecodePayload(envelope, &payload) != nil {
			return protocolError(CodeInvalidMessage, "invalid command result")
		}
		if payload.Accepted && (envelope.RequestID == "" || payload.Code != "") {
			return protocolError(CodeInvalidMessage, "invalid accepted command result")
		}
		if !payload.Accepted && (envelope.RequestID != "" || payload.Code == "") {
			return protocolError(CodeInvalidMessage, "invalid rejected command result")
		}
	case TypeError:
		var payload ErrorPayload
		if envelope.CommandID != "" || envelope.RequestID != "" || envelope.Seq != 0 || DecodePayload(envelope, &payload) != nil || payload.Code == "" {
			return protocolError(CodeInvalidMessage, "invalid error message")
		}
	default:
		return protocolError(CodeInvalidMessage, "unknown message type")
	}
	return nil
}

func isEmptyPayload(envelope Envelope) bool {
	var payload struct{}
	return DecodePayload(envelope, &payload) == nil
}

func protocolError(code, message string) error {
	return &ProtocolError{Code: code, Message: message}
}

func DecodePayload(envelope Envelope, destination any) error {
	if err := decodeStrict(envelope.Payload, destination); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidMessage, err)
	}
	return nil
}

func decodeStrict(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}
