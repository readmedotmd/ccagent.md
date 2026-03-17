// Package ai_adapters defines the interfaces and types for AI CLI adapters.
//
// The core [Adapter] interface provides a uniform way to interact with AI
// assistants through streaming events. Adapters handle the lifecycle of a
// connection (Start/Stop), message sending (Send), response streaming
// (Receive), and graceful cancellation (Cancel).
//
// Optional interfaces extend adapter capabilities: [SessionProvider] for
// session resume support, [HistoryProvider] for conversation retrieval,
// [PermissionResponder] for interactive tool approval, and others.
//
// Stream events flow through a channel returned by [Adapter.Receive] and
// cover the full lifecycle of a response: token streaming, tool use,
// thinking, file changes, sub-agent activity, context compaction, and more.
package ai_adapters

import (
	"context"
	"time"
)

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

// ApprovalResponse represents the possible responses to a permission request.
type ApprovalResponse string

const (
	// ApprovalResponseApprove approves the tool call for this instance only.
	ApprovalResponseApprove ApprovalResponse = "approve"
	// ApprovalResponseApproveForSession approves this tool for the entire session.
	ApprovalResponseApproveForSession ApprovalResponse = "approve_for_session"
	// ApprovalResponseReject rejects the tool call.
	ApprovalResponseReject ApprovalResponse = "reject"
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

	// ExternalTools allows registering custom tools that the model can call.
	ExternalTools []ExternalTool
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
type AdapterCapabilities struct {
	SupportsStreaming     bool
	SupportsImages        bool
	SupportsFiles         bool
	SupportsToolUse       bool
	SupportsMCP           bool
	SupportsThinking      bool
	SupportsCancellation  bool
	SupportsHistory       bool
	SupportsSubAgents     bool
	SupportsExternalTools bool
	SupportsDisplayBlocks bool
	MaxContextWindow      int
	SupportedModels       []string
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
	ErrCrashed
	ErrRateLimited
	ErrContextLength
	ErrAuth
	ErrTimeout
	ErrCancelled
	ErrPermission
	ErrToolNotFound
	ErrToolExecution
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

// SessionManager is an optional interface for full session lifecycle management.
type SessionManager interface {
	SessionProvider
	ListSessions(ctx context.Context) ([]SessionInfo, error)
	ResumeSession(ctx context.Context, sessionID string) error
	DeleteSession(ctx context.Context, sessionID string) error
}

// SessionInfo describes a persisted session.
type SessionInfo struct {
	ID        string
	WorkDir   string
	UpdatedAt int64  // Timestamp in milliseconds
	Brief     string // First user message preview
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
	RespondPermission(ctx context.Context, toolCallID string, response ApprovalResponse) error
}

// StatusListener is an optional interface for adapters that notify on
// lifecycle changes without polling.
type StatusListener interface {
	OnStatusChange(fn func(AdapterStatus))
}

// Checkpoint represents a saved state that can be restored.
type Checkpoint struct {
	ID        string
	CreatedAt time.Time
	Summary   string
}

// CheckpointManager is an optional interface for adapters that support
// checkpoint/restore functionality.
type CheckpointManager interface {
	CreateCheckpoint(ctx context.Context) (string, error)
	RestoreCheckpoint(ctx context.Context, checkpointID string) error
	ListCheckpoints(ctx context.Context) ([]Checkpoint, error)
}

// ModelSwitcher is an optional interface for adapters that support
// switching models mid-session.
type ModelSwitcher interface {
	SetModel(ctx context.Context, model string) error
	GetModel() string
}
