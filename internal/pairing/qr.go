package pairing

import (
	"github.com/skip2/go-qrcode"
	"github.com/tea4go/claude-mobile-remote/protocol"
)

// RenderTerminalQR renders a pairing Fragment for display in a UTF-8 terminal.
// It does not persist or otherwise expose the pairing payload outside the QR
// graphic.
func RenderTerminalQR(payload protocol.Pairing) (string, error) {
	content := protocol.EncodePairingFragment(payload)
	qr, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return "", err
	}
	// The terminal output already has surrounding whitespace from the prompt
	// and the following text. Avoid duplicating the library's four-module quiet
	// zone, which makes the QR unnecessarily large in a text terminal.
	qr.DisableBorder = true
	return qr.ToSmallString(false), nil
}
