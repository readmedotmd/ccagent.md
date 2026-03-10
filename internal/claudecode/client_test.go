package claudecode

import (
	"context"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient()
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewClientWithOptions(t *testing.T) {
	c := NewClient(
		WithModel("opus"),
		WithCLIPath("/custom/claude"),
	)
	impl := c.(*clientImpl)
	if impl.options.Model == nil || *impl.options.Model != "opus" {
		t.Fatal("expected model option")
	}
	if impl.options.CLIPath == nil || *impl.options.CLIPath != "/custom/claude" {
		t.Fatal("expected CLI path option")
	}
}

func TestClientQueryNotConnected(t *testing.T) {
	c := NewClient()
	err := c.Query(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error when not connected")
	}
}

func TestClientQueryWithSessionNotConnected(t *testing.T) {
	c := NewClient()
	err := c.QueryWithSession(context.Background(), "hello", "sess-1")
	if err == nil {
		t.Fatal("expected error when not connected")
	}
}

func TestClientQueryWithSessionDefaultID(t *testing.T) {
	c := NewClient()
	// Empty session ID should use default
	err := c.QueryWithSession(context.Background(), "hello", "")
	if err == nil {
		t.Fatal("expected error when not connected")
	}
}

func TestClientQueryStreamNotConnected(t *testing.T) {
	c := NewClient()
	ch := make(chan StreamMessage)
	close(ch)
	err := c.QueryStream(context.Background(), ch)
	if err == nil {
		t.Fatal("expected error when not connected")
	}
}

func TestClientReceiveMessagesNotConnected(t *testing.T) {
	c := NewClient()
	ch := c.ReceiveMessages(context.Background())
	// Should return a closed channel
	_, ok := <-ch
	if ok {
		t.Fatal("expected closed channel")
	}
}

func TestClientDisconnectNotConnected(t *testing.T) {
	c := NewClient()
	err := c.Disconnect()
	if err != nil {
		t.Fatalf("disconnect when not connected should succeed: %v", err)
	}
}

func TestClientConnectCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := NewClient()
	err := c.Connect(ctx)
	if err == nil {
		t.Fatal("expected error with cancelled context")
	}
}

func TestClientQueryCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := NewClient()
	err := c.Query(ctx, "hello")
	if err == nil {
		t.Fatal("expected error with cancelled context")
	}
}

func TestClientQueryWithSessionCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := NewClient()
	err := c.QueryWithSession(ctx, "hello", "sess")
	if err == nil {
		t.Fatal("expected error with cancelled context")
	}
}

func TestClientReceiveErrorsNotConnected(t *testing.T) {
	c := NewClient()
	ch := c.ReceiveErrors()
	// Should return a closed channel.
	_, ok := <-ch
	if ok {
		t.Fatal("expected closed channel")
	}
}
