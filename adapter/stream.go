package ai_adapters

import "time"

// ToolStatusValue represents the lifecycle state of a tool call.
type ToolStatusValue string

const (
	ToolRunning  ToolStatusValue = "running"
	ToolComplete ToolStatusValue = "complete"
	ToolFailed   ToolStatusValue = "failed"
)

// SubAgentStatus represents the lifecycle state of a sub-agent.
type SubAgentStatus string

const (
	SubAgentStarted   SubAgentStatus = "started"
	SubAgentCompleted SubAgentStatus = "completed"
	SubAgentFailed    SubAgentStatus = "failed"
)

// StreamEventType categorizes streaming events.
type StreamEventType int

const (
	EventToken StreamEventType = iota
	EventDone
	EventError
	EventToolUse
	EventToolResult
	EventSystem
	EventThinking
	EventPermissionRequest
	EventPermissionResult
	EventProgress
	EventFileChange
	EventSubAgent
	EventCostUpdate
	EventCompactionBegin
	EventCompactionEnd
	EventStepBegin
	EventStepEnd
	EventToolInputStream
	EventExternalToolCall
	EventDisplayBlock
)

// TokenUsage reports token consumption and cost for a turn or session.
type TokenUsage struct {
	InputTokens  int
	OutputTokens int
	CacheRead    int
	CacheWrite   int
	TotalCost    float64
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
	OldPath string
}

// PermissionRequest is sent when the agent needs user approval.
type PermissionRequest struct {
	ToolCallID  string
	ToolName    string
	ToolInput   any
	Description string
}

// SubAgentEvent describes sub-agent lifecycle events.
type SubAgentEvent struct {
	AgentID   string
	AgentName string
	Status    SubAgentStatus
	Prompt    string
	Result    string
}

// CompactionInfo provides details about context compaction.
type CompactionInfo struct {
	Reason       string
	TokensBefore int
	TokensAfter  int
	Summary      string
}

// StepInfo provides details about step lifecycle.
type StepInfo struct {
	StepNumber int
	TotalSteps int
}

// DisplayBlockType identifies the type of rich display content.
type DisplayBlockType string

const (
	DisplayBlockBrief   DisplayBlockType = "brief"
	DisplayBlockDiff    DisplayBlockType = "diff"
	DisplayBlockTodo    DisplayBlockType = "todo"
	DisplayBlockShell   DisplayBlockType = "shell"
	DisplayBlockUnknown DisplayBlockType = "unknown"
)

// TodoStatus represents the status of a todo item.
type TodoStatus string

const (
	TodoStatusPending    TodoStatus = "pending"
	TodoStatusInProgress TodoStatus = "in_progress"
	TodoStatusDone       TodoStatus = "done"
)

// TodoItem represents a single todo item.
type TodoItem struct {
	Title  string
	Status TodoStatus
}

// DisplayBlock represents rich output content from tools.
type DisplayBlock struct {
	Type     DisplayBlockType
	Text     string
	Path     string
	OldText  string
	NewText  string
	Items    []TodoItem
	Language string
	Command  string
	Data     map[string]any
}

// StreamEvent represents a single event in the streaming response.
type StreamEvent struct {
	Type      StreamEventType
	Timestamp time.Time

	Token    string
	Thinking string

	ToolCallID string
	ToolName   string
	ToolInput  any
	ToolOutput any
	ToolStatus ToolStatusValue

	Permission *PermissionRequest
	FileChange *FileChange
	SubAgent   *SubAgentEvent

	ProgressPct float64
	ProgressMsg string

	Usage *TokenUsage

	Compaction *CompactionInfo
	Step       *StepInfo
	DisplayBlock *DisplayBlock

	Error   error
	Message *Message
}
