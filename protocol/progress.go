package protocol

import "regexp"

var progressToolName = regexp.MustCompile(`^[a-zA-Z0-9_:.@/\-]{1,128}$`)

// TurnProgress is a content-free live view of one active Turn. Times are Unix ms.
type TurnProgress struct {
	ToolName        string `json:"toolName,omitempty"`
	Phase           string `json:"phase"`
	StartedAt       int64  `json:"startedAt"`
	UpdatedAt       int64  `json:"updatedAt"`
	OutputTokens    int    `json:"outputTokens"`
	TokensKnown     bool   `json:"tokensKnown"`
	TokensEstimated bool   `json:"tokensEstimated"`
}

func (p TurnProgress) Valid() bool {
	switch p.Phase {
	case "waiting", "working", "thinking", "writing", "tool", "subtask", "compacting", "requesting", "retrying", "waiting_followup", "tool_result", "subtask_result":
	default:
		return false
	}
	return (p.ToolName == "" || p.Phase == "tool" && progressToolName.MatchString(p.ToolName)) && p.StartedAt > 0 && p.UpdatedAt >= p.StartedAt && p.OutputTokens >= 0 && p.OutputTokens <= 1000000000
}

// SessionDiagnostic contains only allowlisted codes and numeric metadata.
type SessionDiagnostic struct {
	Code         string `json:"code"`
	HTTPStatus   int    `json:"httpStatus,omitempty"`
	Attempt      int    `json:"attempt,omitempty"`
	MaxAttempts  int    `json:"maxAttempts,omitempty"`
	RetryDelayMs int    `json:"retryDelayMs,omitempty"`
}

func (d SessionDiagnostic) Valid() bool {
	switch d.Code {
	case "api_retry", "query_failed", "receive_failed", "response_incomplete", "assistant_error", "result_error", "turn_failed", "tool_failed":
	default:
		return false
	}
	return (d.HTTPStatus == 0 || d.HTTPStatus >= 100 && d.HTTPStatus <= 599) && d.Attempt >= 0 && d.Attempt <= 1000000 && d.MaxAttempts >= 0 && d.MaxAttempts <= 1000000 && d.RetryDelayMs >= 0 && d.RetryDelayMs <= 86400000
}
