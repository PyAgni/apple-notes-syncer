package gitops

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/agni/apple-notes-sync/internal/shell"
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

func newTestGit(mockExec *MockCommandExecutor) *ShellGitClient {
	return NewShellGitClient(mockExec, "/tmp/repo", "origin", "main", zap.NewNop())
}

func TestShellGitClient_Init_AlreadyInitialized(t *testing.T) {
	mockExec := new(MockCommandExecutor)
	git := newTestGit(mockExec)

	mockExec.On("ExecuteInDir", mock.Anything, "/tmp/repo", "git", []string{"rev-parse", "--is-inside-work-tree"}).
		Return(&shell.CommandResult{Stdout: "true\n"}, nil)

	err := git.Init(context.Background())
	require.NoError(t, err)
	mockExec.AssertExpectations(t)
}

func TestShellGitClient_Init_NewRepo(t *testing.T) {
	mockExec := new(MockCommandExecutor)
	git := newTestGit(mockExec)

	mockExec.On("ExecuteInDir", mock.Anything, "/tmp/repo", "git", []string{"rev-parse", "--is-inside-work-tree"}).
		Return(nil, assert.AnError)
	mockExec.On("ExecuteInDir", mock.Anything, "/tmp/repo", "git", []string{"init"}).
		Return(&shell.CommandResult{}, nil)

	err := git.Init(context.Background())
	require.NoError(t, err)
	mockExec.AssertExpectations(t)
}

func TestShellGitClient_Init_Error(t *testing.T) {
	mockExec := new(MockCommandExecutor)
	git := newTestGit(mockExec)

	mockExec.On("ExecuteInDir", mock.Anything, "/tmp/repo", "git", []string{"rev-parse", "--is-inside-work-tree"}).
		Return(nil, assert.AnError)
	mockExec.On("ExecuteInDir", mock.Anything, "/tmp/repo", "git", []string{"init"}).
		Return(nil, assert.AnError)

	err := git.Init(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "initializing git repo")
}

func TestShellGitClient_AddAll(t *testing.T) {
	mockExec := new(MockCommandExecutor)
	git := newTestGit(mockExec)

	mockExec.On("ExecuteInDir", mock.Anything, "/tmp/repo", "git", []string{"add", "-A"}).
		Return(&shell.CommandResult{}, nil)

	err := git.AddAll(context.Background())
	require.NoError(t, err)
	mockExec.AssertExpectations(t)
}

func TestShellGitClient_AddAll_Error(t *testing.T) {
	mockExec := new(MockCommandExecutor)
	git := newTestGit(mockExec)

	mockExec.On("ExecuteInDir", mock.Anything, "/tmp/repo", "git", []string{"add", "-A"}).
		Return(nil, assert.AnError)

	err := git.AddAll(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "staging changes")
}

func TestShellGitClient_HasChanges_WithChanges(t *testing.T) {
	mockExec := new(MockCommandExecutor)
	git := newTestGit(mockExec)

	mockExec.On("ExecuteInDir", mock.Anything, "/tmp/repo", "git", []string{"status", "--porcelain"}).
		Return(&shell.CommandResult{Stdout: " M file.md\n"}, nil)

	hasChanges, err := git.HasChanges(context.Background())
	require.NoError(t, err)
	assert.True(t, hasChanges)
}

func TestShellGitClient_HasChanges_NoChanges(t *testing.T) {
	mockExec := new(MockCommandExecutor)
	git := newTestGit(mockExec)

	mockExec.On("ExecuteInDir", mock.Anything, "/tmp/repo", "git", []string{"status", "--porcelain"}).
		Return(&shell.CommandResult{Stdout: ""}, nil)

	hasChanges, err := git.HasChanges(context.Background())
	require.NoError(t, err)
	assert.False(t, hasChanges)
}

func TestShellGitClient_HasChanges_Error(t *testing.T) {
	mockExec := new(MockCommandExecutor)
	git := newTestGit(mockExec)

	mockExec.On("ExecuteInDir", mock.Anything, "/tmp/repo", "git", []string{"status", "--porcelain"}).
		Return(nil, assert.AnError)

	_, err := git.HasChanges(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checking git status")
}

func TestShellGitClient_Commit(t *testing.T) {
	mockExec := new(MockCommandExecutor)
	git := newTestGit(mockExec)

	mockExec.On("ExecuteInDir", mock.Anything, "/tmp/repo", "git", []string{"commit", "-m", "sync: test"}).
		Return(&shell.CommandResult{}, nil)
	mockExec.On("ExecuteInDir", mock.Anything, "/tmp/repo", "git", []string{"rev-parse", "--short", "HEAD"}).
		Return(&shell.CommandResult{Stdout: "abc1234\n"}, nil)

	hash, err := git.Commit(context.Background(), "sync: test")
	require.NoError(t, err)
	assert.Equal(t, "abc1234", hash)
	mockExec.AssertExpectations(t)
}

func TestShellGitClient_Commit_Error(t *testing.T) {
	mockExec := new(MockCommandExecutor)
	git := newTestGit(mockExec)

	mockExec.On("ExecuteInDir", mock.Anything, "/tmp/repo", "git", []string{"commit", "-m", "sync"}).
		Return(nil, assert.AnError)

	_, err := git.Commit(context.Background(), "sync")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "creating commit")
}

func TestShellGitClient_Push(t *testing.T) {
	mockExec := new(MockCommandExecutor)
	git := newTestGit(mockExec)

	mockExec.On("ExecuteInDir", mock.Anything, "/tmp/repo", "git", []string{"push", "origin", "main"}).
		Return(&shell.CommandResult{}, nil)

	err := git.Push(context.Background())
	require.NoError(t, err)
	mockExec.AssertExpectations(t)
}

func TestShellGitClient_Push_Error(t *testing.T) {
	mockExec := new(MockCommandExecutor)
	git := newTestGit(mockExec)

	mockExec.On("ExecuteInDir", mock.Anything, "/tmp/repo", "git", []string{"push", "origin", "main"}).
		Return(nil, assert.AnError)

	err := git.Push(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pushing to origin/main")
}
