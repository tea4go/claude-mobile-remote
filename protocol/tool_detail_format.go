package protocol

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// ToolCallDetail projects tool-specific inputs from SDK arguments already seen by the host.
func ToolCallDetail(name string, input map[string]any) ToolDetail {
	switch name {
	case "Bash":
		command, _ := input["command"].(string)
		text, truncated := clipToolText(command, MaxToolInputBytes)
		return ToolDetail{Input: text, InputTruncated: truncated}
	case AskUserQuestionToolName:
		var display detailText
		questions, _ := input["questions"].([]any)
		for _, value := range questions {
			question, _ := value.(map[string]any)
			if label := stringField(question, "question"); label != "" {
				display.add(label + "\n")
				options, _ := question["options"].([]any)
				for _, entry := range options {
					option, _ := entry.(map[string]any)
					if label := stringField(option, "label"); label != "" {
						display.add("• " + label)
						if description := stringField(option, "description"); description != "" {
							display.add(" — " + description)
						}
						display.add("\n")
					}
				}
				display.add("\n")
			}
			if display.truncated {
				break
			}
		}
		return ToolDetail{Input: strings.TrimSpace(display.text.String()), InputTruncated: display.truncated}
	default:
		return ToolInputDetail(input)
	}
}

// ToolResultWithMetadata keeps the legacy textual event and enriches only the
// on-demand detail using known SDK fields. Paths in metadata are display text.
func ToolResultWithMetadata(name string, content any, metadata map[string]any) (string, bool, ToolDetail) {
	preview, truncated, detail := ToolResultDisplay(content)
	if len(metadata) == 0 {
		return preview, truncated, detail
	}

	var output detailText
	output.limit = MaxToolResultBytes
	switch name {
	case "Read":
		file, _ := metadata["file"].(map[string]any)
		output.add(stringField(file, "content"))
		if output.text.Len() == 0 {
			output.add(stringField(metadata, "content"))
		}
	case "Edit", "MultiEdit", "Write":
		patches, _ := metadata["structuredPatch"].([]any)
		if len(patches) > 0 {
			for _, value := range patches {
				patch, _ := value.(map[string]any)
				lines, _ := patch["lines"].([]any)
				if len(lines) == 0 {
					continue
				}
				output.add(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", numberField(patch, "oldStart"), numberField(patch, "oldLines"), numberField(patch, "newStart"), numberField(patch, "newLines")))
				for _, entry := range lines {
					line, _ := entry.(string)
					output.add(line)
					output.add("\n")
					if output.truncated {
						break
					}
				}
				if output.truncated {
					break
				}
			}
		} else if name == "Write" {
			output.add(stringField(metadata, "content"))
		}
	case "Bash":
		if detail.Output != "" {
			return preview, truncated, detail
		}
		output.add(stringField(metadata, "stdout"))
		if stderr := stringField(metadata, "stderr"); stderr != "" {
			if output.text.Len() > 0 {
				output.add("\n\n")
			}
			output.add(stderr)
		}
	case AskUserQuestionToolName:
		answers, _ := metadata["answers"].(map[string]any)
		questions, _ := metadata["questions"].([]any)
		if len(questions) == 0 {
			for _, question := range slices.Sorted(maps.Keys(answers)) {
				output.answer(question, answers[question])
			}
		} else {
			for _, question := range questions {
				fields, _ := question.(map[string]any)
				label := stringField(fields, "question")
				output.answer(label, answers[label])
			}
		}
	}

	if output.text.Len() == 0 {
		return preview, truncated, detail
	}
	if name != "Bash" && name != AskUserQuestionToolName && detail.Output != "" && !strings.Contains(detail.Output, strings.TrimSpace(output.text.String())) {
		output.add("\n\n")
		output.add(detail.Output)
		output.truncated = output.truncated || detail.OutputTruncated
	}
	detail.Output, detail.OutputTruncated = output.text.String(), output.truncated
	return preview, truncated, detail
}

func stringField(fields map[string]any, key string) string {
	value, _ := fields[key].(string)
	return value
}

func numberField(fields map[string]any, key string) int {
	switch number := fields[key].(type) {
	case float64:
		return int(number)
	case int:
		return number
	default:
		return 0
	}
}

type detailText struct {
	text      strings.Builder
	limit     int
	truncated bool
}

func (d *detailText) add(value string) {
	limit := d.limit
	if limit == 0 {
		limit = MaxToolInputBytes
	}
	part, clipped := clipToolText(value, limit-d.text.Len())
	d.text.WriteString(part)
	d.truncated = d.truncated || clipped
}

func (d *detailText) answer(question string, value any) {
	answer, _ := value.(string)
	if question != "" && answer != "" {
		d.add(question)
		d.add("\n")
		d.add(answer)
		d.add("\n\n")
	}
}
