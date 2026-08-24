package pairing

import (
	"encoding/json"
	"time"
)

const PayloadVersion = 1

// Payload is the platform-neutral pairing data shown to an authorized user.
type Payload struct {
	Version   int       `json:"version"`
	WSURL     string    `json:"wsUrl"`
	SessionID string    `json:"sessionId"`
	PairToken string    `json:"pairToken"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// Marshal encodes the pairing payload without logging or decorating it.
func (p Payload) Marshal() ([]byte, error) {
	return json.Marshal(p)
}
