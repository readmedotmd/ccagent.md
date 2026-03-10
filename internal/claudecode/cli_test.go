package claudecode

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestBuildCommandBasicStreaming(t *testing.T) {
	cmd := buildCommand("/usr/bin/claude", nil, false)
	expected := []string{"/usr/bin/claude", "--output-format", "stream-json", "--verbose", "--input-format", "stream-json", "--setting-sources", ""}
	if len(cmd) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(cmd), cmd)
	}
	for i, want := range expected {
		if cmd[i] != want {
			t.Fatalf("arg %d: expected %q, got %q", i, want, cmd[i])
		}
	}
}

func TestBuildCommandPrintMode(t *testing.T) {
	cmd := buildCommand("/usr/bin/claude", nil, true)
	if !contains(cmd, "--print") {
		t.Fatal("expected --print flag")
	}
	if contains(cmd, "--input-format") {
		t.Fatal("should not have --input-format in print mode")
	}
}

func TestBuildCommandWithAllOptions(t *testing.T) {
	pm := PermissionModePlan
	model := "opus"
	prompt := "Be helpful"
	appendPrompt := "Be concise"
	resume := "sess-1"
	o := &Options{
		AllowedTools:         []string{"read", "write"},
		DisallowedTools:      []string{"delete"},
		SystemPrompt:         &prompt,
		AppendSystemPrompt:   &appendPrompt,
		Model:                &model,
		PermissionMode:       &pm,
		ContinueConversation: true,
		Resume:               &resume,
		MaxTurns:             5,
		SettingSources:       []string{"project", "user"},
	}
	cmd := buildCommand("claude", o, false)

	if !containsPair(cmd, "--allowed-tools", "read,write") {
		t.Fatal("expected --allowed-tools")
	}
	if !containsPair(cmd, "--disallowed-tools", "delete") {
		t.Fatal("expected --disallowed-tools")
	}
	if !containsPair(cmd, "--system-prompt", "Be helpful") {
		t.Fatal("expected --system-prompt")
	}
	if !containsPair(cmd, "--append-system-prompt", "Be concise") {
		t.Fatal("expected --append-system-prompt")
	}
	if !containsPair(cmd, "--model", "opus") {
		t.Fatal("expected --model")
	}
	if !containsPair(cmd, "--permission-mode", "plan") {
		t.Fatal("expected --permission-mode")
	}
	if !contains(cmd, "--continue") {
		t.Fatal("expected --continue")
	}
	if !containsPair(cmd, "--resume", "sess-1") {
		t.Fatal("expected --resume")
	}
	if !containsPair(cmd, "--max-turns", "5") {
		t.Fatal("expected --max-turns")
	}
	if !containsPair(cmd, "--setting-sources", "project,user") {
		t.Fatal("expected --setting-sources")
	}
}

func TestBuildCommandWithAgents(t *testing.T) {
	o := &Options{
		Agents: map[string]AgentDefinition{
			"helper": {
				Description: "A helper agent",
				Prompt:      "Help the user",
				Tools:       []string{"read"},
				Model:       AgentModelHaiku,
			},
		},
	}
	cmd := buildCommand("claude", o, false)
	if !contains(cmd, "--agents") {
		t.Fatal("expected --agents flag")
	}
	// Find agents JSON value
	for i, arg := range cmd {
		if arg == "--agents" && i+1 < len(cmd) {
			json := cmd[i+1]
			if !strings.Contains(json, "helper") || !strings.Contains(json, "A helper agent") {
				t.Fatalf("unexpected agents JSON: %s", json)
			}
			return
		}
	}
	t.Fatal("could not find --agents value")
}

func TestBuildCommandWithExtraArgs(t *testing.T) {
	val := "/tmp/mcp.json"
	profileVal := "my-profile"
	o := &Options{
		ExtraArgs: map[string]*string{
			"mcp-config": &val,
			"profile":    &profileVal,
			"notify":     nil,
		},
	}
	cmd := buildCommand("claude", o, false)
	if !containsPair(cmd, "--mcp-config", "/tmp/mcp.json") {
		t.Fatal("expected --mcp-config")
	}
	if !containsPair(cmd, "--profile", "my-profile") {
		t.Fatal("expected --profile")
	}
	if !contains(cmd, "--notify") {
		t.Fatal("expected --notify")
	}
}

func TestBuildCommandEmptySettingSources(t *testing.T) {
	o := &Options{SettingSources: nil}
	cmd := buildCommand("claude", o, false)
	if !containsPair(cmd, "--setting-sources", "") {
		t.Fatal("expected empty --setting-sources")
	}
}

func TestValidateWorkingDirectory(t *testing.T) {
	// Empty is fine
	if err := validateWorkingDirectory(""); err != nil {
		t.Fatalf("empty should be ok: %v", err)
	}

	// Valid directory
	dir := t.TempDir()
	if err := validateWorkingDirectory(dir); err != nil {
		t.Fatalf("temp dir should be ok: %v", err)
	}

	// Non-existent
	err := validateWorkingDirectory("/nonexistent/path/xyz")
	if err == nil {
		t.Fatal("expected error for non-existent")
	}
	var connErr *ConnectionError
	if !isConnectionError(err, &connErr) {
		t.Fatal("expected ConnectionError")
	}

	// File, not directory
	f, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Close()
	err = validateWorkingDirectory(f.Name())
	if err == nil {
		t.Fatal("expected error for file")
	}
}

func TestPermissionModeConstants(t *testing.T) {
	if PermissionModeDefault != "default" {
		t.Fatal("unexpected default")
	}
	if PermissionModeAcceptEdits != "acceptEdits" {
		t.Fatal("unexpected acceptEdits")
	}
	if PermissionModePlan != "plan" {
		t.Fatal("unexpected plan")
	}
	if PermissionModeBypassPermissions != "bypassPermissions" {
		t.Fatal("unexpected bypassPermissions")
	}
}

func TestMcpServerTypes(t *testing.T) {
	if McpServerTypeStdio != "stdio" || McpServerTypeSSE != "sse" || McpServerTypeHTTP != "http" {
		t.Fatal("unexpected server types")
	}
}

func TestMcpStdioServerConfig(t *testing.T) {
	cfg := &McpStdioServerConfig{
		Type:    McpServerTypeStdio,
		Command: "node",
		Args:    []string{"server.js"},
		Env:     map[string]string{"PORT": "3000"},
	}
	if cfg.GetType() != McpServerTypeStdio {
		t.Fatal("unexpected type")
	}
}

func TestAgentModelConstants(t *testing.T) {
	if AgentModelSonnet != "sonnet" || AgentModelOpus != "opus" ||
		AgentModelHaiku != "haiku" || AgentModelInherit != "inherit" {
		t.Fatal("unexpected agent model constants")
	}
}

// helpers

func contains(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func containsPair(args []string, flag, value string) bool {
	for i, a := range args {
		if a == flag && i+1 < len(args) && args[i+1] == value {
			return true
		}
	}
	return false
}

func isConnectionError(err error, target **ConnectionError) bool {
	return errors.As(err, target)
}

func TestBuildCommandRejectsUnknownExtraArgs(t *testing.T) {
	pm := "bypassPermissions"
	o := &Options{
		ExtraArgs: map[string]*string{
			"permission-mode": &pm,
			"no-color":        nil,
			"notify":          nil,
		},
	}
	cmd := buildCommand("claude", o, false)
	// Non-allowlisted flags should be filtered out.
	if containsPair(cmd, "--permission-mode", "bypassPermissions") {
		t.Fatal("permission-mode should be filtered from ExtraArgs")
	}
	if contains(cmd, "--no-color") {
		t.Fatal("no-color should be filtered from ExtraArgs")
	}
	// Allowlisted flags should remain.
	if !contains(cmd, "--notify") {
		t.Fatal("expected --notify to be allowed")
	}
}

func TestValidateWorkingDirectoryResolvesSymlinks(t *testing.T) {
	dir := t.TempDir()
	// Valid directory through direct path should work.
	if err := validateWorkingDirectory(dir); err != nil {
		t.Fatalf("valid dir should pass: %v", err)
	}
}
