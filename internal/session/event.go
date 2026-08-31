package session

import "github.com/tea4go/claude-mobile-remote/protocol"

type (
	EventKind       = protocol.EventKind
	UnsupportedCode = protocol.UnsupportedCode
	Event           = protocol.Event
)

const (
	EventSessionStarted = protocol.EventSessionStarted
	EventTurnStarted    = protocol.EventTurnStarted
	EventTextDelta      = protocol.EventTextDelta
	EventTurnCompleted  = protocol.EventTurnCompleted
	EventTurnFailed     = protocol.EventTurnFailed
	EventSessionFailed  = protocol.EventSessionFailed
	EventSessionStopped = protocol.EventSessionStopped
	EventContentBlock   = protocol.EventContentBlock
	EventActionRequired = protocol.EventActionRequired
	EventActionResolved = protocol.EventActionResolved
	EventUnsupported    = protocol.EventUnsupported

	UnsupportedUnknownBlock     = protocol.UnsupportedUnknownBlock
	UnsupportedUnknownSubtype   = protocol.UnsupportedUnknownSubtype
	UnsupportedConcurrentAction = protocol.UnsupportedConcurrentAction
	UnsupportedTextDisabled     = protocol.UnsupportedTextDisabled
)
