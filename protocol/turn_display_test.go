package protocol

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTurnDisplayOrderBoundsAndCopy(t *testing.T) {
	var d *TurnDisplay
	d = AppendTurnDisplay(d, Event{Kind: EventTurnStarted})
	d = AppendTurnDisplay(d, Event{Kind: EventTextDelta, Text: "一"})
	d = AppendTurnDisplay(d, Event{Kind: EventContentBlock, Block: &ContentBlock{ID: "b", Kind: BlockThinking, Thinking: "思考"}})
	d = AppendTurnDisplay(d, Event{Kind: EventTextDelta, Text: "二"})
	if len(d.Items) != 3 || d.Items[0].Text != "一" || d.Items[1].Block.ID != "b" || d.Items[2].Text != "二" {
		t.Fatalf("order: %#v", d)
	}
	copy := CloneTurnDisplay(d)
	copy.Items[1].Block.ID = "changed"
	if d.Items[1].Block.ID != "b" {
		t.Fatal("copy shares block")
	}
	d = AppendTurnDisplay(d, Event{Kind: EventTextDelta, Text: strings.Repeat("界", MaxTurnDisplayBytes)})
	data, err := json.Marshal(d)
	if err != nil || len(data) > MaxTurnDisplayBytes || !d.Truncated || !utf8.Valid(data) {
		t.Fatalf("bounded: bytes=%d truncated=%t err=%v", len(data), d.Truncated, err)
	}
	d = AppendTurnDisplay(d, Event{Kind: EventTurnStarted})
	if len(d.Items) != 0 || d.Truncated {
		t.Fatal("new turn retained prior data")
	}
}
