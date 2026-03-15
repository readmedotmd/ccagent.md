package claude

import (
	"context"
	"errors"
	"testing"
	"time"

	ai "github.com/readmedotmd/ccagent.md/adapter"
)

// --- Interface compliance ---

func TestCompileTimeInterfaceChecks(t *testing.T) {
	var _ ai.Adapter = (*ClaudeAdapter)(nil)
	var _ ai.SessionProvider = (*ClaudeAdapter)(nil)
}

// --- Status lifecycle ---

func TestNewClaudeAdapterInitialStatus(t *testing.T) {
	a := NewClaudeAdapter()
	if a.Status() != ai.StatusIdle {
		t.Fatalf("expected StatusIdle, got %d", a.Status())
	}
}

func TestStopOnIdleIsNoOp(t *testing.T) {
	a := NewClaudeAdapter()
	if err := a.Stop(); err != nil {
		t.Fatalf("stop on idle should succeed: %v", err)
	}
	if a.Status() != ai.StatusIdle {
		t.Fatal("expected still idle after stop")
	}
}

func TestDoubleStartReturnsError(t *testing.T) {
	a := NewClaudeAdapter()
	a.mu.Lock()
	a.status = ai.StatusRunning
	a.events = make(chan ai.StreamEvent, 64)
	a.done = make(chan struct{})
	a.mu.Unlock()

	cfg := ai.AdapterConfig{Name: "test"}
	err := a.Start(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error on double start")
	}
	if !errors.Is(err, ErrAdapterRunning) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStopOnRunningAdapter(t *testing.T) {
	a := NewClaudeAdapter()
	a.mu.Lock()
	a.status = ai.StatusRunning
	a.events = make(chan ai.StreamEvent, 64)
	a.done = make(chan struct{})
	a.mu.Unlock()

	if err := a.Stop(); err != nil {
		t.Fatalf("stop should succeed: %v", err)
	}
	if a.Status() != ai.StatusStopped {
		t.Fatalf("expected StatusStopped, got %d", a.Status())
	}
}

func TestStopClosesEventsChannel(t *testing.T) {
	a := NewClaudeAdapter()
	a.mu.Lock()
	a.status = ai.StatusRunning
	a.events = make(chan ai.StreamEvent, 64)
	a.done = make(chan struct{})
	a.mu.Unlock()

	ch := a.Receive()
	if err := a.Stop(); err != nil {
		t.Fatal(err)
	}
	// Channel should be closed after Stop.
	_, ok := <-ch
	if ok {
		t.Fatal("expected events channel to be closed after Stop")
	}
}

func TestStopOnAlreadyStoppedIsNoOp(t *testing.T) {
	a := NewClaudeAdapter()
	a.mu.Lock()
	a.status = ai.StatusStopped
	a.mu.Unlock()

	if err := a.Stop(); err != nil {
		t.Fatalf("stop on stopped should succeed: %v", err)
	}
}

// --- Send ---

func TestSendWhenNotRunning(t *testing.T) {
	a := NewClaudeAdapter()
	msg := ai.Message{Content: ai.TextContent("hello")}
	err := a.Send(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error when not running")
	}
	if !errors.Is(err, ErrAdapterNotRunning) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSendQueuesWhenRunning(t *testing.T) {
	a := NewClaudeAdapter()
	a.mu.Lock()
	a.status = ai.StatusRunning
	a.events = make(chan ai.StreamEvent, 64)
	a.done = make(chan struct{})
	a.running = true // simulate an active run
	a.mu.Unlock()

	msg := ai.Message{Content: ai.TextContent("queued message")}
	err := a.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("send should succeed (queue): %v", err)
	}

	a.mu.Lock()
	qLen := len(a.queue)
	a.mu.Unlock()

	if qLen != 1 {
		t.Fatalf("expected 1 queued message, got %d", qLen)
	}
}

func TestSendMultipleQueuedMessages(t *testing.T) {
	a := NewClaudeAdapter()
	a.mu.Lock()
	a.status = ai.StatusRunning
	a.events = make(chan ai.StreamEvent, 64)
	a.done = make(chan struct{})
	a.running = true
	a.mu.Unlock()

	for i := 0; i < 5; i++ {
		msg := ai.Message{Content: ai.TextContent("msg")}
		if err := a.Send(context.Background(), msg); err != nil {
			t.Fatal(err)
		}
	}

	a.mu.Lock()
	qLen := len(a.queue)
	a.mu.Unlock()

	if qLen != 5 {
		t.Fatalf("expected 5 queued messages, got %d", qLen)
	}
}

func TestSendQueueFull(t *testing.T) {
	a := NewClaudeAdapter()
	a.mu.Lock()
	a.status = ai.StatusRunning
	a.events = make(chan ai.StreamEvent, 64)
	a.done = make(chan struct{})
	a.running = true
	a.queue = make([]ai.Message, maxQueueSize)
	a.mu.Unlock()

	msg := ai.Message{Content: ai.TextContent("overflow")}
	err := a.Send(context.Background(), msg)
	if !errors.Is(err, ErrQueueFull) {
		t.Fatalf("expected ErrQueueFull, got %v", err)
	}
}

// --- Cancel ---

func TestCancelWhenNotRunning(t *testing.T) {
	a := NewClaudeAdapter()
	if err := a.Cancel(); err != nil {
		t.Fatalf("cancel when not running should be no-op: %v", err)
	}
}

func TestCancelClearsQueue(t *testing.T) {
	a := NewClaudeAdapter()
	a.mu.Lock()
	a.status = ai.StatusRunning
	a.running = true
	a.queue = []ai.Message{
		{Content: ai.TextContent("1")},
		{Content: ai.TextContent("2")},
	}
	ctx, cancel := context.WithCancel(context.Background())
	_ = ctx
	a.runCancel = cancel
	a.mu.Unlock()

	if err := a.Cancel(); err != nil {
		t.Fatal(err)
	}

	a.mu.Lock()
	qLen := len(a.queue)
	a.mu.Unlock()

	if qLen != 0 {
		t.Fatalf("expected empty queue after cancel, got %d", qLen)
	}
}

// --- Receive ---

func TestReceiveReturnsChannel(t *testing.T) {
	a := NewClaudeAdapter()
	a.mu.Lock()
	a.events = make(chan ai.StreamEvent, 10)
	a.mu.Unlock()

	ch := a.Receive()
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}
}

func TestReceiveNilWhenNoEvents(t *testing.T) {
	a := NewClaudeAdapter()
	ch := a.Receive()
	if ch != nil {
		t.Fatal("expected nil channel when events not initialized")
	}
}

// --- Capabilities ---

func TestCapabilities(t *testing.T) {
	a := NewClaudeAdapter()
	caps := a.Capabilities()

	if !caps.SupportsStreaming {
		t.Fatal("expected streaming support")
	}
	if !caps.SupportsImages {
		t.Fatal("expected image support")
	}
	if !caps.SupportsFiles {
		t.Fatal("expected file support")
	}
	if !caps.SupportsToolUse {
		t.Fatal("expected tool use support")
	}
	if !caps.SupportsMCP {
		t.Fatal("expected MCP support")
	}
	if !caps.SupportsThinking {
		t.Fatal("expected thinking support")
	}
	if !caps.SupportsCancellation {
		t.Fatal("expected cancellation support")
	}
	if !caps.SupportsHistory {
		t.Fatal("expected history support")
	}
	if !caps.SupportsSubAgents {
		t.Fatal("expected sub-agent support")
	}
	if caps.MaxContextWindow != DefaultContextWindow {
		t.Fatalf("expected %d, got %d", DefaultContextWindow, caps.MaxContextWindow)
	}
}

// --- Health ---

func TestHealthRunning(t *testing.T) {
	a := NewClaudeAdapter()
	a.mu.Lock()
	a.status = ai.StatusRunning
	a.mu.Unlock()

	if err := a.Health(context.Background()); err != nil {
		t.Fatalf("expected healthy: %v", err)
	}
}

func TestHealthError(t *testing.T) {
	a := NewClaudeAdapter()
	a.mu.Lock()
	a.status = ai.StatusError
	a.mu.Unlock()

	err := a.Health(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	ae, ok := err.(*ai.AdapterError)
	if !ok {
		t.Fatalf("expected AdapterError, got %T", err)
	}
	if ae.Code != ai.ErrCrashed {
		t.Fatalf("expected ErrCrashed, got %d", ae.Code)
	}
}

func TestHealthStopped(t *testing.T) {
	a := NewClaudeAdapter()
	a.mu.Lock()
	a.status = ai.StatusStopped
	a.mu.Unlock()

	err := a.Health(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	ae := err.(*ai.AdapterError)
	if ae.Code != ai.ErrCrashed {
		t.Fatalf("expected ErrCrashed, got %d", ae.Code)
	}
}

func TestHealthIdle(t *testing.T) {
	a := NewClaudeAdapter()
	err := a.Health(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	ae := err.(*ai.AdapterError)
	if ae.Code != ai.ErrUnknown {
		t.Fatalf("expected ErrUnknown, got %d", ae.Code)
	}
}

// --- SessionID ---

func TestSessionIDInitiallyEmpty(t *testing.T) {
	a := NewClaudeAdapter()
	if a.SessionID() != "" {
		t.Fatal("expected empty session ID initially")
	}
}

func TestSessionIDAfterSet(t *testing.T) {
	a := NewClaudeAdapter()
	a.mu.Lock()
	a.sessionID = "sess-abc"
	a.mu.Unlock()

	if a.SessionID() != "sess-abc" {
		t.Fatalf("expected 'sess-abc', got %q", a.SessionID())
	}
}

// --- Helper functions ---

func TestMessageText(t *testing.T) {
	tests := []struct {
		name string
		msg  ai.Message
		want string
	}{
		{
			name: "single text block",
			msg:  ai.Message{Content: ai.TextContent("hello")},
			want: "hello",
		},
		{
			name: "multiple text blocks",
			msg: ai.Message{Content: []ai.ContentBlock{
				{Type: ai.ContentText, Text: "line1"},
				{Type: ai.ContentText, Text: "line2"},
			}},
			want: "line1\nline2",
		},
		{
			name: "mixed blocks - only text extracted",
			msg: ai.Message{Content: []ai.ContentBlock{
				{Type: ai.ContentText, Text: "hello"},
				{Type: ai.ContentImage, Data: []byte{1, 2, 3}},
				{Type: ai.ContentText, Text: "world"},
			}},
			want: "hello\nworld",
		},
		{
			name: "no text blocks",
			msg: ai.Message{Content: []ai.ContentBlock{
				{Type: ai.ContentImage, Data: []byte{1}},
			}},
			want: "",
		},
		{
			name: "empty content",
			msg:  ai.Message{Content: nil},
			want: "",
		},
		{
			name: "empty text blocks skipped",
			msg: ai.Message{Content: []ai.ContentBlock{
				{Type: ai.ContentText, Text: ""},
				{Type: ai.ContentText, Text: "real"},
			}},
			want: "real",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := messageText(tt.msg)
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestMessageImages(t *testing.T) {
	msg := ai.Message{Content: []ai.ContentBlock{
		{Type: ai.ContentText, Text: "look at this"},
		{Type: ai.ContentImage, Data: []byte{0x89, 0x50}, MimeType: "image/png"},
		{Type: ai.ContentImage, Data: []byte{0xFF, 0xD8}, MimeType: "image/jpeg"},
		{Type: ai.ContentImage, Data: []byte{0x00}}, // no mime type
	}}

	images, mediaTypes := messageImages(msg)
	if len(images) != 3 {
		t.Fatalf("expected 3 images, got %d", len(images))
	}
	if len(mediaTypes) != 3 {
		t.Fatalf("expected 3 media types, got %d", len(mediaTypes))
	}
	if mediaTypes[0] != "image/png" {
		t.Fatalf("expected image/png, got %s", mediaTypes[0])
	}
	if mediaTypes[1] != "image/jpeg" {
		t.Fatalf("expected image/jpeg, got %s", mediaTypes[1])
	}
	if mediaTypes[2] != "image/png" {
		t.Fatalf("expected default image/png, got %s", mediaTypes[2])
	}
}

func TestMessageImagesNoImages(t *testing.T) {
	msg := ai.Message{Content: ai.TextContent("just text")}
	images, mediaTypes := messageImages(msg)
	if len(images) != 0 || len(mediaTypes) != 0 {
		t.Fatal("expected no images")
	}
}

func TestMessageImagesEmptyData(t *testing.T) {
	msg := ai.Message{Content: []ai.ContentBlock{
		{Type: ai.ContentImage, Data: nil},
		{Type: ai.ContentImage, Data: []byte{}},
	}}
	images, _ := messageImages(msg)
	if len(images) != 0 {
		t.Fatal("expected no images for empty data")
	}
}

func TestCombineMessagesSingle(t *testing.T) {
	msgs := []ai.Message{
		{ID: "1", Content: ai.TextContent("only"), Timestamp: time.Now()},
	}
	combined := combineMessages(msgs)
	if combined.ID != "1" {
		t.Fatalf("expected ID '1', got %q", combined.ID)
	}
	if messageText(combined) != "only" {
		t.Fatalf("unexpected content: %q", messageText(combined))
	}
}

func TestCombineMessagesMultiple(t *testing.T) {
	msgs := []ai.Message{
		{ID: "1", Content: ai.TextContent("first")},
		{ID: "2", Content: ai.TextContent("second")},
		{ID: "3", Content: ai.TextContent("third")},
	}
	combined := combineMessages(msgs)
	if combined.ID != "3" {
		t.Fatalf("expected ID '3', got %q", combined.ID)
	}
	if combined.Role != ai.RoleUser {
		t.Fatalf("expected user role, got %s", combined.Role)
	}
	text := messageText(combined)
	expected := "first\n\nsecond\n\nthird"
	if text != expected {
		t.Fatalf("expected %q, got %q", expected, text)
	}
}

func TestCombineMessagesPreservesLastTimestamp(t *testing.T) {
	t1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	msgs := []ai.Message{
		{ID: "1", Content: ai.TextContent("a"), Timestamp: t1},
		{ID: "2", Content: ai.TextContent("b"), Timestamp: t2},
	}
	combined := combineMessages(msgs)
	if !combined.Timestamp.Equal(t2) {
		t.Fatalf("expected last timestamp, got %v", combined.Timestamp)
	}
}

func TestCombineMessagesWithMultipleContentBlocks(t *testing.T) {
	msgs := []ai.Message{
		{ID: "1", Content: []ai.ContentBlock{
			{Type: ai.ContentText, Text: "part1"},
			{Type: ai.ContentText, Text: "part2"},
		}},
		{ID: "2", Content: ai.TextContent("second")},
	}
	combined := combineMessages(msgs)
	text := messageText(combined)
	expected := "part1\npart2\n\nsecond"
	if text != expected {
		t.Fatalf("expected %q, got %q", expected, text)
	}
}

func TestEstimateTokens(t *testing.T) {
	if got := estimateTokens("hello world"); got != 2 {
		t.Fatalf("expected 2, got %d", got)
	}
	if got := estimateTokens(""); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
}

// --- Start with cancelled context ---

func TestStartWithCancelledContext(t *testing.T) {
	a := NewClaudeAdapter()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := ai.AdapterConfig{Name: "test"}
	err := a.Start(ctx, cfg)
	if err == nil {
		a.Stop()
	}
}

// --- AdapterConfig mapping ---

func TestAdapterConfigPermissionModes(t *testing.T) {
	if ai.PermissionDefault != "default" {
		t.Fatal("unexpected default")
	}
	if ai.PermissionAcceptAll != "accept_all" {
		t.Fatal("unexpected accept_all")
	}
	if ai.PermissionPlan != "plan" {
		t.Fatal("unexpected plan")
	}
}

func TestAdapterConfigFields(t *testing.T) {
	cfg := ai.AdapterConfig{
		Name:               "claude",
		WorkDir:            "/tmp",
		SystemPrompt:       "Be helpful",
		AppendSystemPrompt: "Be concise",
		Model:              "opus",
		MaxThinkingTokens:  16000,
		PermissionMode:     ai.PermissionPlan,
		SessionID:          "sess-1",
		ContinueSession:    true,
		ContextWindow:      100000,
		AllowedTools:       []string{"read"},
		DisallowedTools:    []string{"delete"},
		MCPServers: map[string]ai.MCPServerConfig{
			"test": {Command: "node", Args: []string{"s.js"}, Env: map[string]string{"K": "V"}},
		},
		Agents: map[string]ai.AgentDef{
			"helper": {Description: "desc", Prompt: "prompt", Tools: []string{"t"}, Model: "sonnet"},
		},
	}
	if cfg.Name != "claude" || cfg.ContextWindow != 100000 {
		t.Fatal("unexpected config fields")
	}
}

// --- SendOption functional options ---

func TestSendOptionsApply(t *testing.T) {
	opts := &ai.SendOptions{}
	ai.WithMaxTokens(2048)(opts)
	ai.WithTemperature(0.5)(opts)
	ai.WithStopSequences([]string{"END"})(opts)
	ai.WithTools([]string{"bash"})(opts)

	if opts.MaxTokens != 2048 {
		t.Fatal("unexpected max tokens")
	}
	if opts.Temperature != 0.5 {
		t.Fatal("unexpected temperature")
	}
	if len(opts.StopSequences) != 1 || opts.StopSequences[0] != "END" {
		t.Fatal("unexpected stop sequences")
	}
	if len(opts.Tools) != 1 || opts.Tools[0] != "bash" {
		t.Fatal("unexpected tools")
	}
}

// --- Sentinel errors ---

func TestSentinelErrors(t *testing.T) {
	if ErrAdapterRunning.Error() != "adapter already running" {
		t.Fatalf("unexpected: %s", ErrAdapterRunning)
	}
	if ErrAdapterNotRunning.Error() != "adapter not running" {
		t.Fatalf("unexpected: %s", ErrAdapterNotRunning)
	}
	if ErrQueueFull.Error() != "message queue is full" {
		t.Fatalf("unexpected: %s", ErrQueueFull)
	}
}

func TestDefaultContextWindowConstant(t *testing.T) {
	if DefaultContextWindow != 200_000 {
		t.Fatalf("expected 200000, got %d", DefaultContextWindow)
	}
}
