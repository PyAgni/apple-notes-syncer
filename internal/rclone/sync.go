// Package rclone provides an interface for syncing the notes repository
// to a cloud storage remote (e.g. Google Drive) via the rclone CLI tool.
package rclone

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/PyAgni/apple-notes-syncer/internal/shell"
)

// Syncer syncs a local directory to a cloud remote via rclone.
type Syncer interface {
	// Sync runs rclone sync to push local changes to the remote.
	Sync(ctx context.Context) error

	// IsAvailable checks if rclone is installed and the configured remote exists.
	IsAvailable(ctx context.Context) (bool, error)
}

// RcloneSyncer is the real implementation that invokes the rclone binary.
type RcloneSyncer struct {
	executor   shell.CommandExecutor
	localPath  string
	remoteName string
	remotePath string
	extraFlags []string
	logger     *zap.Logger
}

// NewRcloneSyncer creates a new rclone syncer for the given local path and remote.
func NewRcloneSyncer(executor shell.CommandExecutor, localPath, remoteName, remotePath string, extraFlags []string, logger *zap.Logger) *RcloneSyncer {
	return &RcloneSyncer{
		executor:   executor,
		localPath:  localPath,
		remoteName: remoteName,
		remotePath: remotePath,
		extraFlags: extraFlags,
		logger:     logger,
	}
}

// Sync runs `rclone sync <localPath> <remote>:<remotePath>` with any
// configured extra flags.
func (r *RcloneSyncer) Sync(ctx context.Context) error {
	dest := fmt.Sprintf("%s:%s", r.remoteName, r.remotePath)

	args := []string{"sync", r.localPath, dest}
	args = append(args, r.extraFlags...)

	r.logger.Info("starting rclone sync",
		zap.String("source", r.localPath),
		zap.String("dest", dest),
	)

	_, err := r.executor.Execute(ctx, "rclone", args...)
	if err != nil {
		return fmt.Errorf("rclone sync to %s: %w", dest, err)
	}

	r.logger.Info("rclone sync completed")
	return nil
}

// IsAvailable checks if the rclone binary is installed and the configured
// remote is present in rclone's remote list.
func (r *RcloneSyncer) IsAvailable(ctx context.Context) (bool, error) {
	// Check if rclone is installed.
	_, err := r.executor.Execute(ctx, "rclone", "version")
	if err != nil {
		return false, nil
	}

	// Check if the remote is configured.
	result, err := r.executor.Execute(ctx, "rclone", "listremotes")
	if err != nil {
		return false, fmt.Errorf("listing rclone remotes: %w", err)
	}

	remoteWithColon := r.remoteName + ":"
	for _, line := range strings.Split(result.Stdout, "\n") {
		if strings.TrimSpace(line) == remoteWithColon {
			return true, nil
		}
	}

	r.logger.Warn("rclone remote not found", zap.String("remote", r.remoteName))
	return false, nil
}
