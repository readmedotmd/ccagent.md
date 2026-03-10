package claudecode

import (
	"strings"
	"testing"
)

func TestParserAssistantMessage(t *testing.T) {
	line := `{"type":"assistant","message":{"role":"assistant","model":"claude-sonnet-4-20250514","content":[{"type":"text","text":"Hello world"}]}}`
	p := newParser()
	msgs, err := p.processLine(line)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	am, ok := msgs[0].(*AssistantMessage)
	if !ok {
		t.Fatalf("expected AssistantMessage, got %T", msgs[0])
	}
	if am.Model != "claude-sonnet-4-20250514" {
		t.Fatalf("unexpected model: %s", am.Model)
	}
	if len(am.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(am.Content))
	}
	tb, ok := am.Content[0].(*TextBlock)
	if !ok {
		t.Fatalf("expected TextBlock, got %T", am.Content[0])
	}
	if tb.Text != "Hello world" {
		t.Fatalf("unexpected text: %q", tb.Text)
	}
}

func TestParserResultMessage(t *testing.T) {
	line := `{"type":"result","subtype":"success","duration_ms":1500,"duration_api_ms":1200,"is_error":false,"num_turns":3,"session_id":"sess-abc","total_cost_usd":0.05,"result":"Done"}`
	p := newParser()
	msgs, err := p.processLine(line)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	rm, ok := msgs[0].(*ResultMessage)
	if !ok {
		t.Fatalf("expected ResultMessage, got %T", msgs[0])
	}
	if rm.Subtype != "success" {
		t.Fatalf("unexpected subtype: %s", rm.Subtype)
	}
	if rm.DurationMs != 1500 {
		t.Fatalf("unexpected duration: %d", rm.DurationMs)
	}
	if rm.DurationAPIMs != 1200 {
		t.Fatalf("unexpected api duration: %d", rm.DurationAPIMs)
	}
	if rm.IsError {
		t.Fatal("expected IsError false")
	}
	if rm.NumTurns != 3 {
		t.Fatalf("unexpected num turns: %d", rm.NumTurns)
	}
	if rm.SessionID != "sess-abc" {
		t.Fatalf("unexpected session: %s", rm.SessionID)
	}
	if rm.TotalCostUSD == nil || *rm.TotalCostUSD != 0.05 {
		t.Fatal("unexpected cost")
	}
	if rm.Result == nil || *rm.Result != "Done" {
		t.Fatal("unexpected result")
	}
}

func TestParserResultMessageError(t *testing.T) {
	line := `{"type":"result","is_error":true,"result":"Something went wrong"}`
	p := newParser()
	msgs, err := p.processLine(line)
	if err != nil {
		t.Fatal(err)
	}
	rm := msgs[0].(*ResultMessage)
	if !rm.IsError {
		t.Fatal("expected IsError true")
	}
	if rm.Result == nil || *rm.Result != "Something went wrong" {
		t.Fatal("unexpected result")
	}
}

func TestParserResultMessageNoCost(t *testing.T) {
	line := `{"type":"result","session_id":"s1"}`
	p := newParser()
	msgs, _ := p.processLine(line)
	rm := msgs[0].(*ResultMessage)
	if rm.TotalCostUSD != nil {
		t.Fatal("expected nil cost")
	}
	if rm.Result != nil {
		t.Fatal("expected nil result")
	}
}

func TestParserSystemMessage(t *testing.T) {
	line := `{"type":"system","subtype":"init","data":{"version":"1.0"}}`
	p := newParser()
	msgs, err := p.processLine(line)
	if err != nil {
		t.Fatal(err)
	}
	sm, ok := msgs[0].(*SystemMessage)
	if !ok {
		t.Fatalf("expected SystemMessage, got %T", msgs[0])
	}
	if sm.Subtype != "init" {
		t.Fatalf("unexpected subtype: %s", sm.Subtype)
	}
}

func TestParserUserMessage(t *testing.T) {
	line := `{"type":"user","message":{"role":"user","content":"Hello"}}`
	p := newParser()
	msgs, err := p.processLine(line)
	if err != nil {
		t.Fatal(err)
	}
	um, ok := msgs[0].(*UserMessage)
	if !ok {
		t.Fatalf("expected UserMessage, got %T", msgs[0])
	}
	if um.Content != "Hello" {
		t.Fatalf("unexpected content: %v", um.Content)
	}
}

func TestParserControlMessages(t *testing.T) {
	for _, msgType := range []string{"control_request", "control_response"} {
		line := `{"type":"` + msgType + `","data":{"key":"val"}}`
		p := newParser()
		msgs, err := p.processLine(line)
		if err != nil {
			t.Fatal(err)
		}
		rcm, ok := msgs[0].(*RawControlMessage)
		if !ok {
			t.Fatalf("expected RawControlMessage, got %T", msgs[0])
		}
		if rcm.MessageType != msgType {
			t.Fatalf("unexpected message type: %s", rcm.MessageType)
		}
	}
}

func TestParserStreamEventSkipped(t *testing.T) {
	line := `{"type":"stream_event","data":{"something":"value"}}`
	p := newParser()
	msgs, err := p.processLine(line)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages for stream_event, got %d", len(msgs))
	}
}

func TestParserUnknownType(t *testing.T) {
	line := `{"type":"unknown_type","data":{}}`
	p := newParser()
	_, err := p.processLine(line)
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
	if !strings.Contains(err.Error(), "unknown message type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParserEmptyLine(t *testing.T) {
	p := newParser()
	msgs, err := p.processLine("")
	if err != nil {
		t.Fatal(err)
	}
	if msgs != nil {
		t.Fatal("expected nil for empty line")
	}

	msgs, err = p.processLine("   ")
	if err != nil {
		t.Fatal(err)
	}
	if msgs != nil {
		t.Fatal("expected nil for whitespace line")
	}
}

func TestParserIncompleteJSON(t *testing.T) {
	p := newParser()

	// First part — incomplete
	msgs, err := p.processLine(`{"type":"result"`)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Fatal("expected no messages for incomplete JSON")
	}

	// Second part — completes the JSON
	msgs, err = p.processLine(`,"session_id":"s1"}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message after completing JSON, got %d", len(msgs))
	}
	rm, ok := msgs[0].(*ResultMessage)
	if !ok {
		t.Fatalf("expected ResultMessage, got %T", msgs[0])
	}
	if rm.SessionID != "s1" {
		t.Fatalf("unexpected session: %s", rm.SessionID)
	}
}

func TestParserBufferOverflow(t *testing.T) {
	p := newParser()
	// Exceed max buffer size
	bigLine := strings.Repeat("x", maxBufferSize+1)
	_, err := p.processLine(bigLine)
	if err == nil {
		t.Fatal("expected buffer overflow error")
	}
	if !strings.Contains(err.Error(), "buffer overflow") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParserMultipleMessagesInLine(t *testing.T) {
	// processLine splits on newlines within a single line
	p := newParser()
	msgs, err := p.processLine(`{"type":"result","session_id":"s1"}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
}

func TestParserToolUseBlock(t *testing.T) {
	line := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu-1","name":"bash","input":{"command":"ls -la"}}]}}`
	p := newParser()
	msgs, err := p.processLine(line)
	if err != nil {
		t.Fatal(err)
	}
	am := msgs[0].(*AssistantMessage)
	if len(am.Content) != 1 {
		t.Fatalf("expected 1 block, got %d", len(am.Content))
	}
	tub, ok := am.Content[0].(*ToolUseBlock)
	if !ok {
		t.Fatalf("expected ToolUseBlock, got %T", am.Content[0])
	}
	if tub.ToolUseID != "tu-1" {
		t.Fatalf("unexpected tool use ID: %s", tub.ToolUseID)
	}
	if tub.Name != "bash" {
		t.Fatalf("unexpected tool name: %s", tub.Name)
	}
	if tub.Input["command"] != "ls -la" {
		t.Fatalf("unexpected input: %v", tub.Input)
	}
}

func TestParserToolUseBlockNoInput(t *testing.T) {
	line := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tu-2","name":"cancel"}]}}`
	p := newParser()
	msgs, err := p.processLine(line)
	if err != nil {
		t.Fatal(err)
	}
	tub := msgs[0].(*AssistantMessage).Content[0].(*ToolUseBlock)
	if tub.Input == nil {
		t.Fatal("expected non-nil input map")
	}
	if len(tub.Input) != 0 {
		t.Fatal("expected empty input map")
	}
}

func TestParserThinkingBlock(t *testing.T) {
	line := `{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"Let me consider...","signature":"sig123"}]}}`
	p := newParser()
	msgs, err := p.processLine(line)
	if err != nil {
		t.Fatal(err)
	}
	am := msgs[0].(*AssistantMessage)
	tb, ok := am.Content[0].(*ThinkingBlock)
	if !ok {
		t.Fatalf("expected ThinkingBlock, got %T", am.Content[0])
	}
	if tb.Thinking != "Let me consider..." {
		t.Fatalf("unexpected thinking: %s", tb.Thinking)
	}
	if tb.Signature != "sig123" {
		t.Fatalf("unexpected signature: %s", tb.Signature)
	}
}

func TestParserToolResultBlock(t *testing.T) {
	line := `{"type":"assistant","message":{"content":[{"type":"tool_result","tool_use_id":"tu-1","content":"output data"}]}}`
	p := newParser()
	msgs, err := p.processLine(line)
	if err != nil {
		t.Fatal(err)
	}
	am := msgs[0].(*AssistantMessage)
	trb, ok := am.Content[0].(*ToolResultBlock)
	if !ok {
		t.Fatalf("expected ToolResultBlock, got %T", am.Content[0])
	}
	if trb.ToolUseID != "tu-1" {
		t.Fatalf("unexpected tool use ID: %s", trb.ToolUseID)
	}
	if trb.Content != "output data" {
		t.Fatalf("unexpected content: %v", trb.Content)
	}
}

func TestParserUnknownBlockTypeSkipped(t *testing.T) {
	line := `{"type":"assistant","message":{"content":[{"type":"future_block","data":"val"},{"type":"text","text":"hello"}]}}`
	p := newParser()
	msgs, err := p.processLine(line)
	if err != nil {
		t.Fatal(err)
	}
	am := msgs[0].(*AssistantMessage)
	if len(am.Content) != 1 {
		t.Fatalf("expected 1 block (unknown skipped), got %d", len(am.Content))
	}
	if am.Content[0].(*TextBlock).Text != "hello" {
		t.Fatal("unexpected text")
	}
}

func TestParserMixedContentBlocks(t *testing.T) {
	line := `{"type":"assistant","message":{"model":"opus","content":[{"type":"thinking","thinking":"hmm"},{"type":"text","text":"Here is the result"},{"type":"tool_use","id":"tu-5","name":"read","input":{"path":"main.go"}}]}}`
	p := newParser()
	msgs, err := p.processLine(line)
	if err != nil {
		t.Fatal(err)
	}
	am := msgs[0].(*AssistantMessage)
	if am.Model != "opus" {
		t.Fatalf("unexpected model: %s", am.Model)
	}
	if len(am.Content) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(am.Content))
	}
	if am.Content[0].BlockType() != ContentBlockTypeThinking {
		t.Fatal("expected thinking block first")
	}
	if am.Content[1].BlockType() != ContentBlockTypeText {
		t.Fatal("expected text block second")
	}
	if am.Content[2].BlockType() != ContentBlockTypeToolUse {
		t.Fatal("expected tool_use block third")
	}
}

func TestParserInvalidContentBlockType(t *testing.T) {
	line := `{"type":"assistant","message":{"content":"not an array"}}`
	p := newParser()
	_, err := p.processLine(line)
	if err == nil {
		t.Fatal("expected error for non-array content")
	}
}

func TestParserInvalidContentBlockObject(t *testing.T) {
	line := `{"type":"assistant","message":{"content":["not an object"]}}`
	p := newParser()
	_, err := p.processLine(line)
	if err == nil {
		t.Fatal("expected error for non-object content block")
	}
}

func TestParserMissingMessageField(t *testing.T) {
	line := `{"type":"assistant"}`
	p := newParser()
	_, err := p.processLine(line)
	if err == nil {
		t.Fatal("expected error for missing message field")
	}
}

func TestParserUserMessageMissingMessage(t *testing.T) {
	line := `{"type":"user"}`
	p := newParser()
	_, err := p.processLine(line)
	if err == nil {
		t.Fatal("expected error for missing message in user message")
	}
}

func TestParserBufferResetAfterSuccess(t *testing.T) {
	p := newParser()

	// Parse a valid message
	msgs, err := p.processLine(`{"type":"result","session_id":"s1"}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatal("expected 1 message")
	}

	// Parse another valid message — buffer should be clean
	msgs, err = p.processLine(`{"type":"result","session_id":"s2"}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatal("expected 1 message")
	}
	rm := msgs[0].(*ResultMessage)
	if rm.SessionID != "s2" {
		t.Fatalf("expected s2, got %s", rm.SessionID)
	}
}

func TestMessageTypeConstants(t *testing.T) {
	if MessageTypeUser != "user" || MessageTypeAssistant != "assistant" ||
		MessageTypeSystem != "system" || MessageTypeResult != "result" ||
		MessageTypeControlRequest != "control_request" ||
		MessageTypeControlResponse != "control_response" ||
		MessageTypeStreamEvent != "stream_event" {
		t.Fatal("unexpected message type constants")
	}
}

func TestBlockTypeConstants(t *testing.T) {
	if ContentBlockTypeText != "text" || ContentBlockTypeThinking != "thinking" ||
		ContentBlockTypeToolUse != "tool_use" || ContentBlockTypeToolResult != "tool_result" {
		t.Fatal("unexpected block type constants")
	}
}

func TestMessageInterfaceTypes(t *testing.T) {
	// Verify all message types implement Message interface
	var _ Message = (*AssistantMessage)(nil)
	var _ Message = (*ResultMessage)(nil)
	var _ Message = (*SystemMessage)(nil)
	var _ Message = (*UserMessage)(nil)
	var _ Message = (*RawControlMessage)(nil)

	if (&AssistantMessage{}).Type() != "assistant" {
		t.Fatal("unexpected type")
	}
	if (&ResultMessage{}).Type() != "result" {
		t.Fatal("unexpected type")
	}
	if (&SystemMessage{}).Type() != "system" {
		t.Fatal("unexpected type")
	}
	if (&UserMessage{}).Type() != "user" {
		t.Fatal("unexpected type")
	}
	if (&RawControlMessage{MessageType: "control_request"}).Type() != "control_request" {
		t.Fatal("unexpected type")
	}
}

func TestContentBlockInterfaceTypes(t *testing.T) {
	var _ ContentBlock = (*TextBlock)(nil)
	var _ ContentBlock = (*ThinkingBlock)(nil)
	var _ ContentBlock = (*ToolUseBlock)(nil)
	var _ ContentBlock = (*ToolResultBlock)(nil)

	if (&TextBlock{}).BlockType() != "text" {
		t.Fatal("unexpected block type")
	}
	if (&ThinkingBlock{}).BlockType() != "thinking" {
		t.Fatal("unexpected block type")
	}
	if (&ToolUseBlock{}).BlockType() != "tool_use" {
		t.Fatal("unexpected block type")
	}
	if (&ToolResultBlock{}).BlockType() != "tool_result" {
		t.Fatal("unexpected block type")
	}
}
