package claudecode

import (
	"testing"
)

func TestNewOptionsDefaults(t *testing.T) {
	o := NewOptions()
	if o.MaxThinkingTokens != 8000 {
		t.Fatalf("expected 8000, got %d", o.MaxThinkingTokens)
	}
	if o.McpServers == nil {
		t.Fatal("expected non-nil McpServers")
	}
	if o.ExtraArgs == nil {
		t.Fatal("expected non-nil ExtraArgs")
	}
	if o.ExtraEnv == nil {
		t.Fatal("expected non-nil ExtraEnv")
	}
	if o.SystemPrompt != nil || o.Model != nil || o.Cwd != nil {
		t.Fatal("expected nil pointer fields")
	}
}

func TestWithAllowedTools(t *testing.T) {
	o := NewOptions(WithAllowedTools("bash", "read", "write"))
	if len(o.AllowedTools) != 3 || o.AllowedTools[0] != "bash" {
		t.Fatalf("unexpected tools: %v", o.AllowedTools)
	}
}

func TestWithDisallowedTools(t *testing.T) {
	o := NewOptions(WithDisallowedTools("delete"))
	if len(o.DisallowedTools) != 1 || o.DisallowedTools[0] != "delete" {
		t.Fatalf("unexpected tools: %v", o.DisallowedTools)
	}
}

func TestWithSystemPrompt(t *testing.T) {
	o := NewOptions(WithSystemPrompt("You are helpful"))
	if o.SystemPrompt == nil || *o.SystemPrompt != "You are helpful" {
		t.Fatal("unexpected system prompt")
	}
}

func TestWithAppendSystemPrompt(t *testing.T) {
	o := NewOptions(WithAppendSystemPrompt("Be concise"))
	if o.AppendSystemPrompt == nil || *o.AppendSystemPrompt != "Be concise" {
		t.Fatal("unexpected append system prompt")
	}
}

func TestWithModel(t *testing.T) {
	o := NewOptions(WithModel("claude-sonnet-4-20250514"))
	if o.Model == nil || *o.Model != "claude-sonnet-4-20250514" {
		t.Fatal("unexpected model")
	}
}

func TestWithMaxThinkingTokens(t *testing.T) {
	o := NewOptions(WithMaxThinkingTokens(16000))
	if o.MaxThinkingTokens != 16000 {
		t.Fatalf("expected 16000, got %d", o.MaxThinkingTokens)
	}
}

func TestWithPermissionMode(t *testing.T) {
	o := NewOptions(WithPermissionMode(PermissionModePlan))
	if o.PermissionMode == nil || *o.PermissionMode != PermissionModePlan {
		t.Fatal("unexpected permission mode")
	}
}

func TestWithContinueConversation(t *testing.T) {
	o := NewOptions(WithContinueConversation(true))
	if !o.ContinueConversation {
		t.Fatal("expected true")
	}
}

func TestWithResume(t *testing.T) {
	o := NewOptions(WithResume("sess-123"))
	if o.Resume == nil || *o.Resume != "sess-123" {
		t.Fatal("unexpected resume")
	}
}

func TestWithCwd(t *testing.T) {
	o := NewOptions(WithCwd("/tmp/work"))
	if o.Cwd == nil || *o.Cwd != "/tmp/work" {
		t.Fatal("unexpected cwd")
	}
}

func TestWithMcpServers(t *testing.T) {
	servers := map[string]McpServerConfig{
		"test": &McpStdioServerConfig{
			Type:    McpServerTypeStdio,
			Command: "node",
			Args:    []string{"server.js"},
			Env:     map[string]string{"PORT": "3000"},
		},
	}
	o := NewOptions(WithMcpServers(servers))
	if len(o.McpServers) != 1 {
		t.Fatal("expected 1 server")
	}
	srv := o.McpServers["test"]
	if srv.GetType() != McpServerTypeStdio {
		t.Fatal("unexpected server type")
	}
}

func TestWithAgents(t *testing.T) {
	agents := map[string]AgentDefinition{
		"researcher": {
			Description: "Research agent",
			Prompt:      "You research things",
			Tools:       []string{"read", "grep"},
			Model:       AgentModelSonnet,
		},
	}
	o := NewOptions(WithAgents(agents))
	if len(o.Agents) != 1 {
		t.Fatal("expected 1 agent")
	}
	a := o.Agents["researcher"]
	if a.Description != "Research agent" {
		t.Fatal("unexpected description")
	}
	if a.Model != AgentModelSonnet {
		t.Fatal("unexpected model")
	}
}

func TestWithCLIPath(t *testing.T) {
	o := NewOptions(WithCLIPath("/usr/local/bin/claude"))
	if o.CLIPath == nil || *o.CLIPath != "/usr/local/bin/claude" {
		t.Fatal("unexpected CLI path")
	}
}

func TestMultipleOptions(t *testing.T) {
	o := NewOptions(
		WithModel("opus"),
		WithSystemPrompt("Be helpful"),
		WithMaxThinkingTokens(4000),
		WithCwd("/home/user"),
		WithAllowedTools("read", "write"),
		WithPermissionMode(PermissionModeBypassPermissions),
	)
	if *o.Model != "opus" {
		t.Fatal("unexpected model")
	}
	if *o.SystemPrompt != "Be helpful" {
		t.Fatal("unexpected prompt")
	}
	if o.MaxThinkingTokens != 4000 {
		t.Fatal("unexpected thinking tokens")
	}
	if *o.Cwd != "/home/user" {
		t.Fatal("unexpected cwd")
	}
	if len(o.AllowedTools) != 2 {
		t.Fatal("unexpected tools count")
	}
	if *o.PermissionMode != PermissionModeBypassPermissions {
		t.Fatal("unexpected permission mode")
	}
}
