package claudecode

import (
	"context"
	"fmt"
	"sync"
)

const defaultSessionID = "default"

// Client provides bidirectional streaming communication with Claude Code CLI.
type Client interface {
	Connect(ctx context.Context) error
	Disconnect() error
	Query(ctx context.Context, prompt string) error
	QueryWithSession(ctx context.Context, prompt string, sessionID string) error
	QueryStream(ctx context.Context, messages <-chan StreamMessage) error
	ReceiveMessages(ctx context.Context) <-chan Message
	ReceiveErrors() <-chan error
}

// clientImpl implements the Client interface.
type clientImpl struct {
	mu        sync.RWMutex
	transport *transport
	options   *Options
	connected bool
	msgChan   <-chan Message
	errChan   <-chan error
}

// NewClient creates a new Client with the given options.
func NewClient(opts ...Option) Client {
	options := NewOptions(opts...)
	return &clientImpl{options: options}
}

// Connect establishes a connection to the Claude Code CLI.
func (c *clientImpl) Connect(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Find CLI
	cliPath := ""
	if c.options != nil && c.options.CLIPath != nil {
		cliPath = *c.options.CLIPath
	} else {
		var err error
		cliPath, err = findCLI()
		if err != nil {
			return fmt.Errorf("claude CLI not found: %w", err)
		}
	}

	c.transport = newTransport(cliPath, c.options)

	if err := c.transport.connect(ctx); err != nil {
		return fmt.Errorf("failed to connect transport: %w", err)
	}

	c.msgChan, c.errChan = c.transport.receiveMessages(ctx)
	c.connected = true
	return nil
}

// Disconnect closes the connection to the Claude Code CLI.
func (c *clientImpl) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.transport != nil && c.connected {
		if err := c.transport.close(); err != nil {
			return fmt.Errorf("failed to close transport: %w", err)
		}
	}
	c.connected = false
	c.transport = nil
	c.msgChan = nil
	c.errChan = nil
	return nil
}

// Query sends a simple text query using the default session.
func (c *clientImpl) Query(ctx context.Context, prompt string) error {
	return c.queryWithSession(ctx, prompt, defaultSessionID)
}

// QueryWithSession sends a simple text query using the specified session ID.
func (c *clientImpl) QueryWithSession(ctx context.Context, prompt string, sessionID string) error {
	if sessionID == "" {
		sessionID = defaultSessionID
	}
	return c.queryWithSession(ctx, prompt, sessionID)
}

func (c *clientImpl) queryWithSession(ctx context.Context, prompt string, sessionID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	c.mu.RLock()
	connected := c.connected
	tr := c.transport
	c.mu.RUnlock()

	if !connected || tr == nil {
		return fmt.Errorf("client not connected")
	}

	streamMsg := StreamMessage{
		Type: "user",
		Message: map[string]any{
			"role":    "user",
			"content": prompt,
		},
		SessionID: sessionID,
	}

	return tr.sendMessage(ctx, streamMsg)
}

// QueryStream sends a stream of messages.
func (c *clientImpl) QueryStream(ctx context.Context, messages <-chan StreamMessage) error {
	c.mu.RLock()
	connected := c.connected
	tr := c.transport
	c.mu.RUnlock()

	if !connected || tr == nil {
		return fmt.Errorf("client not connected")
	}

	go func() {
		for {
			select {
			case msg, ok := <-messages:
				if !ok {
					return
				}
				if err := tr.sendMessage(ctx, msg); err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// ReceiveMessages returns a channel of incoming messages.
func (c *clientImpl) ReceiveMessages(_ context.Context) <-chan Message {
	c.mu.RLock()
	connected := c.connected
	msgChan := c.msgChan
	c.mu.RUnlock()

	if !connected || msgChan == nil {
		closedChan := make(chan Message)
		close(closedChan)
		return closedChan
	}

	return msgChan
}

// ReceiveErrors returns a channel of transport-level errors.
func (c *clientImpl) ReceiveErrors() <-chan error {
	c.mu.RLock()
	connected := c.connected
	errChan := c.errChan
	c.mu.RUnlock()

	if !connected || errChan == nil {
		closedChan := make(chan error)
		close(closedChan)
		return closedChan
	}

	return errChan
}
