package applescript

import (
	"context"
	"embed"
	"fmt"

	"go.uber.org/zap"

	"github.com/PyAgni/apple-notes-syncer/internal/model"
	"github.com/PyAgni/apple-notes-syncer/internal/shell"
)

//go:embed scripts
var scripts embed.FS

// NoteExtractor fetches notes and folders from Apple Notes via AppleScript.
type NoteExtractor interface {
	// GetFolders returns the complete folder hierarchy across all accounts.
	GetFolders(ctx context.Context) ([]model.Folder, error)

	// GetAllNotes returns every note, optionally filtered by accounts and folders.
	// If accounts is nil, all accounts are included.
	// If folders is nil, all folders are included.
	GetAllNotes(ctx context.Context, accounts []string, folders []string) ([]model.Note, error)
}

// AppleScriptExtractor extracts notes from Apple Notes by executing
// AppleScript via the osascript command.
type AppleScriptExtractor struct {
	executor shell.CommandExecutor
	logger   *zap.Logger
}

// NewAppleScriptExtractor creates a new extractor that uses the given
// CommandExecutor to invoke osascript.
func NewAppleScriptExtractor(executor shell.CommandExecutor, logger *zap.Logger) *AppleScriptExtractor {
	return &AppleScriptExtractor{
		executor: executor,
		logger:   logger,
	}
}

// GetFolders returns the complete folder hierarchy from Apple Notes.
func (e *AppleScriptExtractor) GetFolders(ctx context.Context) ([]model.Folder, error) {
	script, err := scripts.ReadFile("scripts/get_folders.applescript")
	if err != nil {
		return nil, fmt.Errorf("reading get_folders script: %w", err)
	}

	e.logger.Debug("executing get_folders AppleScript")

	result, err := e.executor.Execute(ctx, "osascript", "-e", string(script))
	if err != nil {
		return nil, fmt.Errorf("executing get_folders AppleScript: %w", err)
	}

	folders, err := ParseFoldersOutput(result.Stdout)
	if err != nil {
		return nil, fmt.Errorf("parsing folders output: %w", err)
	}

	e.logger.Info("extracted folders", zap.Int("count", len(folders)))
	return folders, nil
}

// GetAllNotes returns all notes from Apple Notes, optionally filtered
// by account names and folder paths.
func (e *AppleScriptExtractor) GetAllNotes(ctx context.Context, accounts []string, folders []string) ([]model.Note, error) {
	script, err := scripts.ReadFile("scripts/get_all_notes.applescript")
	if err != nil {
		return nil, fmt.Errorf("reading get_all_notes script: %w", err)
	}

	e.logger.Debug("executing get_all_notes AppleScript")

	result, err := e.executor.Execute(ctx, "osascript", "-e", string(script))
	if err != nil {
		return nil, fmt.Errorf("executing get_all_notes AppleScript: %w", err)
	}

	notes, err := ParseNotesOutput(result.Stdout)
	if err != nil {
		return nil, fmt.Errorf("parsing notes output: %w", err)
	}

	e.logger.Info("extracted notes from AppleScript", zap.Int("total", len(notes)))

	// Apply filters.
	filtered := filterNotes(notes, accounts, folders)

	if len(filtered) != len(notes) {
		e.logger.Info("filtered notes",
			zap.Int("before", len(notes)),
			zap.Int("after", len(filtered)),
		)
	}

	return filtered, nil
}

// filterNotes applies account and folder filters to a slice of notes.
func filterNotes(notes []model.Note, accounts []string, folders []string) []model.Note {
	if len(accounts) == 0 && len(folders) == 0 {
		return notes
	}

	accountSet := toSet(accounts)
	folderSet := toSet(folders)

	var filtered []model.Note
	for _, note := range notes {
		if len(accountSet) > 0 && !accountSet[note.Account] {
			continue
		}
		if len(folderSet) > 0 && !folderSet[note.FolderPath] {
			continue
		}
		filtered = append(filtered, note)
	}

	return filtered
}

// toSet converts a string slice into a set (map) for O(1) lookups.
func toSet(items []string) map[string]bool {
	if len(items) == 0 {
		return nil
	}
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}
