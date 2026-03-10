package ai_adapters

import "time"

// StreamEventType categorizes streaming events.
type StreamEventType int

const (
	EventToken             StreamEventType = iota
	EventDone
	EventError
	EventToolUse
	EventToolResult
	EventSystem
	EventThinking
	EventPermissionRequest // agent requests approval to run a tool
	EventPermissionResult  // result of a permission decision (for logging/replay)
	EventProgress          // progress update for a long-running tool call
	EventFileChange        // agent created, edited, or deleted a file
	EventSubAgent          // agent delegated to a sub-agent
	EventCostUpdate        // token usage / cost update
)

// TokenUsage reports token consumption and cost for a turn or session.
type TokenUsage struct {
	InputTokens  int
	OutputTokens int
	CacheRead    int
	CacheWrite   int
	TotalCost    float64 // estimated cost in USD
}

// FileChangeOp describes what happened to a file.
type FileChangeOp string

const (
	FileCreated FileChangeOp = "created"
	FileEdited  FileChangeOp = "edited"
	FileDeleted FileChangeOp = "deleted"
	FileRenamed FileChangeOp = "renamed"
)

// FileChange describes a file operation performed by the agent.
type FileChange struct {
	Op      FileChangeOp
	Path    string
	OldPath string // for renames
}

// PermissionRequest is sent when the agent needs user approval.
type PermissionRequest struct {
	ToolCallID  string
	ToolName    string
	ToolInput   any
	Description string // human-readable summary of what the tool will do
}

// SubAgentEvent describes sub-agent lifecycle events.
type SubAgentEvent struct {
	AgentID   string
	AgentName string
	Status    string // "started", "completed", "failed"
	Prompt    string
	Result    string
}

// StreamEvent represents a single event in the streaming response.
type StreamEvent struct {
	Type      StreamEventType
	Timestamp time.Time

	// Content
	Token    string
	Thinking string

	// Tool use — ToolCallID correlates request with result.
	ToolCallID string
	ToolName   string
	ToolInput  any
	ToolOutput any
	ToolStatus string // "running", "complete", "failed"

	// Permission flow
	Permission *PermissionRequest

	// File operations
	FileChange *FileChange

	// Sub-agent delegation
	SubAgent *SubAgentEvent

	// Progress for long-running operations
	ProgressPct float64 // 0–1, -1 if indeterminate
	ProgressMsg string

	// Cost / usage
	Usage *TokenUsage

	// Control flow
	Error   error
	Message *Message
}
