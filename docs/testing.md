# Testing

## Running Tests

```bash
# All packages
go test ./...

# Verbose
go test ./... -v

# Specific package
go test ./adapter/...
go test ./internal/claudecode/...
go test .

# Specific test
go test -run TestParserAssistantMessage ./internal/claudecode/...

# With race detector
go test -race ./...
```

## Test Coverage

Three packages, ~97 tests total. No external dependencies or network access required.

### Root Package (`claude_test.go`)

| Test | Verifies |
|---|---|
| `TestCompileTimeInterfaceChecks` | `ClaudeAdapter` satisfies `Adapter` and `SessionProvider` |
| `TestNewClaudeAdapterInitialStatus` | New adapter starts in `StatusIdle` |
| `TestStopOnIdleIsNoOp` | Stopping an idle adapter is safe |
| `TestDoubleStartReturnsError` | Starting an already-running adapter returns error |
| `TestStopOnRunningAdapter` | Stop transitions to `StatusStopped` |
| `TestStopOnAlreadyStoppedIsNoOp` | Double-stop is safe |
| `TestSendWhenNotRunning` | Send fails when adapter is not running |
| `TestSendQueuesWhenRunning` | Send queues when a turn is in progress |
| `TestSendMultipleQueuedMessages` | Multiple sends queue correctly |
| `TestCancelWhenNotRunning` | Cancel is a no-op when idle |
| `TestCancelClearsQueue` | Cancel empties the message queue |
| `TestReceiveReturnsChannel` | Receive returns the event channel |
| `TestReceiveNilWhenNoEvents` | Receive returns nil before Start |
| `TestCapabilities` | All capability flags are correct |
| `TestHealth*` | Health returns appropriate errors for each status |
| `TestSessionID*` | SessionID starts empty, reflects updates |
| `TestMessageText` | Text extraction from content blocks (6 subtests) |
| `TestMessageImages*` | Image extraction, default MIME, empty data |
| `TestCombineMessages*` | Queue message merging (4 tests) |
| `TestBase64Encode*` | Base64 encoding of binary data |
| `TestStartWithCancelledContext` | Start handles cancelled context |

### Adapter Package (`adapter/`)

**adapter_test.go**

| Test | Verifies |
|---|---|
| `TestAdapterErrorWithCause` | Error message includes cause |
| `TestAdapterErrorWithoutCause` | Error message without cause |
| `TestAdapterErrorCodes` | Error code iota values |
| `TestAdapterStatuses` | Status iota values |
| `TestPermissionModes` | Permission mode string values |
| `TestSendOptions` | All functional options apply correctly |
| `TestAdapterCapabilitiesDefaults` | Zero-value capabilities are falsy |

**message_test.go**

| Test | Verifies |
|---|---|
| `TestTextContent` | Helper creates single text block |
| `TestTextContentEmpty` | Empty string creates block with empty text |
| `TestRoleConstants` | Role string values |
| `TestContentTypeConstants` | ContentType string values |
| `TestMessageFields` | All Message fields set correctly |
| `TestContentBlockMultipart` | All content block types work |
| `TestToolCallFields` | ToolCall struct fields |
| `TestConversationFields` | Conversation struct fields |

**stream_test.go**

| Test | Verifies |
|---|---|
| `TestStreamEventTypes` | All 13 event type iota values |
| `TestStreamEvent*` | Each event type's fields (token, thinking, tool use, tool result, error, done, permission, file change, sub-agent, progress, cost) |
| `TestStreamEventFileChange` | FileChangeOp constants |

### Internal SDK (`internal/claudecode/`)

**parser_test.go**

| Test | Verifies |
|---|---|
| `TestParserAssistantMessage` | Full assistant message parsing |
| `TestParserResultMessage` | All result fields (duration, cost, session) |
| `TestParserResultMessageError` | Error result messages |
| `TestParserResultMessageNoCost` | Missing optional fields are nil |
| `TestParserSystemMessage` | System message with subtype |
| `TestParserUserMessage` | User message content extraction |
| `TestParserControlMessages` | Both control message types |
| `TestParserStreamEventSkipped` | Stream events produce no output |
| `TestParserUnknownType` | Unknown types return errors |
| `TestParserEmptyLine` | Empty/whitespace lines are no-ops |
| `TestParserIncompleteJSON` | Multi-line JSON accumulation |
| `TestParserBufferOverflow` | 1MB buffer limit |
| `TestParserToolUseBlock` | Tool use with input |
| `TestParserToolUseBlockNoInput` | Tool use with empty input |
| `TestParserThinkingBlock` | Thinking with signature |
| `TestParserToolResultBlock` | Tool result content |
| `TestParserUnknownBlockTypeSkipped` | Forward-compatible block skipping |
| `TestParserMixedContentBlocks` | Multiple block types in one message |
| `TestParserInvalid*` | Error handling for malformed messages |
| `TestParserBufferResetAfterSuccess` | Buffer cleans up between messages |
| `TestMessageTypeConstants` | All type string values |
| `TestBlockTypeConstants` | All block type string values |
| `TestMessageInterfaceTypes` | Interface compliance for all message types |
| `TestContentBlockInterfaceTypes` | Interface compliance for all block types |

**options_test.go** — all `With*` functional options and defaults.

**cli_test.go** — `buildCommand` flag generation, `validateWorkingDirectory`, constant values.

**errors_test.go** — `ConnectionError`, `CLINotFoundError`, `ProcessError`, `ErrNoMoreMessages`.

**transport_test.go** — send/receive when disconnected, MCP config file generation, cleanup, `isProcessFinished`, JSON serialization.

**client_test.go** — client creation, query methods when not connected, cancelled context handling.

## No External Test Dependencies

All tests run without:
- Network access
- Claude Code CLI installed
- Docker
- External test frameworks

The test suite validates types, parsing, command building, state management, and error handling — everything that doesn't require a live CLI subprocess.
