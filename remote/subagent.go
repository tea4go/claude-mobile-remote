package remote

import (
	"slices"

	"github.com/tea4go/claude-mobile-remote/protocol"
)

func (s *EventStream) applySubagent(event protocol.Event) {
	tasks := s.snapshot.SubagentTasks
	if event.SubagentTask != nil {
		update := *event.SubagentTask
		index := slices.IndexFunc(tasks, func(task protocol.SubagentTask) bool {
			return task.RequestID == update.RequestID && task.ToolUseID == update.ToolUseID
		})
		if index >= 0 {
			previous := tasks[index]
			if previous.Status != protocol.SubagentRunning && update.Status == protocol.SubagentRunning {
				update.Status = previous.Status
				update.Summary = previous.Summary
			}
			if update.Summary == "" {
				update.Summary = previous.Summary
			}
			update.Blocks = previous.Blocks
			tasks = slices.Delete(tasks, index, index+1)
		} else if s.snapshot.Turn.RequestID == update.RequestID && s.snapshot.Turn.Display != nil {
			for _, item := range s.snapshot.Turn.Display.Items {
				if item.Block != nil && item.Block.ParentToolUseID == update.ToolUseID {
					rememberSubagentBlock(&update, *item.Block)
				}
			}
		}
		s.snapshot.SubagentTasks = append(tasks, update)
		if len(s.snapshot.SubagentTasks) > protocol.MaxSubagentTasks {
			// Prefer keeping running tasks when a completed task can be evicted.
			evict := slices.IndexFunc(s.snapshot.SubagentTasks, func(task protocol.SubagentTask) bool { return task.Status != protocol.SubagentRunning })
			if evict < 0 {
				evict = 0
			}
			s.snapshot.SubagentTasks = slices.Delete(s.snapshot.SubagentTasks, evict, evict+1)
		}
		return
	}
	if event.Block == nil || event.Block.ParentToolUseID == "" {
		return
	}
	index := slices.IndexFunc(tasks, func(task protocol.SubagentTask) bool {
		return task.RequestID == event.RequestID && task.ToolUseID == event.Block.ParentToolUseID
	})
	if index >= 0 {
		rememberSubagentBlock(&tasks[index], *event.Block)
	}
}

func rememberSubagentBlock(task *protocol.SubagentTask, block protocol.ContentBlock) {
	// The legacy inline preview is never needed in a v14 task snapshot.
	block.Output, block.Truncated = "", false
	if index := slices.IndexFunc(task.Blocks, func(previous protocol.ContentBlock) bool {
		return previous.Kind == block.Kind && previous.ToolUseID == block.ToolUseID
	}); index >= 0 {
		block.ID = task.Blocks[index].ID
		task.Blocks[index] = block
		return
	}
	task.Blocks = append(task.Blocks, block)
	if len(task.Blocks) > protocol.MaxSubagentBlocks {
		oldest := task.Blocks[0].ToolUseID
		task.Blocks = slices.DeleteFunc(task.Blocks, func(previous protocol.ContentBlock) bool { return previous.ToolUseID == oldest })
	}
}

// SubagentOrigin resolves a known parent, including nested tools, to the
// originating turn and panel. Only observations from this stream are eligible.
func (s *EventStream) SubagentOrigin(parent string) (requestID, toolUseID string) {
	if parent == "" {
		return "", ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, task := range s.snapshot.SubagentTasks {
		if task.ToolUseID == parent || slices.ContainsFunc(task.Blocks, func(block protocol.ContentBlock) bool {
			return block.Kind == protocol.BlockToolUse && block.ToolUseID == parent
		}) {
			return task.RequestID, task.ToolUseID
		}
	}
	return "", ""
}
