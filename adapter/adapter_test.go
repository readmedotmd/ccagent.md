package ai_adapters

import (
	"errors"
	"testing"
)

func TestAdapterErrorWithCause(t *testing.T) {
	cause := errors.New("connection refused")
	e := &AdapterError{Code: ErrCrashed, Message: "adapter died", Err: cause}
	if e.Error() != "adapter died: connection refused" {
		t.Fatalf("unexpected error string: %q", e.Error())
	}
	if !errors.Is(e, cause) {
		t.Fatal("expected Unwrap to return cause")
	}
}

func TestAdapterErrorWithoutCause(t *testing.T) {
	e := &AdapterError{Code: ErrTimeout, Message: "timed out"}
	if e.Error() != "timed out" {
		t.Fatalf("unexpected error string: %q", e.Error())
	}
	if e.Unwrap() != nil {
		t.Fatal("expected nil unwrap")
	}
}

func TestAdapterErrorCodes(t *testing.T) {
	codes := []ErrorCode{
		ErrUnknown, ErrCrashed, ErrRateLimited, ErrContextLength,
		ErrAuth, ErrTimeout, ErrCancelled, ErrPermission,
	}
	for i, code := range codes {
		if int(code) != i {
			t.Fatalf("expected error code %d to equal %d", code, i)
		}
	}
}

func TestAdapterStatuses(t *testing.T) {
	if StatusIdle != 0 || StatusRunning != 1 || StatusStopped != 2 || StatusError != 3 {
		t.Fatal("unexpected status values")
	}
}

func TestPermissionModes(t *testing.T) {
	if PermissionDefault != "default" {
		t.Fatal("unexpected default permission")
	}
	if PermissionAcceptAll != "accept_all" {
		t.Fatal("unexpected accept_all permission")
	}
	if PermissionPlan != "plan" {
		t.Fatal("unexpected plan permission")
	}
}

func TestSendOptions(t *testing.T) {
	opts := &SendOptions{}

	WithMaxTokens(1024)(opts)
	if opts.MaxTokens != 1024 {
		t.Fatalf("expected 1024, got %d", opts.MaxTokens)
	}

	WithStopSequences([]string{"STOP", "END"})(opts)
	if len(opts.StopSequences) != 2 || opts.StopSequences[0] != "STOP" {
		t.Fatalf("unexpected stop sequences: %v", opts.StopSequences)
	}

	WithTemperature(0.7)(opts)
	if opts.Temperature != 0.7 {
		t.Fatalf("expected 0.7, got %f", opts.Temperature)
	}

	WithTools([]string{"read", "write"})(opts)
	if len(opts.Tools) != 2 || opts.Tools[0] != "read" {
		t.Fatalf("unexpected tools: %v", opts.Tools)
	}
}

func TestAdapterCapabilitiesDefaults(t *testing.T) {
	caps := AdapterCapabilities{}
	if caps.SupportsStreaming || caps.MaxContextWindow != 0 {
		t.Fatal("zero value should be falsy")
	}
}
