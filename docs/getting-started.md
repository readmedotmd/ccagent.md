# Getting Started

## Installation

```bash
go get github.com/readmedotmd/ccagent.md
```

No external dependencies — only the Go standard library.

## Prerequisites

1. **Go 1.23.6+**
2. **Claude Code CLI** installed and on your PATH:
   ```bash
   npm install -g @anthropic-ai/claude-code
   ```
3. A valid **Anthropic API key** configured for the CLI

## The Adapter Interface

The adapter implements the `ai_adapters.Adapter` interface from [agent.adapter.md](https://github.com/readmedotmd/agent.adapter.md):

```go
type Adapter interface {
    Start(ctx context.Context, cfg AdapterConfig) error
    Send(ctx context.Context, msg Message, opts ...SendOption) error
    Cancel() error
    Receive() <-chan StreamEvent
    Stop() error
    Status() AdapterStatus
    Capabilities() AdapterCapabilities
    Health(ctx context.Context) error
}
```

## Basic Usage

```go
package main

import (
    "context"
    "fmt"

    claude "github.com/readmedotmd/ccagent.md"
    ai "github.com/readmedotmd/ccagent.md/adapter"
)

func main() {
    ctx := context.Background()
    adapter := claude.NewClaudeAdapter()

    // Start the adapter — this launches the Claude Code CLI subprocess
    err := adapter.Start(ctx, ai.AdapterConfig{
        Name:           "my-agent",
        WorkDir:        ".",
        PermissionMode: ai.PermissionAcceptAll,
    })
    if err != nil {
        panic(err)
    }
    defer adapter.Stop()

    // Build a message
    msg := ai.Message{
        Role:    ai.RoleUser,
        Content: ai.TextContent("Explain the main.go file"),
    }

    // Send it
    if err := adapter.Send(ctx, msg); err != nil {
        panic(err)
    }

    // Consume the streaming response
    for ev := range adapter.Receive() {
        switch ev.Type {
        case ai.EventToken:
            fmt.Print(ev.Token)
        case ai.EventDone:
            fmt.Println()
            return
        case ai.EventError:
            fmt.Printf("error: %v\n", ev.Error)
            return
        }
    }
}
```

## Multipart Messages

Messages support multiple content blocks — text, images, code, files, and tool results:

```go
msg := ai.Message{
    Role: ai.RoleUser,
    Content: []ai.ContentBlock{
        {Type: ai.ContentText, Text: "What's in this screenshot?"},
        {Type: ai.ContentImage, Data: pngBytes, MimeType: "image/png"},
    },
}
adapter.Send(ctx, msg)
```

The `ai.TextContent()` helper creates a single-text-block slice:

```go
// These are equivalent:
ai.TextContent("hello")
[]ai.ContentBlock{{Type: ai.ContentText, Text: "hello"}}
```

## Send Options

Per-turn behavior can be controlled with functional options:

```go
adapter.Send(ctx, msg,
    ai.WithMaxTokens(4096),
    ai.WithTemperature(0.7),
    ai.WithStopSequences([]string{"END"}),
    ai.WithTools([]string{"Read", "Bash"}),
)
```

## Message Queuing

If you call `Send` while a previous turn is still running, the message is queued and processed automatically after the current turn completes. Multiple queued messages are combined into a single turn.

```go
adapter.Send(ctx, ai.Message{Content: ai.TextContent("first")})
adapter.Send(ctx, ai.Message{Content: ai.TextContent("second")})  // queued
adapter.Send(ctx, ai.Message{Content: ai.TextContent("third")})   // queued
// "second" and "third" are combined and sent as one turn after "first" completes
```

## Checking Status

```go
status := adapter.Status()
// ai.StatusIdle    — not started
// ai.StatusRunning — active
// ai.StatusStopped — stopped
// ai.StatusError   — error state
```

## Health Checks

```go
if err := adapter.Health(ctx); err != nil {
    ae := err.(*ai.AdapterError)
    fmt.Printf("unhealthy: code=%d msg=%s\n", ae.Code, ae.Message)
}
```

## Session Resume

The adapter implements `SessionProvider` to expose the current session ID:

```go
sp := adapter.(ai.SessionProvider)
sessionID := sp.SessionID()

// Later, resume:
adapter.Start(ctx, ai.AdapterConfig{
    SessionID: sessionID,
})
```

## Next Steps

- [Configuration reference](./configuration.md) — all config fields explained
- [Events reference](./events.md) — every stream event type
- [Architecture](./architecture.md) — how the internals work
- [Testing](./testing.md) — running and understanding the test suite
