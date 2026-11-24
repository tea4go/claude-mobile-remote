package protocol

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxToolTitleRunes = 160

var toolTitleFields = map[string]string{
	"Read": "file_path", "Write": "file_path", "Edit": "file_path", "MultiEdit": "file_path",
	"Glob": "pattern", "Grep": "pattern", "LSP": "filePath", "NotebookEdit": "notebook_path",
	"Bash": "description", "Agent": "description", "Task": "description",
	"TaskCreate": "subject", "TaskUpdate": "subject", "TaskGet": "taskId", "SendMessage": "recipient",
	"WebFetch": "url", "WebSearch": "query", "Skill": "skill", "CronCreate": "cron", "CronDelete": "id", "ScheduleWakeup": "cron",
}

// ToolTitleSummary selects the display parameter used by WhaleTerm's tool header.
func ToolTitleSummary(name string, input map[string]any) string {
	if name == "TodoWrite" {
		if todos, ok := input["todos"].([]any); ok {
			return fmt.Sprintf("(%d items)", len(todos))
		}
	}
	field, ok := toolTitleFields[name]
	if !ok {
		return ""
	}
	value, _ := input[field].(string)
	if name == "Bash" && strings.TrimSpace(value) == "" {
		value, _ = input["command"].(string)
	}
	value = strings.ToValidUTF8(value, "�")
	words := strings.FieldsFunc(value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) })
	title := strings.Join(words, " ")
	if utf8.RuneCountInString(title) <= MaxToolTitleRunes {
		return title
	}
	return TruncatePreview(title, MaxToolTitleRunes-1)
}

func ValidToolTitleSummary(value string) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) <= MaxToolTitleRunes && !strings.ContainsFunc(value, unicode.IsControl)
}
