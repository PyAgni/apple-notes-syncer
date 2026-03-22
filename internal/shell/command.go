// Package shell provides a generic interface for executing external subprocesses.
// All external process calls (osascript, git, rclone) go through the CommandExecutor
// interface, making them easy to mock in tests.
package shell

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// CommandResult holds the output of a subprocess execution.
type CommandResult struct {
	// Stdout is the standard output of the command.
	Stdout string
	// Stderr is the standard error output of the command.
	Stderr string
	// ExitCode is the process exit code (0 = success).
	ExitCode int
}

// CommandExecutor runs external subprocesses. This is the primary interface
// used across the codebase for all external command invocations.
type CommandExecutor interface {
	// Execute runs a command with the given arguments and returns its result.
	Execute(ctx context.Context, name string, args ...string) (*CommandResult, error)

	// ExecuteInDir runs a command in a specific working directory.
	ExecuteInDir(ctx context.Context, dir string, name string, args ...string) (*CommandResult, error)
}

// OSCommandExecutor is the real implementation of CommandExecutor using os/exec.
type OSCommandExecutor struct{}

// NewOSCommandExecutor creates a new OSCommandExecutor.
func NewOSCommandExecutor() *OSCommandExecutor {
	return &OSCommandExecutor{}
}

// Execute runs a command with the given arguments and returns its result.
func (e *OSCommandExecutor) Execute(ctx context.Context, name string, args ...string) (*CommandResult, error) {
	return e.executeCmd(ctx, "", name, args...)
}

// ExecuteInDir runs a command in a specific working directory.
func (e *OSCommandExecutor) ExecuteInDir(ctx context.Context, dir string, name string, args ...string) (*CommandResult, error) {
	return e.executeCmd(ctx, dir, name, args...)
}

// executeCmd is the shared implementation for Execute and ExecuteInDir.
func (e *OSCommandExecutor) executeCmd(ctx context.Context, dir string, name string, args ...string) (*CommandResult, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &CommandResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			return result, fmt.Errorf("command %q exited with code %d: %w", name, result.ExitCode, err)
		}
		return result, fmt.Errorf("command %q failed to run: %w", name, err)
	}

	return result, nil
}
