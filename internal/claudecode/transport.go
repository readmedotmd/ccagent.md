package claudecode

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	channelBufferSize      = 10
	terminationTimeoutSecs = 5
	maxScanTokenSize       = 1024 * 1024 // 1MB
)

// transport implements subprocess communication with Claude Code CLI.
type transport struct {
	cmd     *exec.Cmd
	cliPath string
	options *Options

	connected bool
	mu        sync.RWMutex

	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr *os.File // temp file for stderr isolation

	mcpConfigFile *os.File

	jsonParser *parser

	msgChan chan Message
	errChan chan error

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func newTransport(cliPath string, options *Options) *transport {
	return &transport{
		cliPath:    cliPath,
		options:    options,
		jsonParser: newParser(),
	}
}

func (t *transport) connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connected {
		return fmt.Errorf("transport already connected")
	}

	opts, err := t.prepareMcpConfig()
	if err != nil {
		return err
	}

	args := buildCommand(t.cliPath, opts, false) // streaming mode
	t.cmd = exec.CommandContext(ctx, args[0], args[1:]...)

	// Environment: the full parent environment is passed to the subprocess.
	// This is intentional — the Claude CLI needs PATH, HOME, API keys, etc.
	// Callers should sanitize their environment before constructing the adapter
	// if they need to restrict what the subprocess can access.
	env := os.Environ()
	env = append(env, "CLAUDE_CODE_ENTRYPOINT=sdk-go-client")
	if t.options != nil && t.options.ExtraEnv != nil {
		for key, value := range t.options.ExtraEnv {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
	}
	t.cmd.Env = env

	// Working directory
	if t.options != nil && t.options.Cwd != nil {
		if err := validateWorkingDirectory(*t.options.Cwd); err != nil {
			return err
		}
		t.cmd.Dir = *t.options.Cwd
	}

	// I/O pipes
	t.stdin, err = t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	t.stdout, err = t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Stderr handling
	if t.options != nil && t.options.DebugWriter != nil {
		t.cmd.Stderr = t.options.DebugWriter
	} else {
		stderrFile, err := os.CreateTemp("", "claude_stderr_*.log")
		if err != nil {
			return fmt.Errorf("failed to create stderr file: %w", err)
		}
		// Restrict permissions to owner-only.
		if err := stderrFile.Chmod(0o600); err != nil {
			_ = stderrFile.Close()
			_ = os.Remove(stderrFile.Name())
			return fmt.Errorf("failed to set stderr file permissions: %w", err)
		}
		t.stderr = stderrFile
		t.cmd.Stderr = t.stderr
	}

	if err := t.cmd.Start(); err != nil {
		t.cleanup()
		return NewConnectionError(fmt.Sprintf("failed to start Claude CLI: %v", err), err)
	}

	t.ctx, t.cancel = context.WithCancel(ctx)
	t.msgChan = make(chan Message, channelBufferSize)
	t.errChan = make(chan error, channelBufferSize)

	t.wg.Add(1)
	go t.handleStdout()

	t.connected = true
	return nil
}

func (t *transport) sendMessage(ctx context.Context, message StreamMessage) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.connected || t.stdin == nil {
		return fmt.Errorf("transport not connected or stdin closed")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	_, err = t.stdin.Write(append(data, '\n'))
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	return nil
}

func (t *transport) receiveMessages(_ context.Context) (<-chan Message, <-chan error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.connected {
		msgChan := make(chan Message)
		errChan := make(chan error)
		close(msgChan)
		close(errChan)
		return msgChan, errChan
	}
	return t.msgChan, t.errChan
}

func (t *transport) interrupt(_ context.Context) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.connected || t.cmd == nil || t.cmd.Process == nil {
		return fmt.Errorf("process not running")
	}
	return t.cmd.Process.Signal(os.Interrupt)
}

func (t *transport) close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return nil
	}
	t.connected = false

	if t.cancel != nil {
		t.cancel()
	}

	if t.stdin != nil {
		_ = t.stdin.Close()
		t.stdin = nil
	}

	// Wait for goroutines with timeout.
	done := make(chan struct{})
	go func() {
		t.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(terminationTimeoutSecs * time.Second):
	}

	var err error
	if t.cmd != nil && t.cmd.Process != nil {
		err = t.terminateProcess()
	}

	t.cleanup()
	return err
}

func (t *transport) handleStdout() {
	defer t.wg.Done()
	defer close(t.msgChan)
	defer close(t.errChan)

	scanner := bufio.NewScanner(t.stdout)
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	for scanner.Scan() {
		select {
		case <-t.ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		messages, err := t.jsonParser.processLine(line)
		if err != nil {
			select {
			case t.errChan <- err:
			case <-t.ctx.Done():
				return
			}
			continue
		}

		for _, msg := range messages {
			if msg == nil {
				continue
			}
			// Skip control messages.
			if _, ok := msg.(*RawControlMessage); ok {
				continue
			}
			select {
			case t.msgChan <- msg:
			case <-t.ctx.Done():
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		select {
		case t.errChan <- fmt.Errorf("stdout scanner error: %w", err):
		case <-t.ctx.Done():
		}
	}
}

func (t *transport) terminateProcess() error {
	if t.cmd == nil || t.cmd.Process == nil {
		return nil
	}

	if err := t.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		if isProcessFinished(err) {
			return nil
		}
		killErr := t.cmd.Process.Kill()
		if killErr != nil && !isProcessFinished(killErr) {
			return killErr
		}
		return nil
	}

	done := make(chan error, 1)
	cmd := t.cmd
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil && strings.Contains(err.Error(), "signal:") {
			return nil
		}
		return err
	case <-time.After(terminationTimeoutSecs * time.Second):
		if killErr := t.cmd.Process.Kill(); killErr != nil && !isProcessFinished(killErr) {
			return killErr
		}
		<-done
		return nil
	case <-t.ctx.Done():
		if killErr := t.cmd.Process.Kill(); killErr != nil && !isProcessFinished(killErr) {
			return killErr
		}
		<-done
		return nil
	}
}

func (t *transport) cleanup() {
	if t.stdout != nil {
		_ = t.stdout.Close()
		t.stdout = nil
	}
	if t.stderr != nil {
		_ = t.stderr.Close()
		_ = os.Remove(t.stderr.Name())
		t.stderr = nil
	}
	if t.mcpConfigFile != nil {
		_ = t.mcpConfigFile.Close()
		_ = os.Remove(t.mcpConfigFile.Name())
		t.mcpConfigFile = nil
	}
	t.cmd = nil
}

func (t *transport) prepareMcpConfig() (*Options, error) {
	if t.options == nil || len(t.options.McpServers) == 0 {
		return t.options, nil
	}

	serversForCLI := make(map[string]any)
	for name, config := range t.options.McpServers {
		serversForCLI[name] = config
	}

	mcpConfig := map[string]any{
		"mcpServers": serversForCLI,
	}

	configData, err := json.MarshalIndent(mcpConfig, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal MCP config: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "claude_mcp_config_*.json")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	// Restrict permissions to owner-only (MCP config may contain secrets in Env).
	if err := tmpFile.Chmod(0o600); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to set MCP config file permissions: %w", err)
	}

	if _, err := tmpFile.Write(configData); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to write MCP config: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to sync MCP config file: %w", err)
	}

	t.mcpConfigFile = tmpFile

	optsCopy := *t.options
	if optsCopy.ExtraArgs == nil {
		optsCopy.ExtraArgs = make(map[string]*string)
	} else {
		extraArgsCopy := make(map[string]*string, len(optsCopy.ExtraArgs)+1)
		for k, v := range optsCopy.ExtraArgs {
			extraArgsCopy[k] = v
		}
		optsCopy.ExtraArgs = extraArgsCopy
	}
	mcpPath := tmpFile.Name()
	optsCopy.ExtraArgs["mcp-config"] = &mcpPath
	return &optsCopy, nil
}

func isProcessFinished(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "process already finished") ||
		strings.Contains(s, "process already released") ||
		strings.Contains(s, "no child processes") ||
		strings.Contains(s, "signal: killed")
}
