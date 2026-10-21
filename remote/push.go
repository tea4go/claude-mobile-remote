package remote

import (
	"context"
	"github.com/tea4go/claude-mobile-remote/protocol"
	"github.com/tea4go/claude-mobile-remote/push"
)

func (s *Server) startPush() {
	if s.pushSender == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.pushCancel = cancel
	s.pushDone = make(chan struct{})
	go func() {
		defer close(s.pushDone)
		push.Watch(ctx, s.controller, s.pushSender, func() (protocol.Pairing, bool) {
			s.mu.Lock()
			defer s.mu.Unlock()
			return s.pushPairing, true
		})
	}()
}
