package shell

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOSCommandExecutor_Execute_Success(t *testing.T) {
	exec := NewOSCommandExecutor()
	result, err := exec.Execute(context.Background(), "echo", "hello world")

	require.NoError(t, err)
	assert.Equal(t, "hello world\n", result.Stdout)
	assert.Empty(t, result.Stderr)
	assert.Equal(t, 0, result.ExitCode)
}

func TestOSCommandExecutor_Execute_NonZeroExit(t *testing.T) {
	exec := NewOSCommandExecutor()
	result, err := exec.Execute(context.Background(), "sh", "-c", "echo errout >&2; exit 1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exited with code 1")
	assert.Equal(t, "errout\n", result.Stderr)
	assert.Equal(t, 1, result.ExitCode)
}

func TestOSCommandExecutor_Execute_CommandNotFound(t *testing.T) {
	exec := NewOSCommandExecutor()
	_, err := exec.Execute(context.Background(), "nonexistent-command-abc123")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to run")
}

func TestOSCommandExecutor_ExecuteInDir(t *testing.T) {
	exec := NewOSCommandExecutor()
	result, err := exec.ExecuteInDir(context.Background(), "/tmp", "pwd")

	require.NoError(t, err)
	// On macOS, /tmp is a symlink to /private/tmp.
	assert.Contains(t, result.Stdout, "tmp")
}

func TestOSCommandExecutor_Execute_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	exec := NewOSCommandExecutor()
	_, err := exec.Execute(ctx, "sleep", "10")

	require.Error(t, err)
}
