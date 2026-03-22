// Package gitops provides an interface for performing git operations
// on the notes repository (add, commit, push).
package gitops

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/PyAgni/apple-notes-syncer/internal/shell"
)

// GitClient performs git operations on the notes repository.
type GitClient interface {
	// Init initializes a git repo if one doesn't already exist.
	Init(ctx context.Context) error

	// AddAll stages all changes (new, modified, deleted files).
	AddAll(ctx context.Context) error

	// HasChanges returns true if there are staged or unstaged changes.
	HasChanges(ctx context.Context) (bool, error)

	// Commit creates a commit with the given message and returns the commit hash.
	Commit(ctx context.Context, message string) (string, error)

	// Push pushes commits to the configured remote and branch.
	Push(ctx context.Context) error
}

// ShellGitClient executes git commands via shell.CommandExecutor.
type ShellGitClient struct {
	executor shell.CommandExecutor
	repoPath string
	remote   string
	branch   string
	logger   *zap.Logger
}

// NewShellGitClient creates a new git client that executes commands in the
// specified repository directory.
func NewShellGitClient(executor shell.CommandExecutor, repoPath, remote, branch string, logger *zap.Logger) *ShellGitClient {
	return &ShellGitClient{
		executor: executor,
		repoPath: repoPath,
		remote:   remote,
		branch:   branch,
		logger:   logger,
	}
}

// Init initializes a git repository if one doesn't already exist.
func (g *ShellGitClient) Init(ctx context.Context) error {
	result, err := g.executor.ExecuteInDir(ctx, g.repoPath, "git", "rev-parse", "--is-inside-work-tree")
	if err == nil && strings.TrimSpace(result.Stdout) == "true" {
		g.logger.Debug("git repo already initialized")
		return nil
	}

	_, err = g.executor.ExecuteInDir(ctx, g.repoPath, "git", "init")
	if err != nil {
		return fmt.Errorf("initializing git repo: %w", err)
	}

	g.logger.Info("initialized git repo", zap.String("path", g.repoPath))
	return nil
}

// AddAll stages all changes in the repository.
func (g *ShellGitClient) AddAll(ctx context.Context) error {
	_, err := g.executor.ExecuteInDir(ctx, g.repoPath, "git", "add", "-A")
	if err != nil {
		return fmt.Errorf("staging changes: %w", err)
	}

	g.logger.Debug("staged all changes")
	return nil
}

// HasChanges returns true if there are any staged or unstaged changes.
func (g *ShellGitClient) HasChanges(ctx context.Context) (bool, error) {
	result, err := g.executor.ExecuteInDir(ctx, g.repoPath, "git", "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("checking git status: %w", err)
	}

	return strings.TrimSpace(result.Stdout) != "", nil
}

// Commit creates a commit with the given message and returns the short hash.
func (g *ShellGitClient) Commit(ctx context.Context, message string) (string, error) {
	_, err := g.executor.ExecuteInDir(ctx, g.repoPath, "git", "commit", "-m", message)
	if err != nil {
		return "", fmt.Errorf("creating commit: %w", err)
	}

	result, err := g.executor.ExecuteInDir(ctx, g.repoPath, "git", "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("getting commit hash: %w", err)
	}

	hash := strings.TrimSpace(result.Stdout)
	g.logger.Info("committed changes", zap.String("hash", hash))
	return hash, nil
}

// Push pushes commits to the configured remote and branch.
func (g *ShellGitClient) Push(ctx context.Context) error {
	_, err := g.executor.ExecuteInDir(ctx, g.repoPath, "git", "push", g.remote, g.branch)
	if err != nil {
		return fmt.Errorf("pushing to %s/%s: %w", g.remote, g.branch, err)
	}

	g.logger.Info("pushed to remote", zap.String("remote", g.remote), zap.String("branch", g.branch))
	return nil
}
