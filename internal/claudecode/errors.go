package claudecode

import (
	"errors"
	"fmt"
)

// ErrNoMoreMessages indicates the message iterator has no more messages.
var ErrNoMoreMessages = errors.New("no more messages")

// ConnectionError represents connection-related failures.
type ConnectionError struct {
	message string
	cause   error
}

func (e *ConnectionError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.message, e.cause)
	}
	return e.message
}

func (e *ConnectionError) Unwrap() error { return e.cause }

func NewConnectionError(message string, cause error) *ConnectionError {
	return &ConnectionError{message: message, cause: cause}
}

// CLINotFoundError indicates the Claude CLI was not found.
type CLINotFoundError struct {
	message string
	Path    string
}

func (e *CLINotFoundError) Error() string { return e.message }

func NewCLINotFoundError(path, message string) *CLINotFoundError {
	if path != "" {
		message = fmt.Sprintf("%s: %s", message, path)
	}
	return &CLINotFoundError{message: message, Path: path}
}

// ProcessError represents subprocess execution failures.
type ProcessError struct {
	message  string
	cause    error
	ExitCode int
	Stderr   string
}

func (e *ProcessError) Error() string {
	msg := e.message
	if e.ExitCode != 0 {
		msg = fmt.Sprintf("%s (exit code: %d)", msg, e.ExitCode)
	}
	if e.Stderr != "" {
		stderr := e.Stderr
		if len(stderr) > 500 {
			stderr = stderr[:500] + "...[truncated]"
		}
		msg = fmt.Sprintf("%s\nError output: %s", msg, stderr)
	}
	return msg
}

func (e *ProcessError) Unwrap() error { return e.cause }

// NewProcessError creates a new ProcessError.
func NewProcessError(message string, exitCode int, stderr string, cause error) *ProcessError {
	return &ProcessError{
		message:  message,
		ExitCode: exitCode,
		Stderr:   stderr,
		cause:    cause,
	}
}
