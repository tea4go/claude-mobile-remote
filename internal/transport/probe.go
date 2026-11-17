package transport

import (
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

// RecheckConnections probes authenticated subscribers after a host network change.
// Ordinary heartbeat settings are unchanged; unrelated network changes do not
// disconnect responsive clients. Concurrent calls coalesce per connection.
func (s *Server) RecheckConnections() {
	s.mu.Lock()
	clients := make([]*connection, 0, len(s.subscribed))
	for c := range s.subscribed {
		clients = append(clients, c)
	}
	s.mu.Unlock()
	for _, c := range clients {
		go c.startProbe(3 * time.Second)
	}
}

func (c *connection) startProbe(timeout time.Duration) {
	c.probeMu.Lock()
	defer c.probeMu.Unlock()
	if c.probeTimer != nil || c.context.Err() != nil {
		return
	}
	c.probeID++
	id := c.probeID
	payload := "network-check:" + strconv.FormatUint(c.probeID, 10)
	c.probeTimer = time.AfterFunc(timeout, func() {
		c.probeMu.Lock()
		defer c.probeMu.Unlock()
		if c.probeTimer == nil || c.probeID != id {
			return
		}
		c.probeTimer = nil
		c.shutdown()
	})
	// WriteControl is safe concurrently with the normal writer and heartbeat.
	if err := c.socket.WriteControl(websocket.PingMessage, []byte(payload), time.Now().Add(time.Second)); err != nil {
		c.probeTimer.Stop()
		c.probeTimer = nil
		c.shutdown()
	}
}

func (c *connection) ackProbe(payload string) {
	c.probeMu.Lock()
	defer c.probeMu.Unlock()
	if c.probeTimer != nil && payload == "network-check:"+strconv.FormatUint(c.probeID, 10) {
		c.probeTimer.Stop()
		c.probeTimer = nil
	}
}

func (c *connection) cancelProbe() {
	c.probeMu.Lock()
	defer c.probeMu.Unlock()
	if c.probeTimer != nil {
		c.probeTimer.Stop()
		c.probeTimer = nil
	}
}
