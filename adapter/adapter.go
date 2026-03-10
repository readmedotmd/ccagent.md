package ai_adapters

import "context"

// AdapterStatus represents the current state of an adapter.
type AdapterStatus int

const (
	StatusIdle    AdapterStatus = iota
	StatusRunning
	StatusStopped
	StatusError
)

// PermissionMode controls how the adapter handles tool permissions.
type PermissionMode string

const (
	PermissionDefault   PermissionMode = "default"
	PermissionAcceptAll PermissionMode = "accept_all"
	PermissionPlan      PermissionMode = "plan"
)

// MCPServerConfig describes an MCP stdio server to attach to the adapter.
type MCPServerConfig struct {
	Command string
	Args    []string
	Env     map[string]string
}

// AgentDef defines a sub-agent that the adapter can delegate to.
type AgentDef struct {
	Description string
	Prompt      string
	Tools       []string
	Model       string
}

// AdapterConfig holds configuration for starting an adapter.
type AdapterConfig struct {
	Name    string
	Command string
	WorkDir string
	Args    []string
	Env     map[string]string

	// Extended configuration supported by pilot and Claude SDK adapters.
	SystemPrompt       string
	AppendSystemPrompt string
	Model              string
	MaxThinkingTokens  int
	PermissionMode     PermissionMode
	SessionID          string
	ContinueSession    bool
	MCPServers         map[string]MCPServerConfig
	AllowedTools       []string
	DisallowedTools    []string
	Agents             map[string]AgentDef
	ContextWindow      int // context window size in tokens (0 = adapter default)
}

// SendOptions controls per-turn behaviour for a Send call.
type SendOptions struct {
	MaxTokens     int
	StopSequences []string
	Temperature   float64
	Tools         []string // override allowed tools for this turn
}

// SendOption is a functional option for Send.
type SendOption func(*SendOptions)

// WithMaxTokens sets the maximum tokens for a send.
func WithMaxTokens(n int) SendOption {
	return func(o *SendOptions) { o.MaxTokens = n }
}

// WithStopSequences sets stop sequences for a send.
func WithStopSequences(s []string) SendOption {
	return func(o *SendOptions) { o.StopSequences = s }
}

// WithTemperature sets the temperature for a send.
func WithTemperature(t float64) SendOption {
	return func(o *SendOptions) { o.Temperature = t }
}

// WithTools overrides the allowed tools for a single send.
func WithTools(tools []string) SendOption {
	return func(o *SendOptions) { o.Tools = tools }
}

// AdapterCapabilities describes what features an adapter supports.
// The UI uses this to decide which controls to show.
type AdapterCapabilities struct {
	SupportsStreaming    bool
	SupportsImages      bool
	SupportsFiles       bool
	SupportsToolUse     bool
	SupportsMCP         bool
	SupportsThinking    bool
	SupportsCancellation bool
	SupportsHistory     bool
	SupportsSubAgents   bool
	MaxContextWindow    int
	SupportedModels     []string
}

// AdapterError is a typed error that lets the UI distinguish failure modes.
type AdapterError struct {
	Code    ErrorCode
	Message string
	Err     error // underlying error
}

func (e *AdapterError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *AdapterError) Unwrap() error { return e.Err }

// ErrorCode classifies adapter errors.
type ErrorCode int

const (
	ErrUnknown       ErrorCode = iota
	ErrCrashed                 // adapter process died
	ErrRateLimited             // upstream rate limit
	ErrContextLength           // conversation too long
	ErrAuth                    // authentication failure
	ErrTimeout                 // operation timed out
	ErrCancelled               // cancelled by user
	ErrPermission              // tool permission denied
)

// Adapter is the core interface for AI CLI adapters.
type Adapter interface {
	Start(ctx context.Context, cfg AdapterConfig) error
	Send(ctx context.Context, msg Message, opts ...SendOption) error
	Cancel() error
	Receive() <-chan StreamEvent
	Stop() error
	Status() AdapterStatus
	Capabilities() AdapterCapabilities
	Health(ctx context.Context) error
}

// SessionProvider is an optional interface that adapters can implement
// to expose their session ID for resume support.
type SessionProvider interface {
	SessionID() string
}

// HistoryClearer is an optional interface that adapters can implement
// to support clearing conversation history.
type HistoryClearer interface {
	ClearHistory(ctx context.Context) error
}

// HistoryProvider is an optional interface for retrieving past messages.
type HistoryProvider interface {
	GetHistory(ctx context.Context) ([]Message, error)
}

// ConversationManager is an optional interface for adapters that persist
// conversations and support listing / resuming them.
type ConversationManager interface {
	ListConversations(ctx context.Context) ([]Conversation, error)
	ResumeConversation(ctx context.Context, conversationID string) error
}

// PermissionResponder is an optional interface for adapters that surface
// permission requests and accept user decisions.
type PermissionResponder interface {
	RespondPermission(ctx context.Context, toolCallID string, approved bool) error
}

// StatusListener is an optional interface for adapters that notify on
// lifecycle changes without polling.
type StatusListener interface {
	OnStatusChange(fn func(AdapterStatus))
}
