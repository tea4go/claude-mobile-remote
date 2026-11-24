package protocol

type SubagentStatus string

const (
	SubagentRunning   SubagentStatus = "running"
	SubagentCompleted SubagentStatus = "completed"
	SubagentFailed    SubagentStatus = "failed"
	SubagentCancelled SubagentStatus = "cancelled"
)

const MaxSubagentTasks = 16
const MaxSubagentBlocks = 24

// SubagentTask is the display state of a host-observed background task. Its
// request ID is the launching turn, which can differ from the foreground turn.
type SubagentTask struct {
	RequestID string         `json:"requestId"`
	ToolUseID string         `json:"toolUseId"`
	Status    SubagentStatus `json:"status"`
	Summary   string         `json:"summary,omitempty"`
	Blocks    []ContentBlock `json:"blocks,omitempty"`
}

func (t SubagentTask) Valid() bool {
	if !ValidToolDetailID(t.RequestID) || !ValidToolDetailID(t.ToolUseID) || !ValidToolTitleSummary(t.Summary) {
		return false
	}
	switch t.Status {
	case SubagentRunning, SubagentCompleted, SubagentFailed, SubagentCancelled:
		return true
	}
	return false
}
