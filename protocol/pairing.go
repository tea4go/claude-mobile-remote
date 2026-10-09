package protocol

import (
	"encoding/base64"
	"encoding/json"
	"time"
)

// PayloadVersion is the pairing payload schema version.
const PayloadVersion = 1

// Pairing is the platform-neutral pairing data shown to an authorized user.
type Pairing struct {
	Version      int       `json:"version"`
	WSURL        string    `json:"wsUrl"`
	SessionID    string    `json:"sessionId"`
	PairToken    string    `json:"pairToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
	Capabilities []string  `json:"capabilities,omitempty"`
	PCID         string    `json:"pcId,omitempty"`
	PCName       string    `json:"pcName,omitempty"`
}

// Marshal encodes the pairing payload without logging or decorating it.
func (p Pairing) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

// EncodePairingFragment encodes a validated pairing payload in the Mobile Web
// URL fragment format. The fragment is intentionally kept out of HTTP request
// URLs so the pair token is not sent to a web server as a query parameter.
func EncodePairingFragment(p Pairing) string {
	data, _ := json.Marshal(p)
	return "#pair=" + base64.RawURLEncoding.EncodeToString(data)
}
