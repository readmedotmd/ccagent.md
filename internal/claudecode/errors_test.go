package claudecode

import (
	"errors"
	"strings"
	"testing"
)

func TestConnectionErrorWithCause(t *testing.T) {
	cause := errors.New("dial tcp: connection refused")
	e := NewConnectionError("failed to connect", cause)
	if e.Error() != "failed to connect: dial tcp: connection refused" {
		t.Fatalf("unexpected error: %q", e.Error())
	}
	if !errors.Is(e, cause) {
		t.Fatal("expected unwrap to return cause")
	}
}

func TestConnectionErrorWithoutCause(t *testing.T) {
	e := NewConnectionError("not available", nil)
	if e.Error() != "not available" {
		t.Fatalf("unexpected error: %q", e.Error())
	}
	if e.Unwrap() != nil {
		t.Fatal("expected nil unwrap")
	}
}

func TestCLINotFoundErrorWithPath(t *testing.T) {
	e := NewCLINotFoundError("/usr/bin/claude", "not found")
	if e.Error() != "not found: /usr/bin/claude" {
		t.Fatalf("unexpected error: %q", e.Error())
	}
	if e.Path != "/usr/bin/claude" {
		t.Fatal("unexpected path")
	}
}

func TestCLINotFoundErrorWithoutPath(t *testing.T) {
	e := NewCLINotFoundError("", "Claude Code not found")
	if e.Error() != "Claude Code not found" {
		t.Fatalf("unexpected error: %q", e.Error())
	}
	if e.Path != "" {
		t.Fatal("expected empty path")
	}
}

func TestProcessError(t *testing.T) {
	e := &ProcessError{message: "process failed", ExitCode: 1, Stderr: "error output"}
	expected := "process failed (exit code: 1)\nError output: error output"
	if e.Error() != expected {
		t.Fatalf("unexpected error: %q", e.Error())
	}
}

func TestProcessErrorNoExitCode(t *testing.T) {
	e := &ProcessError{message: "process failed"}
	if e.Error() != "process failed" {
		t.Fatalf("unexpected error: %q", e.Error())
	}
}

func TestProcessErrorWithStderrOnly(t *testing.T) {
	e := &ProcessError{message: "crash", Stderr: "segfault"}
	if e.Error() != "crash\nError output: segfault" {
		t.Fatalf("unexpected error: %q", e.Error())
	}
}

func TestErrNoMoreMessages(t *testing.T) {
	if ErrNoMoreMessages.Error() != "no more messages" {
		t.Fatalf("unexpected error: %q", ErrNoMoreMessages.Error())
	}
}

func TestProcessErrorUnwrap(t *testing.T) {
	cause := errors.New("underlying error")
	e := NewProcessError("process failed", 1, "stderr output", cause)
	if !errors.Is(e, cause) {
		t.Fatal("expected Unwrap to return cause")
	}
	if e.ExitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", e.ExitCode)
	}
}

func TestProcessErrorUnwrapNil(t *testing.T) {
	e := &ProcessError{message: "no cause"}
	if e.Unwrap() != nil {
		t.Fatal("expected nil unwrap")
	}
}

func TestProcessErrorStderrTruncation(t *testing.T) {
	longStderr := strings.Repeat("x", 600)
	e := &ProcessError{message: "fail", Stderr: longStderr}
	errStr := e.Error()
	if !strings.Contains(errStr, "...[truncated]") {
		t.Fatal("expected truncation marker in long stderr")
	}
	if strings.Contains(errStr, strings.Repeat("x", 600)) {
		t.Fatal("full stderr should not appear in error string")
	}
}

func TestNewProcessErrorConstructor(t *testing.T) {
	cause := errors.New("io error")
	e := NewProcessError("cmd failed", 2, "bad stuff", cause)
	if e.ExitCode != 2 {
		t.Fatal("unexpected exit code")
	}
	if e.Stderr != "bad stuff" {
		t.Fatal("unexpected stderr")
	}
	if e.Unwrap() != cause {
		t.Fatal("unexpected cause")
	}
}
