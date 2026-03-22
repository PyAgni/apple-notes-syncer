// Package filesystem handles writing notes as Markdown files to disk,
// managing directory structure, and cleaning up orphaned files.
package filesystem

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"go.uber.org/zap"

	"github.com/agni/apple-notes-sync/internal/model"
)

// NoteWriter manages writing notes to the filesystem.
type NoteWriter interface {
	// WriteNote writes a single note as a .md file to the correct folder.
	// Returns the relative file path written.
	WriteNote(ctx context.Context, note *model.Note) (string, error)

	// WriteAll writes all notes, creating directories as needed.
	// Returns a list of all relative file paths written.
	WriteAll(ctx context.Context, notes []model.Note) ([]string, error)

	// CleanOrphanedFiles removes .md files in the notes directory that are
	// not in the provided set of current note paths.
	// Returns a list of removed relative file paths.
	CleanOrphanedFiles(ctx context.Context, currentNotePaths []string) ([]string, error)

	// SaveAttachment writes an attachment to disk alongside its note.
	// Returns the relative file path of the saved attachment.
	SaveAttachment(ctx context.Context, notePath string, attachment *model.Attachment) (string, error)
}

// FSNoteWriter is the real filesystem implementation of NoteWriter.
type FSNoteWriter struct {
	basePath      string // Root directory of the repo.
	notesSubdir   string // Subdirectory for notes (empty = repo root).
	frontMatter   bool   // Whether to add YAML front matter.
	attachmentDir string // Subdirectory name for attachments.
	logger        *zap.Logger
}

// NewFSNoteWriter creates a new filesystem-based note writer.
func NewFSNoteWriter(basePath, notesSubdir string, frontMatter bool, attachmentDir string, logger *zap.Logger) *FSNoteWriter {
	return &FSNoteWriter{
		basePath:      basePath,
		notesSubdir:   notesSubdir,
		frontMatter:   frontMatter,
		attachmentDir: attachmentDir,
		logger:        logger,
	}
}

// metadataTableTemplate renders a Markdown table with note metadata,
// placed at the bottom of the file after a horizontal rule.
var metadataTableTemplate = template.Must(template.New("metadata").Parse(`
---

| ID | Created | Modified | Account | Shared |
|----|---------|----------|---------|--------|
| {{.ID}} | {{.Created}} | {{.Modified}} | {{.Account}} | {{.Shared}} |
`))

// metadataData holds the template data for the metadata table.
type metadataData struct {
	ID       string
	Created  string
	Modified string
	Account  string
	Shared   string
}

// notesDir returns the absolute path to the notes directory.
func (w *FSNoteWriter) notesDir() string {
	if w.notesSubdir != "" {
		return filepath.Join(w.basePath, w.notesSubdir)
	}
	return w.basePath
}

// WriteNote writes a single note as a .md file and returns the relative path.
func (w *FSNoteWriter) WriteNote(ctx context.Context, note *model.Note) (string, error) {
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("context cancelled: %w", ctx.Err())
	default:
	}

	// Build the directory path from the folder hierarchy.
	dirPath := filepath.Join(w.notesDir(), filepath.FromSlash(note.FolderPath))
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", fmt.Errorf("creating directory %q: %w", dirPath, err)
	}

	// Build filename from sanitized note name.
	fileName := note.SanitizedFileName() + ".md"
	fullPath := filepath.Join(dirPath, fileName)

	// Build file content: title heading, body, then metadata table at bottom.
	var content strings.Builder

	// Title as a top-level heading.
	content.WriteString("# ")
	content.WriteString(note.Name)
	content.WriteString("\n\n")

	// Note body.
	content.WriteString(note.BodyMarkdown)

	// Metadata table at the bottom after a divider.
	if w.frontMatter {
		shared := "No"
		if note.Shared {
			shared = "Yes"
		}
		data := metadataData{
			ID:       note.ID,
			Created:  note.CreatedAt.Format("2006-01-02 15:04:05"),
			Modified: note.ModifiedAt.Format("2006-01-02 15:04:05"),
			Account:  note.Account,
			Shared:   shared,
		}

		var buf bytes.Buffer
		if err := metadataTableTemplate.Execute(&buf, data); err != nil {
			return "", fmt.Errorf("rendering metadata for %q: %w", note.Name, err)
		}
		content.WriteString(buf.String())
	}

	if err := os.WriteFile(fullPath, []byte(content.String()), 0644); err != nil {
		return "", fmt.Errorf("writing note file %q: %w", fullPath, err)
	}

	// Return relative path from base.
	relPath, err := filepath.Rel(w.basePath, fullPath)
	if err != nil {
		return "", fmt.Errorf("computing relative path for %q: %w", fullPath, err)
	}

	w.logger.Debug("wrote note", zap.String("path", relPath), zap.String("note", note.Name))
	return relPath, nil
}

// WriteAll writes all notes to disk and returns the list of relative paths.
func (w *FSNoteWriter) WriteAll(ctx context.Context, notes []model.Note) ([]string, error) {
	var paths []string
	for i := range notes {
		relPath, err := w.WriteNote(ctx, &notes[i])
		if err != nil {
			return paths, fmt.Errorf("writing note %q: %w", notes[i].Name, err)
		}
		paths = append(paths, relPath)
	}
	return paths, nil
}

// CleanOrphanedFiles removes .md files that are not in the currentNotePaths set.
func (w *FSNoteWriter) CleanOrphanedFiles(ctx context.Context, currentNotePaths []string) ([]string, error) {
	currentSet := make(map[string]bool, len(currentNotePaths))
	for _, p := range currentNotePaths {
		currentSet[p] = true
	}

	var removed []string
	notesRoot := w.notesDir()

	err := filepath.Walk(notesRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("walking %q: %w", path, err)
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
		}

		if info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}

		relPath, err := filepath.Rel(w.basePath, path)
		if err != nil {
			return fmt.Errorf("computing relative path for %q: %w", path, err)
		}

		if !currentSet[relPath] {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("removing orphaned file %q: %w", path, err)
			}
			removed = append(removed, relPath)
			w.logger.Info("removed orphaned note", zap.String("path", relPath))
		}

		return nil
	})
	if err != nil {
		return removed, fmt.Errorf("cleaning orphaned files: %w", err)
	}

	// Clean up empty directories.
	if err := removeEmptyDirs(notesRoot); err != nil {
		w.logger.Warn("failed to clean empty directories", zap.Error(err))
	}

	return removed, nil
}

// SaveAttachment writes an attachment file to disk in the attachments
// subdirectory alongside the note.
func (w *FSNoteWriter) SaveAttachment(ctx context.Context, notePath string, attachment *model.Attachment) (string, error) {
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("context cancelled: %w", ctx.Err())
	default:
	}

	if attachment.Data == nil {
		return "", nil
	}

	// Attachments go in a subdirectory next to the note file.
	noteDir := filepath.Dir(filepath.Join(w.basePath, notePath))
	attachDir := filepath.Join(noteDir, w.attachmentDir)

	if err := os.MkdirAll(attachDir, 0755); err != nil {
		return "", fmt.Errorf("creating attachment directory %q: %w", attachDir, err)
	}

	fileName := attachment.Name
	if fileName == "" {
		fileName = "attachment"
	}

	fullPath := filepath.Join(attachDir, fileName)
	if err := os.WriteFile(fullPath, attachment.Data, 0644); err != nil {
		return "", fmt.Errorf("writing attachment %q: %w", fullPath, err)
	}

	relPath, err := filepath.Rel(w.basePath, fullPath)
	if err != nil {
		return "", fmt.Errorf("computing relative path for attachment %q: %w", fullPath, err)
	}

	w.logger.Debug("saved attachment", zap.String("path", relPath))
	return relPath, nil
}

// removeEmptyDirs walks a directory tree bottom-up and removes empty directories.
func removeEmptyDirs(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors during cleanup.
		}
		if !info.IsDir() || path == root {
			return nil
		}

		entries, err := os.ReadDir(path)
		if err != nil {
			return nil
		}
		if len(entries) == 0 {
			os.Remove(path) //nolint:errcheck
		}
		return nil
	})
}
