package ai_adapters

import (
	"encoding/json"
	"testing"
)

func TestCreateTextResult(t *testing.T) {
	r := CreateTextResult("hello world")
	if r.IsError {
		t.Fatal("expected non-error result")
	}
	if r.Output != "hello world" {
		t.Fatalf("expected 'hello world', got %q", r.Output)
	}
	if r.Message != "Success" {
		t.Fatalf("expected 'Success', got %q", r.Message)
	}
}

func TestCreateTextResultEmpty(t *testing.T) {
	r := CreateTextResult("")
	if r.IsError {
		t.Fatal("expected non-error result")
	}
	if r.Output != "" {
		t.Fatalf("expected empty output, got %q", r.Output)
	}
}

func TestCreateErrorResult(t *testing.T) {
	r := CreateErrorResult("file not found")
	if !r.IsError {
		t.Fatal("expected error result")
	}
	if r.Output != "file not found" {
		t.Fatalf("expected 'file not found', got %q", r.Output)
	}
	if r.Message != "Error" {
		t.Fatalf("expected 'Error', got %q", r.Message)
	}
}

func TestCreateErrorResultEmpty(t *testing.T) {
	r := CreateErrorResult("")
	if !r.IsError {
		t.Fatal("expected error result")
	}
}

func TestToolResultJSON(t *testing.T) {
	r := CreateTextResult("output")
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ToolResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Output != "output" || decoded.IsError || decoded.Message != "Success" {
		t.Fatal("round-trip mismatch")
	}
}

func TestExternalToolJSON(t *testing.T) {
	tool := ExternalTool{
		Name:        "search",
		Description: "Search for files",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}}}`),
	}
	data, err := json.Marshal(tool)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ExternalTool
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Name != "search" {
		t.Fatalf("expected 'search', got %q", decoded.Name)
	}
	if decoded.Description != "Search for files" {
		t.Fatalf("expected description, got %q", decoded.Description)
	}
	if string(decoded.Parameters) != string(tool.Parameters) {
		t.Fatal("parameters mismatch")
	}
}

func TestToolCallRequestJSON(t *testing.T) {
	req := ToolCallRequest{
		ID:        "call-1",
		Name:      "bash",
		Arguments: json.RawMessage(`{"command":"ls -la"}`),
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ToolCallRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ID != "call-1" || decoded.Name != "bash" {
		t.Fatal("round-trip mismatch")
	}
}

func TestToolCallResponseJSON(t *testing.T) {
	resp := ToolCallResponse{
		ToolCallID: "call-1",
		Result:     CreateTextResult("done"),
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ToolCallResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ToolCallID != "call-1" || decoded.Result.Output != "done" {
		t.Fatal("round-trip mismatch")
	}
}
