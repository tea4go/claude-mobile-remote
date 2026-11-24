package protocol

import (
	"container/list"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

const (
	MaxToolDetailIDBytes = 256
	MaxToolInputBytes    = 32 * 1024
	MaxToolResultBytes   = 96 * 1024
	MaxReplyModelBytes   = 128
	maxToolDetailBytes   = 8 * 1024 * 1024
	maxToolDetails       = 256
)

// ToolDetail contains display text from an observed SDK tool call. It is served
// separately from events and snapshots and is never interpreted as a path or command.
type ToolDetail struct {
	Input           string `json:"input,omitempty"`
	Output          string `json:"output,omitempty"`
	InputTruncated  bool   `json:"inputTruncated,omitempty"`
	OutputTruncated bool   `json:"outputTruncated,omitempty"`
}

func ValidToolDetailID(id string) bool {
	return id != "" && len(id) <= MaxToolDetailIDBytes && utf8.ValidString(id) && !strings.ContainsFunc(id, unicode.IsControl)
}

func ValidReplyModel(model string) bool {
	return len(model) <= MaxReplyModelBytes && utf8.ValidString(model) && !strings.ContainsFunc(model, unicode.IsControl)
}

func (detail ToolDetail) Valid() bool {
	return len(detail.Input) <= MaxToolInputBytes && len(detail.Output) <= MaxToolResultBytes &&
		utf8.ValidString(detail.Input) && utf8.ValidString(detail.Output) && (detail.Input != "" || detail.Output != "")
}

// ToolInputDetail selects display fields, leaving arbitrary tool parameters,
// resource payloads and image data in the SDK. It never opens a local file.
func ToolInputDetail(input map[string]any) ToolDetail {
	var text strings.Builder
	var truncated bool
	for _, key := range []string{"description", "subagent_type", "file_path", "path", "command", "pattern", "query", "url", "offset", "limit", "old_string", "new_string", "content"} {
		value, _ := input[key].(string)
		if value == "" {
			continue
		}
		if text.Len() != 0 {
			text.WriteString("\n\n")
		}
		text.WriteString(key)
		text.WriteString(":\n")
		if text.Len() >= MaxToolInputBytes {
			truncated = true
			break
		}
		part, clipped := clipToolText(value, MaxToolInputBytes-text.Len())
		text.WriteString(part)
		truncated = truncated || clipped
		if clipped {
			break
		}
	}
	value, clipped := clipToolText(text.String(), MaxToolInputBytes)
	return ToolDetail{Input: value, InputTruncated: truncated || clipped}
}

type toolDetailKey struct{ requestID, toolUseID string }

type toolDetailEntry struct {
	key    toolDetailKey
	detail ToolDetail
}

// ToolDetails is a bounded, in-memory store owned by one session host. Its zero
// value is ready to use. It accepts only text already observed by that host.
type ToolDetails struct {
	mu    sync.Mutex
	order list.List
	items map[toolDetailKey]*list.Element
	bytes int
}

// Record merges call and result text. Callers publish the detached block, so
// detail bytes never travel through subscribers or the snapshot projection.
func (s *ToolDetails) Record(requestID string, block *ContentBlock) *ContentBlock {
	if block == nil || block.Detail == nil {
		return block
	}
	copy := *block
	copy.Detail = nil
	if !ValidToolDetailID(requestID) || !ValidToolDetailID(block.ToolUseID) || !block.Detail.Valid() {
		return &copy
	}
	key := toolDetailKey{requestID, block.ToolUseID}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.items == nil {
		s.items = make(map[toolDetailKey]*list.Element)
	}
	detail := *block.Detail
	if element := s.items[key]; element != nil {
		previous := s.remove(element)
		if detail.Input == "" {
			detail.Input, detail.InputTruncated = previous.Input, previous.InputTruncated
		}
		if detail.Output == "" {
			detail.Output, detail.OutputTruncated = previous.Output, previous.OutputTruncated
		}
	}
	s.items[key] = s.order.PushBack(toolDetailEntry{key, detail})
	s.bytes += len(detail.Input) + len(detail.Output)
	for len(s.items) > maxToolDetails || s.bytes > maxToolDetailBytes {
		s.remove(s.order.Front())
	}
	copy.DetailAvailable = true
	return &copy
}

func (s *ToolDetails) remove(element *list.Element) ToolDetail {
	entry := s.order.Remove(element).(toolDetailEntry)
	delete(s.items, entry.key)
	s.bytes -= len(entry.detail.Input) + len(entry.detail.Output)
	return entry.detail
}

func (s *ToolDetails) Get(requestID, toolUseID string) (ToolDetail, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if element := s.items[toolDetailKey{requestID, toolUseID}]; element != nil {
		return element.Value.(toolDetailEntry).detail, true
	}
	return ToolDetail{}, false
}
