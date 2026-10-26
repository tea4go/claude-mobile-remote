package transport

import "time"

// ConnectionDiagnostic contains allowlisted metadata, never frames or credentials.
type ConnectionDiagnostic struct {
	Time      time.Time `json:"time"`
	SessionID string    `json:"sessionId"`
	Code      string    `json:"code"`
}

type ConnectionStatus struct {
	OnlineConnections int                    `json:"onlineConnections"`
	Recent            []ConnectionDiagnostic `json:"recent"`
}

func (s *Server) recordConnectionEvent(sessionID, code string) {
	// Unauthenticated session IDs are never copied from arbitrary request data.
	if sessionID != "" {
		if _, ok := s.registry.Get(sessionID); !ok {
			sessionID = ""
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recent = append(s.recent, ConnectionDiagnostic{Time: time.Now().UTC(), SessionID: sessionID, Code: code})
	if len(s.recent) > 100 {
		s.recent = append([]ConnectionDiagnostic(nil), s.recent[len(s.recent)-100:]...)
	}
}

func (s *Server) setSubscribed(c *connection, sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sessionID == "" {
		delete(s.subscribed, c)
	} else {
		s.subscribed[c] = sessionID
	}
}

// ConnectionStatus is a local host API; it is not an HTTP endpoint.
func (s *Server) ConnectionStatus(sessionID string) ConnectionStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := ConnectionStatus{Recent: make([]ConnectionDiagnostic, 0)}
	for _, id := range s.subscribed {
		if id == sessionID {
			status.OnlineConnections++
		}
	}
	for _, event := range s.recent {
		if event.SessionID == "" || event.SessionID == sessionID {
			status.Recent = append(status.Recent, event)
		}
	}
	return status
}
