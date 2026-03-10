package claudecode

// Message type constants.
const (
	MessageTypeUser            = "user"
	MessageTypeAssistant       = "assistant"
	MessageTypeSystem          = "system"
	MessageTypeResult          = "result"
	MessageTypeControlRequest  = "control_request"
	MessageTypeControlResponse = "control_response"
	MessageTypeStreamEvent     = "stream_event"
)

// Content block type constants.
const (
	ContentBlockTypeText       = "text"
	ContentBlockTypeThinking   = "thinking"
	ContentBlockTypeToolUse    = "tool_use"
	ContentBlockTypeToolResult = "tool_result"
)

// Message represents any message type in the Claude Code protocol.
type Message interface {
	Type() string
}

// ContentBlock represents any content block within a message.
type ContentBlock interface {
	BlockType() string
}

// AssistantMessage represents a message from the assistant.
type AssistantMessage struct {
	Content []ContentBlock
	Model   string
}

func (m *AssistantMessage) Type() string { return MessageTypeAssistant }

// ResultMessage represents the final result of a conversation turn.
type ResultMessage struct {
	Subtype       string
	DurationMs    int
	DurationAPIMs int
	IsError       bool
	NumTurns      int
	SessionID     string
	TotalCostUSD  *float64
	Result        *string
}

func (m *ResultMessage) Type() string { return MessageTypeResult }

// SystemMessage represents a system message.
type SystemMessage struct {
	Subtype string
	Data    map[string]any
}

func (m *SystemMessage) Type() string { return MessageTypeSystem }

// UserMessage represents a message from the user.
type UserMessage struct {
	Content any
}

func (m *UserMessage) Type() string { return MessageTypeUser }

// TextBlock represents text content.
type TextBlock struct {
	Text string
}

func (b *TextBlock) BlockType() string { return ContentBlockTypeText }

// ThinkingBlock represents thinking content.
type ThinkingBlock struct {
	Thinking  string
	Signature string
}

func (b *ThinkingBlock) BlockType() string { return ContentBlockTypeThinking }

// ToolUseBlock represents a tool use request.
type ToolUseBlock struct {
	ToolUseID string
	Name      string
	Input     map[string]any
}

func (b *ToolUseBlock) BlockType() string { return ContentBlockTypeToolUse }

// ToolResultBlock represents the result of a tool use.
type ToolResultBlock struct {
	ToolUseID string
	Content   any
	IsError   *bool
}

func (b *ToolResultBlock) BlockType() string { return ContentBlockTypeToolResult }

// RawControlMessage wraps raw control protocol messages.
type RawControlMessage struct {
	MessageType string
	Data        map[string]any
}

func (m *RawControlMessage) Type() string { return m.MessageType }

// StreamMessage represents messages sent to the CLI for streaming communication.
type StreamMessage struct {
	Type            string         `json:"type"`
	Message         any            `json:"message,omitempty"`
	ParentToolUseID *string        `json:"parent_tool_use_id,omitempty"`
	SessionID       string         `json:"session_id,omitempty"`
	RequestID       string         `json:"request_id,omitempty"`
	Request         map[string]any `json:"request,omitempty"`
	Response        map[string]any `json:"response,omitempty"`
}

// PermissionMode represents the different permission handling modes.
type PermissionMode string

const (
	PermissionModeDefault           PermissionMode = "default"
	PermissionModeAcceptEdits       PermissionMode = "acceptEdits"
	PermissionModePlan              PermissionMode = "plan"
	PermissionModeBypassPermissions PermissionMode = "bypassPermissions"
)

// McpServerType represents the type of MCP server.
type McpServerType string

const (
	McpServerTypeStdio McpServerType = "stdio"
	McpServerTypeSSE   McpServerType = "sse"
	McpServerTypeHTTP  McpServerType = "http"
)

// McpServerConfig represents MCP server configuration.
type McpServerConfig interface {
	GetType() McpServerType
}

// McpStdioServerConfig configures an MCP stdio server.
type McpStdioServerConfig struct {
	Type    McpServerType     `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

func (c *McpStdioServerConfig) GetType() McpServerType { return McpServerTypeStdio }

// AgentModel represents the model to use for an agent.
type AgentModel string

const (
	AgentModelSonnet  AgentModel = "sonnet"
	AgentModelOpus    AgentModel = "opus"
	AgentModelHaiku   AgentModel = "haiku"
	AgentModelInherit AgentModel = "inherit"
)

// AgentDefinition defines a programmatic subagent.
type AgentDefinition struct {
	Description string     `json:"description"`
	Prompt      string     `json:"prompt"`
	Tools       []string   `json:"tools,omitempty"`
	Model       AgentModel `json:"model,omitempty"`
}
