package ai_adapters

import (
	"errors"
	"testing"
	"time"
)

func TestStreamEventTypes(t *testing.T) {
	expected := []StreamEventType{
		EventToken, EventDone, EventError, EventToolUse,
		EventToolResult, EventSystem, EventThinking,
		EventPermissionRequest, EventPermissionResult,
		EventProgress, EventFileChange, EventSubAgent, EventCostUpdate,
	}
	for i, et := range expected {
		if int(et) != i {
			t.Errorf("expected event type %d at index %d, got %d", i, i, et)
		}
	}
}

func TestStreamEventToken(t *testing.T) {
	ev := StreamEvent{
		Type:      EventToken,
		Token:     "Hello",
		Timestamp: time.Now(),
	}
	if ev.Token != "Hello" || ev.Type != EventToken {
		t.Fatal("unexpected token event")
	}
}

func TestStreamEventThinking(t *testing.T) {
	ev := StreamEvent{
		Type:      EventThinking,
		Thinking:  "Let me think about this...",
		Timestamp: time.Now(),
	}
	if ev.Thinking != "Let me think about this..." {
		t.Fatal("unexpected thinking content")
	}
}

func TestStreamEventToolUse(t *testing.T) {
	ev := StreamEvent{
		Type:       EventToolUse,
		ToolCallID: "call-123",
		ToolName:   "bash",
		ToolInput:  map[string]string{"command": "ls"},
		ToolStatus: "running",
		Timestamp:  time.Now(),
	}
	if ev.ToolCallID != "call-123" || ev.ToolName != "bash" {
		t.Fatal("unexpected tool use fields")
	}
	if ev.ToolStatus != "running" {
		t.Fatal("unexpected tool status")
	}
}

func TestStreamEventToolResult(t *testing.T) {
	ev := StreamEvent{
		Type:       EventToolResult,
		ToolCallID: "call-123",
		ToolName:   "bash",
		ToolOutput: "file1.go\nfile2.go",
		ToolStatus: "complete",
		Timestamp:  time.Now(),
	}
	if ev.ToolOutput != "file1.go\nfile2.go" || ev.ToolStatus != "complete" {
		t.Fatal("unexpected tool result fields")
	}
}

func TestStreamEventError(t *testing.T) {
	ev := StreamEvent{
		Type:      EventError,
		Error:     errors.New("something failed"),
		Timestamp: time.Now(),
	}
	if ev.Error == nil || ev.Error.Error() != "something failed" {
		t.Fatal("unexpected error event")
	}
}

func TestStreamEventDoneWithMessage(t *testing.T) {
	msg := &Message{
		ID:      "msg-1",
		Role:    RoleAssistant,
		Content: TextContent("Done!"),
	}
	ev := StreamEvent{
		Type:      EventDone,
		Message:   msg,
		Timestamp: time.Now(),
	}
	if ev.Message == nil || ev.Message.ID != "msg-1" {
		t.Fatal("unexpected done event message")
	}
}

func TestStreamEventPermission(t *testing.T) {
	perm := &PermissionRequest{
		ToolCallID:  "call-456",
		ToolName:    "write",
		ToolInput:   map[string]string{"path": "/tmp/test"},
		Description: "Write to /tmp/test",
	}
	ev := StreamEvent{
		Type:       EventPermissionRequest,
		Permission: perm,
		Timestamp:  time.Now(),
	}
	if ev.Permission.ToolCallID != "call-456" {
		t.Fatal("unexpected permission tool call ID")
	}
	if ev.Permission.Description != "Write to /tmp/test" {
		t.Fatal("unexpected permission description")
	}
}

func TestStreamEventFileChange(t *testing.T) {
	tests := []struct {
		op   FileChangeOp
		want string
	}{
		{FileCreated, "created"},
		{FileEdited, "edited"},
		{FileDeleted, "deleted"},
		{FileRenamed, "renamed"},
	}
	for _, tt := range tests {
		if string(tt.op) != tt.want {
			t.Errorf("expected %q, got %q", tt.want, tt.op)
		}
	}

	ev := StreamEvent{
		Type: EventFileChange,
		FileChange: &FileChange{
			Op:      FileRenamed,
			Path:    "/new/path.go",
			OldPath: "/old/path.go",
		},
		Timestamp: time.Now(),
	}
	if ev.FileChange.OldPath != "/old/path.go" {
		t.Fatal("unexpected old path")
	}
}

func TestStreamEventSubAgent(t *testing.T) {
	ev := StreamEvent{
		Type: EventSubAgent,
		SubAgent: &SubAgentEvent{
			AgentID:   "agent-1",
			AgentName: "researcher",
			Status:    "completed",
			Prompt:    "find the bug",
			Result:    "found it in line 42",
		},
		Timestamp: time.Now(),
	}
	if ev.SubAgent.AgentID != "agent-1" || ev.SubAgent.Status != "completed" {
		t.Fatal("unexpected sub agent fields")
	}
}

func TestStreamEventProgress(t *testing.T) {
	ev := StreamEvent{
		Type:        EventProgress,
		ProgressPct: 0.5,
		ProgressMsg: "Halfway done",
		Timestamp:   time.Now(),
	}
	if ev.ProgressPct != 0.5 || ev.ProgressMsg != "Halfway done" {
		t.Fatal("unexpected progress fields")
	}

	// Indeterminate progress
	ev2 := StreamEvent{
		Type:        EventProgress,
		ProgressPct: -1,
		ProgressMsg: "Working...",
	}
	if ev2.ProgressPct != -1 {
		t.Fatal("expected -1 for indeterminate")
	}
}

func TestStreamEventCostUpdate(t *testing.T) {
	ev := StreamEvent{
		Type: EventCostUpdate,
		Usage: &TokenUsage{
			InputTokens:  1000,
			OutputTokens: 500,
			CacheRead:    200,
			CacheWrite:   100,
			TotalCost:    0.025,
		},
		Timestamp: time.Now(),
	}
	if ev.Usage.InputTokens != 1000 || ev.Usage.OutputTokens != 500 {
		t.Fatal("unexpected token counts")
	}
	if ev.Usage.CacheRead != 200 || ev.Usage.CacheWrite != 100 {
		t.Fatal("unexpected cache counts")
	}
	if ev.Usage.TotalCost != 0.025 {
		t.Fatal("unexpected total cost")
	}
}

// --- Extended event type tests ---

func TestStreamEventTypesExtended(t *testing.T) {
	// Verify the full set of event types have expected iota values.
	types := map[StreamEventType]int{
		EventCompactionBegin: 13,
		EventCompactionEnd:   14,
		EventStepBegin:       15,
		EventStepEnd:         16,
		EventToolInputStream: 17,
		EventExternalToolCall: 18,
		EventDisplayBlock:    19,
	}
	for et, want := range types {
		if int(et) != want {
			t.Errorf("event type %d: expected iota %d", et, want)
		}
	}
}

func TestStreamEventCompaction(t *testing.T) {
	ev := StreamEvent{
		Type: EventCompactionBegin,
		Compaction: &CompactionInfo{
			Reason:       "context_limit",
			TokensBefore: 180000,
			TokensAfter:  0,
			Summary:      "",
		},
		Timestamp: time.Now(),
	}
	if ev.Compaction.Reason != "context_limit" {
		t.Fatal("unexpected compaction reason")
	}
	if ev.Compaction.TokensBefore != 180000 {
		t.Fatal("unexpected tokens before")
	}

	ev2 := StreamEvent{
		Type: EventCompactionEnd,
		Compaction: &CompactionInfo{
			Reason:       "context_limit",
			TokensBefore: 180000,
			TokensAfter:  5000,
			Summary:      "User asked to refactor auth module. Changes made to auth.go and handler.go.",
		},
		Timestamp: time.Now(),
	}
	if ev2.Compaction.TokensAfter != 5000 {
		t.Fatal("unexpected tokens after")
	}
	if ev2.Compaction.Summary == "" {
		t.Fatal("expected non-empty summary")
	}
}

func TestStreamEventStep(t *testing.T) {
	begin := StreamEvent{
		Type: EventStepBegin,
		Step: &StepInfo{
			StepNumber: 1,
			TotalSteps: -1, // unknown total
		},
		Timestamp: time.Now(),
	}
	if begin.Step.StepNumber != 1 || begin.Step.TotalSteps != -1 {
		t.Fatal("unexpected step begin fields")
	}

	end := StreamEvent{
		Type: EventStepEnd,
		Step: &StepInfo{
			StepNumber: 1,
			TotalSteps: 1,
		},
		Timestamp: time.Now(),
	}
	if end.Step.StepNumber != 1 || end.Step.TotalSteps != 1 {
		t.Fatal("unexpected step end fields")
	}
}

func TestStreamEventDisplayBlock(t *testing.T) {
	tests := []struct {
		name  string
		block DisplayBlock
	}{
		{
			name: "brief",
			block: DisplayBlock{
				Type: DisplayBlockBrief,
				Text: "Working on auth module",
			},
		},
		{
			name: "diff",
			block: DisplayBlock{
				Type:    DisplayBlockDiff,
				Path:    "auth.go",
				OldText: "func login() {}",
				NewText: "func login(ctx context.Context) error {}",
			},
		},
		{
			name: "todo",
			block: DisplayBlock{
				Type: DisplayBlockTodo,
				Items: []TodoItem{
					{Title: "Fix auth", Status: TodoStatusDone},
					{Title: "Add tests", Status: TodoStatusInProgress},
					{Title: "Deploy", Status: TodoStatusPending},
				},
			},
		},
		{
			name: "shell",
			block: DisplayBlock{
				Type:     DisplayBlockShell,
				Command:  "go test ./...",
				Language: "bash",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := StreamEvent{
				Type:         EventDisplayBlock,
				DisplayBlock: &tt.block,
				Timestamp:    time.Now(),
			}
			if ev.DisplayBlock == nil {
				t.Fatal("expected non-nil display block")
			}
		})
	}
}

func TestDisplayBlockTypes(t *testing.T) {
	types := map[DisplayBlockType]string{
		DisplayBlockBrief:   "brief",
		DisplayBlockDiff:    "diff",
		DisplayBlockTodo:    "todo",
		DisplayBlockShell:   "shell",
		DisplayBlockUnknown: "unknown",
	}
	for dt, want := range types {
		if string(dt) != want {
			t.Errorf("expected %q, got %q", want, dt)
		}
	}
}

func TestTodoStatuses(t *testing.T) {
	statuses := map[TodoStatus]string{
		TodoStatusPending:    "pending",
		TodoStatusInProgress: "in_progress",
		TodoStatusDone:       "done",
	}
	for ts, want := range statuses {
		if string(ts) != want {
			t.Errorf("expected %q, got %q", want, ts)
		}
	}
}

func TestSubAgentStatuses(t *testing.T) {
	statuses := map[SubAgentStatus]string{
		SubAgentStarted:   "started",
		SubAgentCompleted: "completed",
		SubAgentFailed:    "failed",
	}
	for s, want := range statuses {
		if string(s) != want {
			t.Errorf("expected %q, got %q", want, s)
		}
	}
}

func TestToolStatusValues(t *testing.T) {
	statuses := map[ToolStatusValue]string{
		ToolRunning:  "running",
		ToolComplete: "complete",
		ToolFailed:   "failed",
	}
	for s, want := range statuses {
		if string(s) != want {
			t.Errorf("expected %q, got %q", want, s)
		}
	}
}

func TestStreamEventDisplayBlockWithData(t *testing.T) {
	ev := StreamEvent{
		Type: EventDisplayBlock,
		DisplayBlock: &DisplayBlock{
			Type: DisplayBlockUnknown,
			Data: map[string]any{
				"custom_key": "custom_value",
				"count":      42.0,
			},
		},
		Timestamp: time.Now(),
	}
	if ev.DisplayBlock.Data["custom_key"] != "custom_value" {
		t.Fatal("unexpected custom data")
	}
}
