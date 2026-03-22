// Package main is the entrypoint for the apple-notes-sync CLI tool.
// It sets up the cobra command, loads configuration, wires dependencies,
// and executes the sync pipeline.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/agni/apple-notes-sync/internal/applescript"
	"github.com/agni/apple-notes-sync/internal/config"
	"github.com/agni/apple-notes-sync/internal/converter"
	"github.com/agni/apple-notes-sync/internal/filesystem"
	"github.com/agni/apple-notes-sync/internal/gitops"
	"github.com/agni/apple-notes-sync/internal/logging"
	"github.com/agni/apple-notes-sync/internal/rclone"
	"github.com/agni/apple-notes-sync/internal/shell"
	"github.com/agni/apple-notes-sync/internal/syncer"
)

// Build-time variables set via -ldflags.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

// newRootCmd creates the root cobra command with all flags and configuration.
func newRootCmd() *cobra.Command {
	var cfgPath string
	var repoPath string
	var dryRun bool
	var logLevel string
	var verbose bool

	cmd := &cobra.Command{
		Use:   "apple-notes-sync",
		Short: "Export Apple Notes to a Git repository as Markdown files",
		Long: `apple-notes-sync extracts notes from the macOS Notes app via AppleScript,
converts them to Markdown, and commits them to a Git repository.
Optionally syncs to Google Drive via rclone.`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), cfgPath, repoPath, dryRun, logLevel, verbose)
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVarP(&cfgPath, "config", "c", "", "path to config file (default: ~/.apple-notes-sync.yaml)")
	cmd.Flags().StringVar(&repoPath, "repo-path", "", "path to the git repository")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview changes without writing files or committing")
	cmd.Flags().StringVar(&logLevel, "log-level", "", "log level (debug, info, warn, error)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "shortcut for --log-level=debug")

	return cmd
}

// run is the main execution function that loads config, creates dependencies,
// and runs the sync pipeline.
func run(ctx context.Context, cfgPath, repoPath string, dryRun bool, logLevel string, verbose bool) error {
	// Set up signal handling for graceful shutdown.
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Load configuration.
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Apply CLI flag overrides.
	if repoPath != "" {
		cfg.RepoPath = repoPath
	}
	if dryRun {
		cfg.DryRun = true
	}
	if verbose {
		cfg.Log.Level = "debug"
	}
	if logLevel != "" {
		cfg.Log.Level = logLevel
	}

	// Validate configuration.
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Create logger.
	logger, err := logging.NewLogger(cfg.Log.Level, cfg.Log.Format, cfg.Log.File)
	if err != nil {
		return fmt.Errorf("creating logger: %w", err)
	}
	defer logger.Sync() //nolint:errcheck

	logger.Info("apple-notes-sync starting",
		zap.String("version", version),
		zap.String("repo_path", cfg.RepoPath),
		zap.Bool("dry_run", cfg.DryRun),
	)

	// Wire dependencies.
	executor := shell.NewOSCommandExecutor()

	extractor := applescript.NewAppleScriptExtractor(executor, logger)
	conv := converter.NewHTMLToMDConverter(cfg.FrontMatter)
	writer := filesystem.NewFSNoteWriter(cfg.RepoPath, cfg.NotesSubdir, cfg.FrontMatter, cfg.Attachments.Dir, logger)
	gitClient := gitops.NewShellGitClient(executor, cfg.RepoPath, cfg.Git.Remote, cfg.Git.Branch, logger)
	rcloneSyncer := rclone.NewRcloneSyncer(executor, cfg.RepoPath, cfg.Rclone.RemoteName, cfg.Rclone.RemotePath, cfg.Rclone.ExtraFlags, logger)

	s := syncer.NewSyncer(cfg, extractor, conv, writer, gitClient, rcloneSyncer, logger)

	// Run the sync pipeline.
	result, err := s.Sync(ctx)
	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	logger.Info("sync complete",
		zap.Int("written", result.WrittenNotes),
		zap.Int("skipped", result.SkippedNotes),
		zap.Int("errors", len(result.Errors)),
		zap.String("commit", result.GitCommitHash),
		zap.Bool("rclone_synced", result.RcloneSynced),
		zap.Duration("duration", result.Duration),
	)

	return nil
}
