package claudecode

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestNewTransport(t *testing.T) {
	tr := newTransport("/usr/bin/claude", nil)
	if tr.cliPath != "/usr/bin/claude" {
		t.Fatal("unexpected CLI path")
	}
	if tr.connected {
		t.Fatal("should not be connected")
	}
	if tr.jsonParser == nil {
		t.Fatal("expected parser")
	}
}

func TestTransportSendMessageNotConnected(t *testing.T) {
	tr := newTransport("claude", nil)
	msg := StreamMessage{Type: "user", Message: "hello"}
	err := tr.sendMessage(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error when not connected")
	}
}

func TestTransportReceiveNotConnected(t *testing.T) {
	tr := newTransport("claude", nil)
	msgCh, errCh := tr.receiveMessages(context.Background())
	// Channels should be closed immediately
	if _, ok := <-msgCh; ok {
		t.Fatal("expected closed msg channel")
	}
	if _, ok := <-errCh; ok {
		t.Fatal("expected closed err channel")
	}
}

func TestTransportInterruptNotRunning(t *testing.T) {
	tr := newTransport("claude", nil)
	err := tr.interrupt(context.Background())
	if err == nil {
		t.Fatal("expected error when not running")
	}
}

func TestTransportCloseNotConnected(t *testing.T) {
	tr := newTransport("claude", nil)
	err := tr.close()
	if err != nil {
		t.Fatalf("close on disconnected should succeed: %v", err)
	}
}

func TestTransportConnectAlreadyConnected(t *testing.T) {
	tr := newTransport("claude", nil)
	tr.connected = true
	err := tr.connect(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "transport already connected" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTransportPrepareMcpConfigNil(t *testing.T) {
	tr := newTransport("claude", nil)
	opts, err := tr.prepareMcpConfig()
	if err != nil {
		t.Fatal(err)
	}
	if opts != nil {
		t.Fatal("expected nil options")
	}
}

func TestTransportPrepareMcpConfigEmpty(t *testing.T) {
	o := NewOptions()
	tr := newTransport("claude", o)
	opts, err := tr.prepareMcpConfig()
	if err != nil {
		t.Fatal(err)
	}
	if opts != o {
		t.Fatal("expected same options back")
	}
}

func TestTransportPrepareMcpConfigWithServers(t *testing.T) {
	o := NewOptions(WithMcpServers(map[string]McpServerConfig{
		"test-server": &McpStdioServerConfig{
			Type:    McpServerTypeStdio,
			Command: "node",
			Args:    []string{"server.js"},
		},
	}))
	tr := newTransport("claude", o)
	opts, err := tr.prepareMcpConfig()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if tr.mcpConfigFile != nil {
			os.Remove(tr.mcpConfigFile.Name())
			tr.mcpConfigFile.Close()
		}
	}()

	// Should have created a temp file
	if tr.mcpConfigFile == nil {
		t.Fatal("expected MCP config file")
	}

	// Verify the config file content
	data, err := os.ReadFile(tr.mcpConfigFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("invalid JSON in config file: %v", err)
	}
	servers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("expected mcpServers in config")
	}
	if _, ok := servers["test-server"]; !ok {
		t.Fatal("expected test-server in mcpServers")
	}

	// opts should have mcp-config in ExtraArgs
	if opts.ExtraArgs == nil {
		t.Fatal("expected ExtraArgs")
	}
	mcpPath, ok := opts.ExtraArgs["mcp-config"]
	if !ok || mcpPath == nil {
		t.Fatal("expected mcp-config in ExtraArgs")
	}
	if *mcpPath != tr.mcpConfigFile.Name() {
		t.Fatal("mcp-config path mismatch")
	}

	// Original options should not be modified
	if _, ok := o.ExtraArgs["mcp-config"]; ok {
		t.Fatal("original options should not be modified")
	}
}

func TestTransportPrepareMcpConfigPreservesExtraArgs(t *testing.T) {
	existingVal := "value1"
	o := NewOptions(WithMcpServers(map[string]McpServerConfig{
		"srv": &McpStdioServerConfig{Type: McpServerTypeStdio, Command: "cmd"},
	}))
	o.ExtraArgs["existing-flag"] = &existingVal

	tr := newTransport("claude", o)
	opts, err := tr.prepareMcpConfig()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if tr.mcpConfigFile != nil {
			os.Remove(tr.mcpConfigFile.Name())
			tr.mcpConfigFile.Close()
		}
	}()

	// Both old and new args should be present
	if opts.ExtraArgs["existing-flag"] == nil || *opts.ExtraArgs["existing-flag"] != "value1" {
		t.Fatal("lost existing extra arg")
	}
	if opts.ExtraArgs["mcp-config"] == nil {
		t.Fatal("missing mcp-config")
	}
}

func TestTransportCleanup(t *testing.T) {
	tr := newTransport("claude", nil)

	// Create temp files to simulate state
	stderr, err := os.CreateTemp("", "test_stderr_*")
	if err != nil {
		t.Fatal(err)
	}
	stderrName := stderr.Name()

	mcpFile, err := os.CreateTemp("", "test_mcp_*")
	if err != nil {
		t.Fatal(err)
	}
	mcpName := mcpFile.Name()

	tr.stderr = stderr
	tr.mcpConfigFile = mcpFile

	tr.cleanup()

	// Files should be removed
	if _, err := os.Stat(stderrName); !os.IsNotExist(err) {
		t.Fatal("stderr file should be removed")
	}
	if _, err := os.Stat(mcpName); !os.IsNotExist(err) {
		t.Fatal("mcp config file should be removed")
	}
	if tr.stderr != nil || tr.mcpConfigFile != nil || tr.cmd != nil {
		t.Fatal("fields should be nil after cleanup")
	}
}

func TestIsProcessFinished(t *testing.T) {
	tests := []struct {
		err  string
		want bool
	}{
		{"process already finished", true},
		{"process already released", true},
		{"no child processes", true},
		{"signal: killed", true},
		{"some other error", false},
		{"", false},
	}
	for _, tt := range tests {
		var err error
		if tt.err != "" {
			err = &testError{tt.err}
		}
		if got := isProcessFinished(err); got != tt.want {
			t.Errorf("isProcessFinished(%q) = %v, want %v", tt.err, got, tt.want)
		}
	}

	// nil error
	if isProcessFinished(nil) {
		t.Fatal("nil should return false")
	}
}

func TestTransportSendMessageCancelledContext(t *testing.T) {
	tr := newTransport("claude", nil)
	tr.connected = true
	// stdin is nil, but context check comes first if connected
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	msg := StreamMessage{Type: "user"}
	err := tr.sendMessage(ctx, msg)
	if err == nil {
		t.Fatal("expected error with cancelled context")
	}
}

func TestStreamMessageJSON(t *testing.T) {
	msg := StreamMessage{
		Type: "user",
		Message: map[string]interface{}{
			"role":    "user",
			"content": "hello",
		},
		SessionID: "sess-1",
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["type"] != "user" {
		t.Fatal("unexpected type")
	}
	if parsed["session_id"] != "sess-1" {
		t.Fatal("unexpected session_id")
	}
}

func TestStreamMessageJSONOmitsEmpty(t *testing.T) {
	msg := StreamMessage{Type: "user"}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Contains(s, "session_id") {
		t.Fatal("empty session_id should be omitted")
	}
	if strings.Contains(s, "parent_tool_use_id") {
		t.Fatal("nil parent_tool_use_id should be omitted")
	}
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
