package claude

import (
	"strings"
	"unicode/utf8"
)

// DefaultToolPreviewMaxRunes 是 B2 §3.1 预览截断的默认上限。
// 运行时上限统一由 ServiceConfig.ToolPreviewMaxRunes 提供，这里只作为
// 该配置的默认值与非法值兜底，避免上限散落到各处常量。
const DefaultToolPreviewMaxRunes = 200

// previewEllipsis 标记预览已被截断，是单个 rune，便于按 rune 计数断言。
const previewEllipsis = '…'

// TruncatePreview 按 rune 边界把展示字符串截断到 maxRunes 个 rune，超长时补一个
// 省略号。它是工具审批预览（B2 §4.3）和排队项预览（§4.4）唯一的截断实现，
// 保证两者语义一致且结果始终是合法 UTF-8。
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

// previewJoinSeparator 连接 Glob/Grep 的 pattern 与 path。
const previewJoinSeparator = " @ "

// previewRule 描述一个白名单工具从 input 抽取预览的方式。joinField 非空时，该
// 字段为非空字符串则追加到主字段之后。
type previewRule struct {
	field     string
	joinField string
}

// toolPreviewRules 是 B2 §3.1 的工具预览白名单。白名单之外的工具（含全部 MCP
// 工具）一律不下发预览，移动端只展示工具名；服务端不做通用 input 序列化，
// 避免把任意工具参数当作展示文本外泄。
var toolPreviewRules = map[string]previewRule{
	"Bash":         {field: "command"},
	"Read":         {field: "file_path"},
	"Write":        {field: "file_path"},
	"Edit":         {field: "file_path"},
	"NotebookEdit": {field: "file_path"},
	"Glob":         {field: "pattern", joinField: "path"},
	"Grep":         {field: "pattern", joinField: "path"},
	"WebFetch":     {field: "url"},
	"WebSearch":    {field: "query"},
}

// ToolPreview 从工具 input 抽取一行展示预览，并按 maxRunes 截断。第二个返回值为
// false 表示不下发预览：工具名未命中白名单、字段缺失、字段不是字符串或去空白后
// 为空。函数只读 input，不修改也不记录其内容。
func ToolPreview(toolName string, input map[string]any, maxRunes int) (string, bool) {
	rule, ok := toolPreviewRules[toolName]
	if !ok {
		return "", false
	}
	preview, ok := previewField(input, rule.field)
	if !ok {
		return "", false
	}
	if rule.joinField != "" {
		if extra, ok := previewField(input, rule.joinField); ok {
			preview += previewJoinSeparator + extra
		}
	}
	return TruncatePreview(preview, maxRunes), true
}

// previewField 取出一个可展示的字符串字段。非字符串（含嵌套对象与数字）与
// 空白字符串都视为不可展示。
func previewField(input map[string]any, field string) (string, bool) {
	value, ok := input[field].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", false
	}
	return value, true
}
