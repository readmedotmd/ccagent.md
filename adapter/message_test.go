package ai_adapters

import (
	"testing"
	"time"
)

func TestTextContent(t *testing.T) {
	blocks := TextContent("hello world")
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Type != ContentText {
		t.Fatalf("expected text type, got %s", blocks[0].Type)
	}
	if blocks[0].Text != "hello world" {
		t.Fatalf("expected 'hello world', got %q", blocks[0].Text)
	}
}

func TestTextContentEmpty(t *testing.T) {
	blocks := TextContent("")
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Text != "" {
		t.Fatalf("expected empty text, got %q", blocks[0].Text)
	}
}

func TestRoleConstants(t *testing.T) {
	tests := []struct {
		role Role
		want string
	}{
		{RoleUser, "user"},
		{RoleAssistant, "assistant"},
		{RoleSystem, "system"},
		{RoleTool, "tool"},
	}
	for _, tt := range tests {
		if string(tt.role) != tt.want {
			t.Errorf("expected %q, got %q", tt.want, tt.role)
		}
	}
}

func TestContentTypeConstants(t *testing.T) {
	tests := []struct {
		ct   ContentType
		want string
	}{
		{ContentText, "text"},
		{ContentCode, "code"},
		{ContentImage, "image"},
		{ContentFile, "file"},
		{ContentToolUse, "tool_use"},
		{ContentToolResult, "tool_result"},
	}
	for _, tt := range tests {
		if string(tt.ct) != tt.want {
			t.Errorf("expected %q, got %q", tt.want, tt.ct)
		}
	}
}

func TestMessageFields(t *testing.T) {
	now := time.Now()
	msg := Message{
		ID:        "msg-123",
		Role:      RoleUser,
		Content:   TextContent("test"),
		Timestamp: now,
		Metadata:  map[string]string{"key": "value"},
	}
	if msg.ID != "msg-123" {
		t.Fatalf("unexpected ID: %s", msg.ID)
	}
	if msg.Role != RoleUser {
		t.Fatalf("unexpected role: %s", msg.Role)
	}
	if len(msg.Content) != 1 || msg.Content[0].Text != "test" {
		t.Fatalf("unexpected content: %v", msg.Content)
	}
	if msg.Metadata["key"] != "value" {
		t.Fatal("unexpected metadata")
	}
}

func TestContentBlockMultipart(t *testing.T) {
	blocks := []ContentBlock{
		{Type: ContentText, Text: "Look at this image:"},
		{Type: ContentImage, Data: []byte{0x89, 0x50, 0x4E, 0x47}, MimeType: "image/png"},
		{Type: ContentCode, Text: "fmt.Println(\"hello\")", Language: "go"},
		{Type: ContentFile, Data: []byte("file data"), MimeType: "text/plain"},
		{Type: ContentToolUse, ToolCall: &ToolCall{ID: "tc-1", Name: "read", Input: "file.go"}},
		{Type: ContentToolResult, ToolCall: &ToolCall{ID: "tc-1", Output: "contents", Status: "complete"}},
	}
	if len(blocks) != 6 {
		t.Fatalf("expected 6 blocks, got %d", len(blocks))
	}
	if blocks[1].MimeType != "image/png" {
		t.Fatal("unexpected mime type")
	}
	if blocks[2].Language != "go" {
		t.Fatal("unexpected language")
	}
	if blocks[4].ToolCall.Name != "read" {
		t.Fatal("unexpected tool name")
	}
	if blocks[5].ToolCall.Status != "complete" {
		t.Fatal("unexpected tool status")
	}
}

func TestToolCallFields(t *testing.T) {
	tc := ToolCall{
		ID:     "call-1",
		Name:   "bash",
		Input:  map[string]string{"command": "ls"},
		Output: "file1\nfile2",
		Status: "complete",
	}
	if tc.ID != "call-1" || tc.Name != "bash" || tc.Status != "complete" {
		t.Fatal("unexpected tool call fields")
	}
}

func TestConversationFields(t *testing.T) {
	now := time.Now()
	conv := Conversation{
		ID:        "conv-1",
		Adapter:   "claude",
		Title:     "Test Conversation",
		CreatedAt: now,
		UpdatedAt: now,
		Messages: []Message{
			{ID: "1", Role: RoleUser, Content: TextContent("hi")},
			{ID: "2", Role: RoleAssistant, Content: TextContent("hello")},
		},
		Metadata: map[string]string{"source": "test"},
	}
	if conv.ID != "conv-1" || conv.Adapter != "claude" {
		t.Fatal("unexpected conversation fields")
	}
	if len(conv.Messages) != 2 {
		t.Fatal("expected 2 messages")
	}
	if conv.Title != "Test Conversation" {
		t.Fatal("unexpected title")
	}
}
