package pairing

import "github.com/tea4go/claude-mobile-remote/protocol"

// PayloadVersion is the pairing payload schema version.
const PayloadVersion = protocol.PayloadVersion

// Payload is the platform-neutral pairing data shown to an authorized user.
// It is an alias to protocol.Pairing, the single Go source of truth; the Marshal
// method and JSON tags come from protocol.
type Payload = protocol.Pairing
