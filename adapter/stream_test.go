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
