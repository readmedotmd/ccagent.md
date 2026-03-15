package claude

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	ai "github.com/readmedotmd/ccagent.md/adapter"
	claudecode "github.com/readmedotmd/ccagent.md/internal/claudecode"
)

// DefaultContextWindow is the default context window size in tokens.
const DefaultContextWindow = 200_000

// maxQueueSize is the maximum number of messages that can be queued.
const maxQueueSize = 100

// Sentinel errors for adapter state.
var (
	ErrAdapterRunning    = errors.New("adapter already running")
	ErrAdapterNotRunning = errors.New("adapter not running")
	ErrQueueFull         = errors.New("message queue is full")
)

// Compile-time checks: Adapter + optional interfaces.
var (
	_ ai.Adapter            = (*ClaudeAdapter)(nil)
	_ ai.SessionProvider    = (*ClaudeAdapter)(nil)
	_ ai.HistoryProvider    = (*ClaudeAdapter)(nil)
	_ ai.HistoryClearer     = (*ClaudeAdapter)(nil)
	_ ai.PermissionResponder = (*ClaudeAdapter)(nil)
)

// trackedMessage records a message exchanged with Claude for token estimation.
type trackedMessage struct {
	role    string
	content string
}

// permissionDecision records a pending permission decision.
type permissionDecision struct {
	toolCallID string
	response   ai.ApprovalResponse
}

type ClaudeAdapter struct {
	mu     sync.Mutex
	wg     sync.WaitGroup
	status ai.AdapterStatus
	events chan ai.StreamEvent
	done   chan struct{}
	config ai.AdapterConfig

	// SDK client.
	client    claudecode.Client
	sessionID string

	// Queue and per-run cancellation.
	running   bool
	runCancel context.CancelFunc
	queue     []ai.Message

	// Conversation tracking for context compaction.
	history         []trackedMessage
	estimatedTokens int
	contextWindow   int

	// Permission handling.
	permissionCh chan permissionDecision
	
	// Status change callbacks.
	statusCallbacks []func(ai.AdapterStatus)
}

func NewClaudeAdapter() *ClaudeAdapter {
	return &ClaudeAdapter{
		status:       ai.StatusIdle,
		permissionCh: make(chan permissionDecision, 16),
	}
}

func (c *ClaudeAdapter) Start(ctx context.Context, cfg ai.AdapterConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.status == ai.StatusRunning {
		return ErrAdapterRunning
	}
	c.config = cfg
	c.events = make(chan ai.StreamEvent, 64)
	c.done = make(chan struct{})

	// Build SDK client options — default to safe permission mode.
	permMode := claudecode.PermissionModeDefault
	switch cfg.PermissionMode {
	case ai.PermissionPlan:
		permMode = claudecode.PermissionModePlan
	case ai.PermissionAcceptAll:
		permMode = claudecode.PermissionModeBypassPermissions
	}
	opts := []claudecode.Option{
		claudecode.WithPermissionMode(permMode),
	}
	if cfg.WorkDir != "" {
		opts = append(opts, claudecode.WithCwd(cfg.WorkDir))
	}
	if cfg.SystemPrompt != "" {
		opts = append(opts, claudecode.WithSystemPrompt(cfg.SystemPrompt))
	}
	if cfg.AppendSystemPrompt != "" {
		opts = append(opts, claudecode.WithAppendSystemPrompt(cfg.AppendSystemPrompt))
	}
	if cfg.Model != "" {
		opts = append(opts, claudecode.WithModel(cfg.Model))
	}
	if cfg.MaxThinkingTokens > 0 {
		opts = append(opts, claudecode.WithMaxThinkingTokens(cfg.MaxThinkingTokens))
	}
	if cfg.SessionID != "" {
		opts = append(opts, claudecode.WithResume(cfg.SessionID))
	}
	if cfg.ContinueSession {
		opts = append(opts, claudecode.WithContinueConversation(true))
	}
	if len(cfg.MCPServers) > 0 {
		mcpServers := make(map[string]claudecode.McpServerConfig, len(cfg.MCPServers))
		for name, srv := range cfg.MCPServers {
			mcpServers[name] = &claudecode.McpStdioServerConfig{
				Type:    claudecode.McpServerTypeStdio,
				Command: srv.Command,
				Args:    srv.Args,
				Env:     srv.Env,
			}
		}
		opts = append(opts, claudecode.WithMcpServers(mcpServers))
	}
	if len(cfg.AllowedTools) > 0 {
		opts = append(opts, claudecode.WithAllowedTools(cfg.AllowedTools...))
	}
	if len(cfg.DisallowedTools) > 0 {
		opts = append(opts, claudecode.WithDisallowedTools(cfg.DisallowedTools...))
	}
	if len(cfg.Agents) > 0 {
		agents := make(map[string]claudecode.AgentDefinition, len(cfg.Agents))
		for name, def := range cfg.Agents {
			agents[name] = claudecode.AgentDefinition{
				Description: def.Description,
				Prompt:      def.Prompt,
				Tools:       def.Tools,
				Model:       claudecode.AgentModel(def.Model),
			}
		}
		opts = append(opts, claudecode.WithAgents(agents))
	}

	client := claudecode.NewClient(opts...)
	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("claude sdk connect: %w", err)
	}
	c.client = client

	// Forward client-level transport errors to the events channel.
	c.wg.Add(1)
	go c.forwardClientErrors(client)

	c.contextWindow = cfg.ContextWindow
	if c.contextWindow <= 0 {
		c.contextWindow = DefaultContextWindow
	}

	c.setStatus(ai.StatusRunning)
	return nil
}

// forwardClientErrors drains the client error channel and emits error events.
func (c *ClaudeAdapter) forwardClientErrors(client claudecode.Client) {
	defer c.wg.Done()
	for err := range client.ReceiveErrors() {
		c.emit(ai.StreamEvent{
			Type:      ai.EventError,
			Error:     fmt.Errorf("transport: %w", err),
			Timestamp: time.Now(),
		})
	}
}

// setStatus updates the adapter status and notifies listeners.
func (c *ClaudeAdapter) setStatus(status ai.AdapterStatus) {
	c.status = status
	for _, fn := range c.statusCallbacks {
		fn(status)
	}
}

// emit safely sends an event to the events channel, returning false if the
// adapter has been stopped.
func (c *ClaudeAdapter) emit(event ai.StreamEvent) bool {
	select {
	case <-c.done:
		return false
	case c.events <- event:
		return true
	}
}

func (c *ClaudeAdapter) Send(ctx context.Context, msg ai.Message, opts ...ai.SendOption) error {
	c.mu.Lock()
	if c.status != ai.StatusRunning {
		c.mu.Unlock()
		return ErrAdapterNotRunning
	}

	// Warn if per-turn options are set (not supported by CLI transport).
	if len(opts) > 0 {
		so := &ai.SendOptions{}
		for _, o := range opts {
			o(so)
		}
		if so.MaxTokens > 0 || so.Temperature != 0 || len(so.StopSequences) > 0 || len(so.Tools) > 0 {
			log.Printf("claude: per-turn SendOptions are not supported by the CLI transport and will be ignored")
		}
	}

	if c.running {
		if len(c.queue) >= maxQueueSize {
			c.mu.Unlock()
			return ErrQueueFull
		}
		c.queue = append(c.queue, msg)
		log.Printf("claude: queued message (queue_len=%d)", len(c.queue))
		c.mu.Unlock()
		return nil
	}

	// Start a new run loop.
	c.running = true
	runCtx, cancel := context.WithCancel(ctx)
	c.runCancel = cancel
	c.wg.Add(1)
	c.mu.Unlock()

	go c.runLoop(runCtx, msg)
	return nil
}

// Cancel cancels the in-progress run and clears the queue.
// The adapter remains running and ready for new Send() calls.
func (c *ClaudeAdapter) Cancel() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return nil
	}

	c.queue = nil
	if c.runCancel != nil {
		c.runCancel()
	}

	log.Printf("claude: cancelled in-progress run")
	return nil
}

// runLoop processes messages sequentially, draining the queue after each run.
func (c *ClaudeAdapter) runLoop(ctx context.Context, msg ai.Message) {
	defer c.wg.Done()
	defer func() {
		c.mu.Lock()
		c.running = false
		c.runCancel = nil
		c.mu.Unlock()
	}()

	for {
		c.runClaude(ctx, msg)

		c.mu.Lock()
		if len(c.queue) == 0 {
			c.mu.Unlock()
			return
		}
		msg = combineMessages(c.queue)
		c.queue = nil
		// Cancel the completed context before creating a new one.
		if c.runCancel != nil {
			c.runCancel()
		}
		ctx2, cancel2 := context.WithCancel(context.Background())
		c.runCancel = cancel2
		ctx = ctx2
		c.mu.Unlock()

		log.Printf("claude: processing queued message")
	}
}

// combineMessages merges multiple queued messages into a single message.
func combineMessages(msgs []ai.Message) ai.Message {
	if len(msgs) == 1 {
		return msgs[0]
	}
	var parts []string
	for _, m := range msgs {
		parts = append(parts, messageText(m))
	}
	return ai.Message{
		ID:        msgs[len(msgs)-1].ID,
		Role:      ai.RoleUser,
		Content:   ai.TextContent(strings.Join(parts, "\n\n")),
		Timestamp: msgs[len(msgs)-1].Timestamp,
	}
}

// messageText extracts the concatenated text from a Message's content blocks.
func messageText(msg ai.Message) string {
	var parts []string
	for _, block := range msg.Content {
		if block.Type == ai.ContentText && block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// messageImages extracts image content blocks from a Message.
func messageImages(msg ai.Message) (images [][]byte, mediaTypes []string) {
	for _, block := range msg.Content {
		if block.Type == ai.ContentImage && len(block.Data) > 0 {
			images = append(images, block.Data)
			mt := block.MimeType
			if mt == "" {
				mt = "image/png"
			}
			mediaTypes = append(mediaTypes, mt)
		}
	}
	return
}

// estimateTokens returns a rough token count for the given text.
func estimateTokens(text string) int {
	return utf8.RuneCountInString(text) / 4
}

// runClaude executes a single query via the SDK client and streams events.
func (c *ClaudeAdapter) runClaude(ctx context.Context, msg ai.Message) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	text := messageText(msg)

	// Track the user message and check if compaction is needed.
	c.mu.Lock()
	c.history = append(c.history, trackedMessage{role: "user", content: text})
	c.estimatedTokens += estimateTokens(text)
	needsCompact := c.estimatedTokens > int(float64(c.contextWindow)*0.80)
	client := c.client
	sessionID := c.sessionID
	c.mu.Unlock()

	if needsCompact {
		c.compactSession(ctx)
		c.mu.Lock()
		sessionID = c.sessionID
		c.mu.Unlock()
	}

	// Emit step begin event
	c.emit(ai.StreamEvent{
		Type:      ai.EventStepBegin,
		Timestamp: time.Now(),
		Step: &ai.StepInfo{
			StepNumber: 1,
			TotalSteps: -1,
		},
	})

	// Send the query — use QueryStream for multimodal, Query/QueryWithSession for text-only.
	images, mediaTypes := messageImages(msg)
	var err error
	if len(images) > 0 {
		// Build multimodal content blocks.
		content := []any{
			map[string]any{"type": "text", "text": text},
		}
		for i, imgBytes := range images {
			mediaType := "image/png"
			if i < len(mediaTypes) {
				mediaType = mediaTypes[i]
			}
			content = append(content, map[string]any{
				"type": "image",
				"source": map[string]any{
					"type":       "base64",
					"media_type": mediaType,
					"data":       base64.StdEncoding.EncodeToString(imgBytes),
				},
			})
		}
		sid := sessionID
		if sid == "" {
			sid = "default"
		}
		streamMsg := claudecode.StreamMessage{
			Type: "user",
			Message: map[string]any{
				"role":    "user",
				"content": content,
			},
			SessionID: sid,
		}
		ch := make(chan claudecode.StreamMessage, 1)
		ch <- streamMsg
		close(ch)
		err = client.QueryStream(ctx, ch)
	} else if sessionID != "" {
		err = client.QueryWithSession(ctx, text, sessionID)
	} else {
		err = client.Query(ctx, text)
	}
	if err != nil {
		log.Printf("claude: query error")
		c.emit(ai.StreamEvent{Type: ai.EventError, Error: err, Timestamp: time.Now()})
		return
	}

	// Stream responses.
	var fullContent strings.Builder
	for sdkMsg := range client.ReceiveMessages(ctx) {
		select {
		case <-ctx.Done():
			c.emit(ai.StreamEvent{
				Type:      ai.EventDone,
				Timestamp: time.Now(),
				Message: &ai.Message{
					ID:        msg.ID,
					Role:      ai.RoleAssistant,
					Content:   ai.TextContent(fullContent.String() + "\n\n[cancelled]"),
					Timestamp: time.Now(),
				},
			})
			return
		default:
		}

		switch m := sdkMsg.(type) {
		case *claudecode.AssistantMessage:
			for _, block := range m.Content {
				switch b := block.(type) {
				case *claudecode.TextBlock:
					fullContent.WriteString(b.Text)
					c.emit(ai.StreamEvent{
						Type:      ai.EventToken,
						Token:     b.Text,
						Timestamp: time.Now(),
					})
				case *claudecode.ToolUseBlock:
					var toolInput any
					if len(b.Input) > 0 {
						if data, err := json.Marshal(b.Input); err == nil {
							toolInput = string(data)
						}
					}
					c.emit(ai.StreamEvent{
						Type:       ai.EventToolUse,
						ToolCallID: b.ToolUseID,
						ToolName:   b.Name,
						ToolInput:  toolInput,
						ToolStatus: ai.ToolRunning,
						Timestamp:  time.Now(),
					})
				case *claudecode.ThinkingBlock:
					c.emit(ai.StreamEvent{
						Type:      ai.EventThinking,
						Thinking:  b.Thinking,
						Timestamp: time.Now(),
					})
				}
			}
		case *claudecode.ResultMessage:
			if m.SessionID != "" {
				c.mu.Lock()
				c.sessionID = m.SessionID
				c.mu.Unlock()
			}
			if m.IsError {
				errMsg := ""
				if m.Result != nil {
					errMsg = *m.Result
				}
				log.Printf("claude: ResultMessage error")
				c.emit(ai.StreamEvent{
					Type:      ai.EventError,
					Error:     errors.New(errMsg),
					Timestamp: time.Now(),
				})
				return
			}
			// Track assistant response for token estimation.
			content := fullContent.String()
			c.mu.Lock()
			c.history = append(c.history, trackedMessage{role: "assistant", content: content})
			c.estimatedTokens += estimateTokens(content)
			c.mu.Unlock()
			log.Printf("claude: ResultMessage done, content_len=%d est_tokens=%d", len(content), c.estimatedTokens)

			// Emit cost update if available.
			if m.TotalCostUSD != nil {
				c.emit(ai.StreamEvent{
					Type:      ai.EventCostUpdate,
					Usage:     &ai.TokenUsage{TotalCost: *m.TotalCostUSD},
					Timestamp: time.Now(),
				})
			}

			// Emit step end event
			c.emit(ai.StreamEvent{
				Type:      ai.EventStepEnd,
				Timestamp: time.Now(),
				Step: &ai.StepInfo{
					StepNumber: 1,
					TotalSteps: 1,
				},
			})

			c.emit(ai.StreamEvent{
				Type:      ai.EventDone,
				Timestamp: time.Now(),
				Message: &ai.Message{
					ID:        msg.ID,
					Role:      ai.RoleAssistant,
					Content:   ai.TextContent(content),
					Timestamp: time.Now(),
				},
			})
			return
		}
	}

	// Fallback: if ReceiveMessages closed without a ResultMessage, emit done.
	content := fullContent.String()
	log.Printf("claude: ReceiveMessages ended without ResultMessage, content_len=%d", len(content))
	
	// Emit step end event
	c.emit(ai.StreamEvent{
		Type:      ai.EventStepEnd,
		Timestamp: time.Now(),
		Step: &ai.StepInfo{
			StepNumber: 1,
			TotalSteps: 1,
		},
	})
	
	c.emit(ai.StreamEvent{
		Type:      ai.EventDone,
		Timestamp: time.Now(),
		Message: &ai.Message{
			ID:        msg.ID,
			Role:      ai.RoleAssistant,
			Content:   ai.TextContent(content),
			Timestamp: time.Now(),
		},
	})
}

// compactSession summarizes the conversation history and restarts the session.
func (c *ClaudeAdapter) compactSession(ctx context.Context) {
	c.mu.Lock()
	history := make([]trackedMessage, len(c.history))
	copy(history, c.history)
	client := c.client
	tokensBefore := c.estimatedTokens
	c.mu.Unlock()

	// Build a conversation transcript for summarization.
	var transcript strings.Builder
	for _, m := range history {
		content := m.content
		if len(content) > 1000 {
			content = content[:1000] + "..."
		}
		fmt.Fprintf(&transcript, "[%s]: %s\n", m.role, content)
	}

	log.Printf("claude: compacting session (history=%d messages, est_tokens=%d)", len(history), tokensBefore)

	// Emit compaction begin event
	c.emit(ai.StreamEvent{
		Type:      ai.EventCompactionBegin,
		Timestamp: time.Now(),
		Compaction: &ai.CompactionInfo{
			Reason:       "context_limit",
			TokensBefore: tokensBefore,
		},
	})

	summarizePrompt := "Summarize the following conversation concisely, preserving key decisions, file changes made, and current state. Be brief.\n\n" + transcript.String()

	if err := client.Query(ctx, summarizePrompt); err != nil {
		log.Printf("claude: compaction summary query failed, skipping compaction")
		c.emit(ai.StreamEvent{
			Type:      ai.EventCompactionEnd,
			Timestamp: time.Now(),
			Compaction: &ai.CompactionInfo{
				Reason:       "context_limit",
				TokensBefore: tokensBefore,
				TokensAfter:  tokensBefore,
				Summary:      "[compaction failed]",
			},
		})
		return
	}

	var summary strings.Builder
summaryLoop:
	for sdkMsg := range client.ReceiveMessages(ctx) {
		switch m := sdkMsg.(type) {
		case *claudecode.AssistantMessage:
			for _, block := range m.Content {
				if tb, ok := block.(*claudecode.TextBlock); ok {
					summary.WriteString(tb.Text)
				}
			}
		case *claudecode.ResultMessage:
			break summaryLoop
		}
	}

	summaryStr := summary.String()
	if summaryStr == "" {
		summaryStr = "[summary unavailable]"
	}

	// Reset session: clear sessionID to force a new session, replace history with summary.
	c.mu.Lock()
	c.sessionID = ""
	c.history = []trackedMessage{{role: "user", content: summaryStr}}
	c.estimatedTokens = estimateTokens(summaryStr)
	tokensAfter := c.estimatedTokens
	c.mu.Unlock()

	// Emit compaction end event
	c.emit(ai.StreamEvent{
		Type:      ai.EventCompactionEnd,
		Timestamp: time.Now(),
		Compaction: &ai.CompactionInfo{
			Reason:       "context_limit",
			TokensBefore: tokensBefore,
			TokensAfter:  tokensAfter,
			Summary:      summaryStr,
		},
	})

	// Send the summary as the first message of a new session so Claude has context.
	contextMsg := "[Previous conversation summary]\n" + summaryStr + "\n\nPlease acknowledge you have this context and continue."
	sid := "default"
	streamMsg := claudecode.StreamMessage{
		Type: "user",
		Message: map[string]any{
			"role":    "user",
			"content": contextMsg,
		},
		SessionID: sid,
	}
	ch := make(chan claudecode.StreamMessage, 1)
	ch <- streamMsg
	close(ch)
	if err := client.QueryStream(ctx, ch); err != nil {
		log.Printf("claude: compaction context send failed")
		return
	}

	// Consume the acknowledgement response and capture the new session ID.
	for sdkMsg := range client.ReceiveMessages(ctx) {
		if rm, ok := sdkMsg.(*claudecode.ResultMessage); ok {
			if rm.SessionID != "" {
				c.mu.Lock()
				c.sessionID = rm.SessionID
				c.mu.Unlock()
			}
			break
		}
	}

	log.Printf("claude: compaction complete, est_tokens=%d", tokensAfter)
}

// SessionID implements the optional ai.SessionProvider interface.
func (c *ClaudeAdapter) SessionID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sessionID
}

// GetHistory implements the optional ai.HistoryProvider interface.
func (c *ClaudeAdapter) GetHistory(ctx context.Context) ([]ai.Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var msgs []ai.Message
	for _, tm := range c.history {
		role := ai.RoleUser
		if tm.role == "assistant" {
			role = ai.RoleAssistant
		}
		msgs = append(msgs, ai.Message{
			Role:      role,
			Content:   ai.TextContent(tm.content),
			Timestamp: time.Now(),
		})
	}
	return msgs, nil
}

// ClearHistory implements the optional ai.HistoryClearer interface.
func (c *ClaudeAdapter) ClearHistory(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.history = nil
	c.estimatedTokens = 0
	return nil
}

// RespondPermission implements the optional ai.PermissionResponder interface.
// Note: Claude Code CLI doesn't support dynamic permission responses during a turn.
// This method stores the decision for future reference but cannot affect in-flight requests.
func (c *ClaudeAdapter) RespondPermission(ctx context.Context, toolCallID string, response ai.ApprovalResponse) error {
	select {
	case c.permissionCh <- permissionDecision{toolCallID: toolCallID, response: response}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// OnStatusChange implements the optional ai.StatusListener interface.
func (c *ClaudeAdapter) OnStatusChange(fn func(ai.AdapterStatus)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.statusCallbacks = append(c.statusCallbacks, fn)
}

func (c *ClaudeAdapter) Receive() <-chan ai.StreamEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.events
}

func (c *ClaudeAdapter) Stop() error {
	c.mu.Lock()
	if c.status != ai.StatusRunning {
		c.mu.Unlock()
		return nil
	}
	close(c.done)
	if c.runCancel != nil {
		c.runCancel()
	}
	if c.client != nil {
		c.client.Disconnect()
	}
	c.setStatus(ai.StatusStopped)
	c.mu.Unlock()

	// Wait for all goroutines to finish before closing the events channel.
	c.wg.Wait()
	close(c.events)
	return nil
}

func (c *ClaudeAdapter) Status() ai.AdapterStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.status
}

// Capabilities returns the features supported by the Claude adapter.
func (c *ClaudeAdapter) Capabilities() ai.AdapterCapabilities {
	return ai.AdapterCapabilities{
		SupportsStreaming:    true,
		SupportsImages:       true,
		SupportsFiles:        true,
		SupportsToolUse:      true,
		SupportsMCP:          true,
		SupportsThinking:     true,
		SupportsCancellation: true,
		SupportsHistory:      true,
		SupportsSubAgents:    true,
		MaxContextWindow:     DefaultContextWindow,
	}
}

// Health checks whether the adapter is in a healthy state.
func (c *ClaudeAdapter) Health(_ context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch c.status {
	case ai.StatusRunning:
		return nil
	case ai.StatusError:
		return &ai.AdapterError{Code: ai.ErrCrashed, Message: "adapter in error state"}
	case ai.StatusStopped:
		return &ai.AdapterError{Code: ai.ErrCrashed, Message: "adapter stopped"}
	default:
		return &ai.AdapterError{Code: ai.ErrUnknown, Message: "adapter not started"}
	}
}
