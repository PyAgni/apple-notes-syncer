package applescript

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	// ResolveAttachments locates attachment files in the Apple Notes media
	// directory and populates the Data field for each attachment. Attachments
	// larger than maxSizeMB are skipped.
	ResolveAttachments(ctx context.Context, notes []model.Note, maxSizeMB int) error
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

// notesMediaDir is the directory where Apple Notes stores attachment files.
const notesMediaDir = "Library/Group Containers/group.com.apple.notes"

// ResolveAttachments walks the Apple Notes media directory and populates
// attachment Data fields by matching filenames. Attachments larger than
// maxSizeMB are skipped.
func (e *AppleScriptExtractor) ResolveAttachments(ctx context.Context, notes []model.Note, maxSizeMB int) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	mediaRoot := filepath.Join(homeDir, notesMediaDir)
	if _, err := os.Stat(mediaRoot); os.IsNotExist(err) {
		e.logger.Warn("Apple Notes media directory not found", zap.String("path", mediaRoot))
		return nil
	}

	return e.resolveAttachmentsFromDir(ctx, mediaRoot, notes, maxSizeMB)
}

// resolveAttachmentsFromDir is the core implementation of ResolveAttachments,
// separated to allow testing with a custom directory.
func (e *AppleScriptExtractor) resolveAttachmentsFromDir(ctx context.Context, mediaRoot string, notes []model.Note, maxSizeMB int) error {
	fileIndex, err := buildFileIndex(ctx, mediaRoot)
	if err != nil {
		return fmt.Errorf("indexing Apple Notes media: %w", err)
	}

	e.logger.Debug("built attachment file index", zap.Int("files", len(fileIndex)))

	maxBytes := int64(maxSizeMB) * 1024 * 1024
	resolved := 0

	for i := range notes {
		for j := range notes[i].Attachments {
			att := &notes[i].Attachments[j]
			if att.Name == "" {
				continue
			}

			paths, ok := fileIndex[att.Name]
			if !ok || len(paths) == 0 {
				e.logger.Debug("attachment file not found in media directory",
					zap.String("name", att.Name),
					zap.String("note", notes[i].Name),
				)
				continue
			}

			// Use the first match. If multiple exist, prefer the one matching
			// the content identifier if possible.
			filePath := paths[0]
			if att.ContentID != "" && len(paths) > 1 {
				for _, p := range paths {
					if strings.Contains(p, att.ContentID) {
						filePath = p
						break
					}
				}
			}

			info, err := os.Stat(filePath)
			if err != nil {
				e.logger.Debug("cannot stat attachment file", zap.String("path", filePath), zap.Error(err))
				continue
			}

			if info.Size() > maxBytes {
				e.logger.Debug("skipping oversized attachment",
					zap.String("name", att.Name),
					zap.Int64("size_bytes", info.Size()),
					zap.Int("max_mb", maxSizeMB),
				)
				continue
			}

			data, err := os.ReadFile(filePath)
			if err != nil {
				e.logger.Warn("failed to read attachment file",
					zap.String("path", filePath),
					zap.Error(err),
				)
				continue
			}

			att.Data = data
			resolved++
		}
	}

	e.logger.Info("resolved attachments", zap.Int("count", resolved))
	return nil
}

// buildFileIndex walks a directory tree and returns a map from filename to
// all absolute paths where that filename exists.
func buildFileIndex(ctx context.Context, root string) (map[string][]string, error) {
	index := make(map[string][]string)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible files.
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
		}

		if info.IsDir() {
			return nil
		}

		index[info.Name()] = append(index[info.Name()], path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %q: %w", root, err)
	}

	return index, nil
}
