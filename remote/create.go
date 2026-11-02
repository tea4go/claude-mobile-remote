package remote

import (
	"context"
	"errors"

	"github.com/tea4go/claude-mobile-remote/internal/transport"
	"github.com/tea4go/claude-mobile-remote/protocol"
)

// SessionCreator creates a host-owned Session from a subscribed source. Hosts
// must bind the working directory themselves and deduplicate commandID retries.
// A returned credential is delivered only to the requesting connection.
type SessionCreator interface {
	CreateSession(context.Context, string, string) (protocol.Pairing, error)
}

var (
	ErrSessionCapacity   = errors.New("session capacity reached")
	ErrCreateRateLimited = errors.New("create requests are too frequent")
)

type sessionCreationAdapter struct {
	creator   SessionCreator
	sessionID string
}

func (a sessionCreationAdapter) CreateSession(ctx context.Context, source, commandID string) (protocol.Pairing, error) {
	if source != a.sessionID {
		return protocol.Pairing{}, protocol.ErrSessionMismatch
	}
	pairing, err := a.creator.CreateSession(ctx, source, commandID)
	if err == nil {
		return pairing, nil
	}
	code, message := transport.CodeInternal, "session creation failed"
	switch {
	case errors.Is(err, ErrSessionCapacity):
		code, message = "session_capacity", "session capacity reached"
	case errors.Is(err, ErrCreateRateLimited):
		code, message = "create_rate_limited", "create requests are too frequent"
	case errors.Is(err, protocol.ErrSessionUnavailable):
		code, message = transport.CodeSessionUnavailable, "session unavailable"
	}
	return protocol.Pairing{}, &transport.ProtocolError{Code: code, Message: message}
}

// Creation does not grant the right to stop a host-owned Session.
func (a sessionCreationAdapter) StopSession(context.Context, string) error {
	return &transport.ProtocolError{Code: transport.CodeSessionUnavailable, Message: "operation unavailable"}
}

func (s *Server) withCapabilities(p protocol.Pairing) protocol.Pairing {
	if s.canCreate {
		p.Capabilities = []string{"create_session"}
	}
	return p
}
