# Stream Events Reference

All responses are delivered through `adapter.Receive()` as a `<-chan StreamEvent`. Each event has a `Type`, `Timestamp`, and type-specific fields.

## Event Types

### EventToken

A text token from the assistant's response. Concatenate all tokens for the full response.

```go
case ai.EventToken:
    fmt.Print(ev.Token) // ev.Token is a string fragment
```

### EventThinking

Extended thinking content — the model's internal reasoning.

```go
case ai.EventThinking:
    fmt.Printf("[thinking] %s\n", ev.Thinking)
```

### EventToolUse

The model is invoking a tool. Includes the tool call ID for correlation with the result.

```go
case ai.EventToolUse:
    fmt.Printf("tool: %s (id: %s)\n", ev.ToolName, ev.ToolCallID)
    fmt.Printf("input: %v\n", ev.ToolInput)
    // ev.ToolStatus is "running"
```

### EventToolResult

A tool has completed. Correlated with `EventToolUse` via `ToolCallID`.

```go
case ai.EventToolResult:
    fmt.Printf("tool %s result: %v\n", ev.ToolCallID, ev.ToolOutput)
    // ev.ToolStatus is "complete" or "failed"
```

### EventPermissionRequest

The agent needs user approval before running a tool.

```go
case ai.EventPermissionRequest:
    perm := ev.Permission
    fmt.Printf("approve %s? (id: %s)\n", perm.ToolName, perm.ToolCallID)
    fmt.Printf("description: %s\n", perm.Description)
    fmt.Printf("input: %v\n", perm.ToolInput)
```

### EventPermissionResult

Records the outcome of a permission decision (useful for logging and replay).

### EventFileChange

The agent created, edited, deleted, or renamed a file.

```go
case ai.EventFileChange:
    fc := ev.FileChange
    fmt.Printf("[%s] %s\n", fc.Op, fc.Path)
    if fc.Op == ai.FileRenamed {
        fmt.Printf("  from: %s\n", fc.OldPath)
    }
```

File change operations: `FileCreated`, `FileEdited`, `FileDeleted`, `FileRenamed`.

### EventSubAgent

A sub-agent was started, completed, or failed.

```go
case ai.EventSubAgent:
    sa := ev.SubAgent
    fmt.Printf("agent %s (%s): %s\n", sa.AgentName, sa.AgentID, sa.Status)
    if sa.Status == "completed" {
        fmt.Printf("result: %s\n", sa.Result)
    }
```

Status values: `"started"`, `"completed"`, `"failed"`.

### EventProgress

Progress update for a long-running operation.

```go
case ai.EventProgress:
    if ev.ProgressPct >= 0 {
        fmt.Printf("progress: %.0f%% — %s\n", ev.ProgressPct*100, ev.ProgressMsg)
    } else {
        fmt.Printf("working: %s\n", ev.ProgressMsg) // indeterminate
    }
```

`ProgressPct` is 0.0–1.0, or -1 for indeterminate.

### EventCostUpdate

Token usage and cost estimate for the turn.

```go
case ai.EventCostUpdate:
    u := ev.Usage
    fmt.Printf("tokens: %d in / %d out, cost: $%.4f\n",
        u.InputTokens, u.OutputTokens, u.TotalCost)
    fmt.Printf("cache: %d read / %d write\n", u.CacheRead, u.CacheWrite)
```

### EventDone

The turn is complete. Contains the full assembled `Message`.

```go
case ai.EventDone:
    if ev.Message != nil {
        for _, block := range ev.Message.Content {
            if block.Type == ai.ContentText {
                fmt.Println(block.Text)
            }
        }
    }
```

### EventError

An error occurred during the turn.

```go
case ai.EventError:
    fmt.Printf("error: %v\n", ev.Error)
```

## StreamEvent Struct

```go
type StreamEvent struct {
    Type      StreamEventType
    Timestamp time.Time

    // Content
    Token    string
    Thinking string

    // Tool use
    ToolCallID string
    ToolName   string
    ToolInput  any
    ToolOutput any
    ToolStatus string // "running", "complete", "failed"

    // Permission flow
    Permission *PermissionRequest

    // File operations
    FileChange *FileChange

    // Sub-agent delegation
    SubAgent *SubAgentEvent

    // Progress
    ProgressPct float64 // 0–1, -1 if indeterminate
    ProgressMsg string

    // Cost / usage
    Usage *TokenUsage

    // Control flow
    Error   error
    Message *Message
}
```

## Typical Event Sequence

A simple text response:
```
EventToken → EventToken → ... → EventDone
```

A response with tool use:
```
EventThinking → EventToolUse → EventToolResult → EventToken → ... → EventCostUpdate → EventDone
```

A response with permission prompt:
```
EventToolUse → EventPermissionRequest → (wait for approval) → EventToolResult → EventDone
```
