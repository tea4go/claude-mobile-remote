package protocol

import (
	"encoding/json"
	"unicode/utf8"
)

// TurnDisplay is a bounded, ordered view of one observed turn, not a history.
type TurnDisplay struct {
	Items     []TurnDisplayItem `json:"items"`
	Truncated bool              `json:"truncated"`
}
type TurnDisplayItem struct {
	Text  string        `json:"text,omitempty"`
	Model string        `json:"model,omitempty"`
	Block *ContentBlock `json:"block,omitempty"`
}

const MaxTurnDisplayItems = 512
const MaxTurnDisplayBytes = 512 * 1024

// AppendTurnDisplay keeps the most recent valid display items within the wire budget.
func AppendTurnDisplay(display *TurnDisplay, event Event) *TurnDisplay {
	if event.Kind == EventTurnStarted {
		return &TurnDisplay{Items: []TurnDisplayItem{}}
	}
	if event.Kind != EventTextDelta && event.Kind != EventContentBlock {
		return display
	}
	if display == nil {
		display = &TurnDisplay{Items: []TurnDisplayItem{}, Truncated: true}
	}
	if event.Kind == EventTextDelta && event.Text != "" {
		n := len(display.Items)
		if n > 0 && display.Items[n-1].Block == nil && display.Items[n-1].Model == event.Model {
			display.Items[n-1].Text += event.Text
		} else {
			display.Items = append(display.Items, TurnDisplayItem{Text: event.Text, Model: event.Model})
		}
	} else if event.Block != nil {
		b := *event.Block
		display.Items = append(display.Items, TurnDisplayItem{Block: &b})
	}
	for {
		data, err := json.Marshal(display)
		if err == nil && len(display.Items) <= MaxTurnDisplayItems && len(data) <= MaxTurnDisplayBytes {
			break
		}
		display.Truncated = true
		if len(display.Items) > 1 {
			display.Items = display.Items[1:]
			continue
		}
		if len(display.Items) == 0 {
			break
		}
		item := &display.Items[0]
		if item.Block != nil {
			display.Items = display.Items[:0]
			break
		}
		if len(item.Text) == 0 {
			display.Items = display.Items[:0]
			break
		}
		cut := len(item.Text) / 2
		for cut < len(item.Text) && !utf8.RuneStart(item.Text[cut]) {
			cut++
		}
		item.Text = item.Text[cut:]
	}
	return display
}
func CloneTurnDisplay(display *TurnDisplay) *TurnDisplay {
	if display == nil {
		return nil
	}
	result := &TurnDisplay{Truncated: display.Truncated, Items: make([]TurnDisplayItem, len(display.Items))}
	for i, item := range display.Items {
		result.Items[i] = item
		if item.Block != nil {
			b := *item.Block
			if b.IsError != nil {
				v := *b.IsError
				b.IsError = &v
			}
			result.Items[i].Block = &b
		}
	}
	return result
}
