# Configuration Reference

## AdapterConfig

All fields are optional unless noted.

| Field | Type | Description |
|---|---|---|
| `Name` | `string` | Identifier for this adapter instance |
| `Command` | `string` | CLI command override (unused — CLI is auto-detected) |
| `WorkDir` | `string` | Working directory for the CLI subprocess |
| `Args` | `[]string` | Extra CLI arguments |
| `Env` | `map[string]string` | Extra environment variables |
| `SystemPrompt` | `string` | Replace the default system prompt |
| `AppendSystemPrompt` | `string` | Append to the default system prompt |
| `Model` | `string` | Model to use (e.g. `claude-sonnet-4-20250514`, `claude-opus-4-20250514`) |
| `MaxThinkingTokens` | `int` | Max tokens for extended thinking (default: 8000) |
| `PermissionMode` | `PermissionMode` | Tool permission handling mode |
| `SessionID` | `string` | Resume a specific session by ID |
| `ContinueSession` | `bool` | Continue the most recent session |
| `MCPServers` | `map[string]MCPServerConfig` | MCP stdio servers to attach |
| `AllowedTools` | `[]string` | Whitelist of tool names |
| `DisallowedTools` | `[]string` | Blacklist of tool names |
| `Agents` | `map[string]AgentDef` | Sub-agent definitions |
| `ContextWindow` | `int` | Context window size in tokens (default: 200000) |

## Permission Modes

| Mode | Constant | Behavior |
|---|---|---|
| Default | `ai.PermissionDefault` | CLI default — prompt for each tool |
| Accept All | `ai.PermissionAcceptAll` | Bypass all permission prompts |
| Plan | `ai.PermissionPlan` | Plan mode — read-only tools auto-approved |

```go
adapter.Start(ctx, ai.AdapterConfig{
    PermissionMode: ai.PermissionAcceptAll,
})
```

## MCP Servers

Attach Model Context Protocol servers that the agent can use as tools:

```go
adapter.Start(ctx, ai.AdapterConfig{
    MCPServers: map[string]ai.MCPServerConfig{
        "filesystem": {
            Command: "npx",
            Args:    []string{"-y", "@anthropic-ai/mcp-filesystem", "/path/to/dir"},
        },
        "database": {
            Command: "node",
            Args:    []string{"db-server.js"},
            Env:     map[string]string{"DB_URL": "postgres://localhost/mydb"},
        },
    },
})
```

The adapter writes MCP config to a temp file and passes it via `--mcp-config`. The file is cleaned up on `Stop()`.

## Sub-agents

Define named agents that Claude can delegate tasks to:

```go
adapter.Start(ctx, ai.AdapterConfig{
    Agents: map[string]ai.AgentDef{
        "researcher": {
            Description: "Searches the codebase for patterns and answers questions",
            Prompt:      "You are a code research agent. Find relevant code.",
            Tools:       []string{"Grep", "Glob", "Read"},
            Model:       "haiku",
        },
        "test-runner": {
            Description: "Runs tests and reports results",
            Prompt:      "Run the test suite and summarize failures.",
            Tools:       []string{"Bash"},
            Model:       "sonnet",
        },
    },
})
```

Available model values: `sonnet`, `opus`, `haiku`, `inherit`.

## Tool Filtering

Control which tools the agent can use:

```go
// Whitelist — only these tools are available
adapter.Start(ctx, ai.AdapterConfig{
    AllowedTools: []string{"Read", "Grep", "Glob"},
})

// Blacklist — these tools are blocked
adapter.Start(ctx, ai.AdapterConfig{
    DisallowedTools: []string{"Write", "Bash"},
})
```

## Context Window and Compaction

The adapter tracks estimated token usage and automatically compacts the conversation when it reaches 80% of the context window:

```go
adapter.Start(ctx, ai.AdapterConfig{
    ContextWindow: 100000, // 100k tokens — compaction triggers at 80k
})
```

Compaction works by:
1. Summarizing the conversation history via a one-shot Claude query
2. Starting a fresh session with the summary as context
3. Resetting the token estimate

Default context window is 200,000 tokens.

## SendOption

Per-turn options passed to `Send`:

| Option | Description |
|---|---|
| `ai.WithMaxTokens(n)` | Maximum output tokens for this turn |
| `ai.WithTemperature(t)` | Sampling temperature (0.0–1.0) |
| `ai.WithStopSequences(s)` | Stop generation at these sequences |
| `ai.WithTools(tools)` | Override allowed tools for this turn only |
