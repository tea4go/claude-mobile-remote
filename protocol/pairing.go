package protocol

import (
	"encoding/json"
	"time"
)

// PayloadVersion is the pairing payload schema version.
const PayloadVersion = 1

// Pairing is the platform-neutral pairing data shown to an authorized user.
type Pairing struct {
	Version   int       `json:"version"`
	WSURL     string    `json:"wsUrl"`
	SessionID string    `json:"sessionId"`
	PairToken string    `json:"pairToken"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// Marshal encodes the pairing payload without logging or decorating it.
func (p Pairing) Marshal() ([]byte, error) {
	return json.Marshal(p)
}
