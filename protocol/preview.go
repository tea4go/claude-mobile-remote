package protocol

import "unicode/utf8"

// DefaultToolPreviewMaxRunes is the B2 §3.1 preview truncation default upper
// bound. The runtime limit is supplied by ServiceConfig.ToolPreviewMaxRunes; this
// only provides the default and the invalid-value fallback so the limit never
// scatters into unrelated constants.
const DefaultToolPreviewMaxRunes = 200

// previewEllipsis marks a truncated preview; it is a single rune so tests can
// count by rune.
const previewEllipsis = '…'

// TruncatePreview truncates a display string to maxRunes runes on a rune
// boundary, appending one ellipsis when truncated. It is the single truncation
// implementation for both tool-approval previews (B2 §4.3) and queued-item
// previews (§4.4), guaranteeing identical semantics and always-valid UTF-8.
func TruncatePreview(text string, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = DefaultToolPreviewMaxRunes
	}
	if utf8.RuneCountInString(text) <= maxRunes {
		return text
	}
	kept := make([]rune, 0, maxRunes+1)
	for _, run := range text {
		if len(kept) == maxRunes {
			break
		}
		kept = append(kept, run)
	}
	return string(append(kept, previewEllipsis))
}
