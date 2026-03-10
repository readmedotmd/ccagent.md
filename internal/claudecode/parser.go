package claudecode

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

const maxBufferSize = 1024 * 1024 // 1MB

// parser handles JSON message parsing with speculative parsing and buffer management.
type parser struct {
	buffer strings.Builder
	mu     sync.Mutex
}

func newParser() *parser {
	return &parser{}
}

// processLine processes a line of JSON input with speculative parsing.
func (p *parser) processLine(line string) ([]Message, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}

	var messages []Message
	for _, jsonLine := range strings.Split(line, "\n") {
		jsonLine = strings.TrimSpace(jsonLine)
		if jsonLine == "" {
			continue
		}
		msg, err := p.processJSONLine(jsonLine)
		if err != nil {
			return messages, err
		}
		if msg != nil {
			messages = append(messages, msg)
		}
	}
	return messages, nil
}

func (p *parser) processJSONLine(jsonLine string) (Message, error) {
	p.buffer.WriteString(jsonLine)

	if p.buffer.Len() > maxBufferSize {
		p.buffer.Reset()
		return nil, fmt.Errorf("buffer overflow")
	}

	var rawData map[string]any
	if err := json.Unmarshal([]byte(p.buffer.String()), &rawData); err != nil {
		// Incomplete JSON — continue accumulating.
		return nil, nil
	}

	p.buffer.Reset()
	return parseMessage(rawData)
}

func parseMessage(data map[string]any) (Message, error) {
	msgType, _ := data["type"].(string)

	switch msgType {
	case MessageTypeAssistant:
		return parseAssistantMessage(data)
	case MessageTypeResult:
		return parseResultMessage(data)
	case MessageTypeSystem:
		return parseSystemMessage(data)
	case MessageTypeUser:
		return parseUserMessage(data)
	case MessageTypeControlRequest, MessageTypeControlResponse:
		return &RawControlMessage{MessageType: msgType, Data: data}, nil
	case MessageTypeStreamEvent:
		// Pass through — not used by the adapter.
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown message type: %s", msgType)
	}
}

func parseAssistantMessage(data map[string]any) (*AssistantMessage, error) {
	messageData, ok := data["message"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("assistant message missing message field")
	}

	contentArray, ok := messageData["content"].([]any)
	if !ok {
		return nil, fmt.Errorf("assistant message content must be array")
	}

	model, _ := messageData["model"].(string)

	blocks := make([]ContentBlock, 0, len(contentArray))
	for _, blockData := range contentArray {
		block, err := parseContentBlock(blockData)
		if err != nil {
			return nil, err
		}
		if block != nil {
			blocks = append(blocks, block)
		}
	}

	return &AssistantMessage{Content: blocks, Model: model}, nil
}

func parseResultMessage(data map[string]any) (*ResultMessage, error) {
	result := &ResultMessage{}
	result.Subtype, _ = data["subtype"].(string)
	if dm, ok := data["duration_ms"].(float64); ok {
		result.DurationMs = int(dm)
	}
	if dam, ok := data["duration_api_ms"].(float64); ok {
		result.DurationAPIMs = int(dam)
	}
	result.IsError, _ = data["is_error"].(bool)
	if nt, ok := data["num_turns"].(float64); ok {
		result.NumTurns = int(nt)
	}
	result.SessionID, _ = data["session_id"].(string)

	if tc, ok := data["total_cost_usd"].(float64); ok {
		result.TotalCostUSD = &tc
	}
	if r, ok := data["result"].(string); ok {
		result.Result = &r
	}

	return result, nil
}

func parseSystemMessage(data map[string]any) (*SystemMessage, error) {
	subtype, _ := data["subtype"].(string)
	return &SystemMessage{Subtype: subtype, Data: data}, nil
}

func parseUserMessage(data map[string]any) (*UserMessage, error) {
	messageData, ok := data["message"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("user message missing message field")
	}
	return &UserMessage{Content: messageData["content"]}, nil
}

func parseContentBlock(blockData any) (ContentBlock, error) {
	data, ok := blockData.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("content block must be an object")
	}

	blockType, _ := data["type"].(string)
	switch blockType {
	case ContentBlockTypeText:
		text, _ := data["text"].(string)
		return &TextBlock{Text: text}, nil
	case ContentBlockTypeThinking:
		thinking, _ := data["thinking"].(string)
		signature, _ := data["signature"].(string)
		return &ThinkingBlock{Thinking: thinking, Signature: signature}, nil
	case ContentBlockTypeToolUse:
		id, _ := data["id"].(string)
		name, _ := data["name"].(string)
		input, _ := data["input"].(map[string]any)
		if input == nil {
			input = make(map[string]any)
		}
		return &ToolUseBlock{ToolUseID: id, Name: name, Input: input}, nil
	case ContentBlockTypeToolResult:
		toolUseID, _ := data["tool_use_id"].(string)
		return &ToolResultBlock{ToolUseID: toolUseID, Content: data["content"]}, nil
	default:
		// Unknown block types are silently skipped for forward compatibility.
		return nil, nil
	}
}
