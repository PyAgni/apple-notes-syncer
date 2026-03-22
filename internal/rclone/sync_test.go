package rclone

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/PyAgni/apple-notes-syncer/internal/shell"
)

// MockCommandExecutor is a testify mock for shell.CommandExecutor.
type MockCommandExecutor struct {
	mock.Mock
}

func (m *MockCommandExecutor) Execute(ctx context.Context, name string, args ...string) (*shell.CommandResult, error) {
	callArgs := m.Called(ctx, name, args)
	if callArgs.Get(0) == nil {
		return nil, callArgs.Error(1)
	}
	return callArgs.Get(0).(*shell.CommandResult), callArgs.Error(1)
}

func (m *MockCommandExecutor) ExecuteInDir(ctx context.Context, dir string, name string, args ...string) (*shell.CommandResult, error) {
	callArgs := m.Called(ctx, dir, name, args)
	if callArgs.Get(0) == nil {
		return nil, callArgs.Error(1)
	}
	return callArgs.Get(0).(*shell.CommandResult), callArgs.Error(1)
}

func newTestSyncer(mockExec *MockCommandExecutor) *RcloneSyncer {
	return NewRcloneSyncer(mockExec, "/tmp/notes", "gdrive", "AppleNotes", nil, zap.NewNop())
}

func TestRcloneSyncer_Sync_Success(t *testing.T) {
	mockExec := new(MockCommandExecutor)
	syncer := newTestSyncer(mockExec)

	mockExec.On("Execute", mock.Anything, "rclone", []string{"sync", "/tmp/notes", "gdrive:AppleNotes"}).
		Return(&shell.CommandResult{}, nil)

	err := syncer.Sync(context.Background())
	require.NoError(t, err)
	mockExec.AssertExpectations(t)
}

func TestRcloneSyncer_Sync_WithExtraFlags(t *testing.T) {
	mockExec := new(MockCommandExecutor)
	syncer := NewRcloneSyncer(mockExec, "/tmp/notes", "gdrive", "Notes", []string{"--verbose", "--dry-run"}, zap.NewNop())

	mockExec.On("Execute", mock.Anything, "rclone", []string{"sync", "/tmp/notes", "gdrive:Notes", "--verbose", "--dry-run"}).
		Return(&shell.CommandResult{}, nil)

	err := syncer.Sync(context.Background())
	require.NoError(t, err)
	mockExec.AssertExpectations(t)
}

func TestRcloneSyncer_Sync_Error(t *testing.T) {
	mockExec := new(MockCommandExecutor)
	syncer := newTestSyncer(mockExec)

	mockExec.On("Execute", mock.Anything, "rclone", []string{"sync", "/tmp/notes", "gdrive:AppleNotes"}).
		Return(nil, assert.AnError)

	err := syncer.Sync(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rclone sync to gdrive:AppleNotes")
}

func TestRcloneSyncer_IsAvailable_Available(t *testing.T) {
	mockExec := new(MockCommandExecutor)
	syncer := newTestSyncer(mockExec)

	mockExec.On("Execute", mock.Anything, "rclone", []string{"version"}).
		Return(&shell.CommandResult{Stdout: "rclone v1.65.0"}, nil)
	mockExec.On("Execute", mock.Anything, "rclone", []string{"listremotes"}).
		Return(&shell.CommandResult{Stdout: "gdrive:\nbackup:\n"}, nil)

	available, err := syncer.IsAvailable(context.Background())
	require.NoError(t, err)
	assert.True(t, available)
	mockExec.AssertExpectations(t)
}

func TestRcloneSyncer_IsAvailable_NotInstalled(t *testing.T) {
	mockExec := new(MockCommandExecutor)
	syncer := newTestSyncer(mockExec)

	mockExec.On("Execute", mock.Anything, "rclone", []string{"version"}).
		Return(nil, assert.AnError)

	available, err := syncer.IsAvailable(context.Background())
	require.NoError(t, err)
	assert.False(t, available)
}

func TestRcloneSyncer_IsAvailable_RemoteNotFound(t *testing.T) {
	mockExec := new(MockCommandExecutor)
	syncer := newTestSyncer(mockExec)

	mockExec.On("Execute", mock.Anything, "rclone", []string{"version"}).
		Return(&shell.CommandResult{}, nil)
	mockExec.On("Execute", mock.Anything, "rclone", []string{"listremotes"}).
		Return(&shell.CommandResult{Stdout: "backup:\ns3:\n"}, nil)

	available, err := syncer.IsAvailable(context.Background())
	require.NoError(t, err)
	assert.False(t, available)
}

func TestRcloneSyncer_IsAvailable_ListRemotesError(t *testing.T) {
	mockExec := new(MockCommandExecutor)
	syncer := newTestSyncer(mockExec)

	mockExec.On("Execute", mock.Anything, "rclone", []string{"version"}).
		Return(&shell.CommandResult{}, nil)
	mockExec.On("Execute", mock.Anything, "rclone", []string{"listremotes"}).
		Return(nil, assert.AnError)

	_, err := syncer.IsAvailable(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing rclone remotes")
}
