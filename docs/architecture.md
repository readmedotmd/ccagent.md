# Architecture

## Overview

ccagent.md wraps the Claude Code CLI in a Go adapter that conforms to the [agent.adapter.md](https://github.com/readmedotmd/agent.adapter.md) interface. All communication happens over stdin/stdout using newline-delimited JSON.

```
┌─────────────────────────────────────────────────┐
│                  Your Application                │
│                                                  │
│   adapter.Start()  adapter.Send()  adapter.Receive()
└──────────┬──────────────┬──────────────┬─────────┘
           │              │              │
    ┌──────▼──────────────▼──────────────▼─────────┐
    │              ClaudeAdapter                    │
    │   queue ─── runLoop ─── event emission        │
    │   session tracking ─── context compaction     │
    └──────────────────┬───────────────────────────┘
                       │
    ┌──────────────────▼───────────────────────────┐
    │           internal/claudecode                 │
    │   Client ─── Transport ─── Parser             │
    │   stdin/stdout JSON streaming                 │
    └──────────────────┬───────────────────────────┘
                       │
    ┌──────────────────▼───────────────────────────┐
    │            Claude Code CLI                    │
    │   claude --output-format stream-json          │
    └──────────────────────────────────────────────┘
```

## Package Structure

```
ccagent.md/
├── claude.go                    # Main adapter — implements ai.Adapter
├── adapter/
│   ├── adapter.go               # Interface definitions, config, errors
│   ├── message.go               # Message, ContentBlock, Role types
│   └── stream.go                # StreamEvent, TokenUsage, FileChange
└── internal/claudecode/
    ├── client.go                # High-level Client interface
    ├── transport.go             # Subprocess management (stdin/stdout/stderr)
    ├── parser.go                # Speculative JSON parser
    ├── cli.go                   # CLI discovery, command building
    ├── options.go               # Functional options (WithModel, WithCwd, etc.)
    ├── types.go                 # Internal message types
    └── errors.go                # Error types
```

## Layer Details

### ClaudeAdapter (`claude.go`)

The public API. Manages:

- **Lifecycle**: `Start` → `Send`/`Receive` → `Stop`
- **Message queue**: If `Send` is called during an active turn, the message is queued and auto-processed after the current turn
- **Run loop**: Processes messages sequentially, draining the queue between turns
- **Event emission**: Translates internal SDK messages into `StreamEvent`s on the `Receive()` channel
- **Context compaction**: When estimated token usage hits 80% of the context window, summarizes the conversation and starts a fresh session
- **Session tracking**: Captures and exposes the CLI's session ID for resume support

### Internal Client (`internal/claudecode/client.go`)

Thin wrapper over the transport layer:

- `Connect(ctx)` — launches the CLI subprocess
- `Query(ctx, prompt)` / `QueryWithSession(ctx, prompt, sessionID)` — sends text
- `QueryStream(ctx, messages)` — sends multimodal or structured messages
- `ReceiveMessages(ctx)` — returns the transport's message channel
- `Disconnect()` — terminates the subprocess

### Transport (`internal/claudecode/transport.go`)

Subprocess lifecycle management:

- **Process launch**: `exec.CommandContext` with stdin/stdout pipes
- **Message send**: JSON-encode to stdin, one message per line
- **Message receive**: Scanner reads stdout line-by-line, feeds parser
- **Termination**: SIGTERM → 5s grace period → SIGKILL
- **MCP config**: Writes server config to a temp file, passes `--mcp-config`
- **Cleanup**: Closes pipes, removes temp files (stderr log, MCP config)

### Parser (`internal/claudecode/parser.go`)

Speculative JSON parser that handles the CLI's streaming output:

- Accumulates incomplete JSON fragments across lines
- Attempts `json.Unmarshal` after each line — if it fails, keeps buffering
- On success, resets buffer and dispatches by message `type` field
- Discriminates: `assistant`, `result`, `system`, `user`, `control_request`, `control_response`, `stream_event`
- Parses content blocks: `text`, `thinking`, `tool_use`, `tool_result`
- Buffer overflow protection at 1MB

### CLI Discovery (`internal/claudecode/cli.go`)

Finds the Claude Code CLI:

1. `exec.LookPath("claude")` — checks PATH
2. Platform-specific locations (npm global, homebrew, yarn, etc.)
3. Falls back with a helpful error message

Builds the CLI command with flags:
```
claude --output-format stream-json --verbose --input-format stream-json \
    --model opus --system-prompt "..." --permission-mode bypassPermissions \
    --allowed-tools read,write --setting-sources ""
```

## Data Flow

### Send Path

```
adapter.Send(msg)
  → if running: queue
  → if idle: start runLoop goroutine
    → messageText(msg) + messageImages(msg)
    → client.Query(text) or client.QueryStream(multimodal)
      → transport.sendMessage(json)
        → stdin.Write(json + "\n")
```

### Receive Path

```
CLI stdout → scanner.Scan()
  → parser.processLine(line)
    → json.Unmarshal → parseMessage → parseContentBlock
  → transport.msgChan ← Message
    → client.ReceiveMessages() channel
      → adapter.runClaude() switch on message type
        → adapter.events ← StreamEvent
          → adapter.Receive() channel
            → your application
```

### Context Compaction

```
estimatedTokens > 80% of contextWindow?
  → build transcript from history
  → one-shot query: "summarize this conversation"
  → collect summary from response
  → reset sessionID (force new session)
  → send summary as first message of new session
  → capture new sessionID from result
  → reset token estimate
```
