package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/agni/apple-notes-sync/internal/model"
)

func newTestWriter(t *testing.T, subdir string, frontMatter bool) (*FSNoteWriter, string) {
	t.Helper()
	base := t.TempDir()
	return NewFSNoteWriter(base, subdir, frontMatter, "_attachments", zap.NewNop()), base
}

func newTestNote(name, folder, body string) model.Note {
	return model.Note{
		ID:           "x-coredata://test-id",
		Name:         name,
		BodyMarkdown: body,
		FolderPath:   folder,
		Account:      "iCloud",
		CreatedAt:    time.Date(2026, 3, 18, 16, 0, 0, 0, time.UTC),
		ModifiedAt:   time.Date(2026, 3, 18, 17, 0, 0, 0, time.UTC),
	}
}

func TestFSNoteWriter_WriteNote_BasicNote(t *testing.T) {
	w, base := newTestWriter(t, "", true)

	note := newTestNote("My Note", "Notes", "Hello world\n")
	relPath, err := w.WriteNote(context.Background(), &note)
	require.NoError(t, err)

	assert.Equal(t, filepath.Join("Notes", "My Note.md"), relPath)

	content, err := os.ReadFile(filepath.Join(base, relPath))
	require.NoError(t, err)

	s := string(content)
	// Title as heading at the top.
	assert.True(t, strings.HasPrefix(s, "# My Note\n"))
	// Body content.
	assert.Contains(t, s, "Hello world")
	// Metadata table at the bottom with divider.
	assert.Contains(t, s, "\n---\n")
	assert.Contains(t, s, "| ID | Created | Modified | Account | Shared |")
	assert.Contains(t, s, "| x-coredata://test-id |")
	assert.Contains(t, s, "| iCloud |")
}

func TestFSNoteWriter_WriteNote_WithSubdir(t *testing.T) {
	w, _ := newTestWriter(t, "notes", true)

	note := newTestNote("Test", "Work", "Content\n")
	relPath, err := w.WriteNote(context.Background(), &note)
	require.NoError(t, err)

	assert.Equal(t, filepath.Join("notes", "Work", "Test.md"), relPath)
}

func TestFSNoteWriter_WriteNote_NoMetadata(t *testing.T) {
	w, base := newTestWriter(t, "", false)

	note := newTestNote("Simple", "Notes", "Just content\n")
	relPath, err := w.WriteNote(context.Background(), &note)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(base, relPath))
	require.NoError(t, err)

	s := string(content)
	// Title heading is always present.
	assert.True(t, strings.HasPrefix(s, "# Simple\n"))
	assert.Contains(t, s, "Just content")
	// No metadata table.
	assert.NotContains(t, s, "| ID |")
}

func TestFSNoteWriter_WriteNote_NestedFolders(t *testing.T) {
	w, base := newTestWriter(t, "", true)

	note := newTestNote("Deep Note", "Work/Projects/Go", "Nested\n")
	relPath, err := w.WriteNote(context.Background(), &note)
	require.NoError(t, err)

	assert.Equal(t, filepath.Join("Work", "Projects", "Go", "Deep Note.md"), relPath)

	_, err = os.Stat(filepath.Join(base, "Work", "Projects", "Go", "Deep Note.md"))
	require.NoError(t, err)
}

func TestFSNoteWriter_WriteNote_SanitizedFilename(t *testing.T) {
	w, _ := newTestWriter(t, "", false)

	note := newTestNote("My/Special:Note*Title", "Notes", "Content\n")
	relPath, err := w.WriteNote(context.Background(), &note)
	require.NoError(t, err)

	assert.Equal(t, filepath.Join("Notes", "My-Special-Note-Title.md"), relPath)
}

func TestFSNoteWriter_WriteNote_ContextCancelled(t *testing.T) {
	w, _ := newTestWriter(t, "", false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	note := newTestNote("Test", "Notes", "Content\n")
	_, err := w.WriteNote(ctx, &note)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context cancelled")
}

func TestFSNoteWriter_WriteAll(t *testing.T) {
	w, _ := newTestWriter(t, "", false)

	notes := []model.Note{
		newTestNote("Note 1", "Notes", "Body 1\n"),
		newTestNote("Note 2", "Work", "Body 2\n"),
		newTestNote("Note 3", "Work/Projects", "Body 3\n"),
	}

	paths, err := w.WriteAll(context.Background(), notes)
	require.NoError(t, err)
	assert.Len(t, paths, 3)
}

func TestFSNoteWriter_CleanOrphanedFiles(t *testing.T) {
	w, base := newTestWriter(t, "", false)

	// Write some notes.
	notes := []model.Note{
		newTestNote("Keep", "Notes", "Keep\n"),
		newTestNote("Delete", "Notes", "Delete\n"),
	}

	paths, err := w.WriteAll(context.Background(), notes)
	require.NoError(t, err)

	// Only keep the first note.
	removed, err := w.CleanOrphanedFiles(context.Background(), paths[:1])
	require.NoError(t, err)
	assert.Len(t, removed, 1)
	assert.Contains(t, removed[0], "Delete.md")

	// Verify the kept file still exists.
	_, err = os.Stat(filepath.Join(base, paths[0]))
	require.NoError(t, err)

	// Verify the deleted file is gone.
	_, err = os.Stat(filepath.Join(base, paths[1]))
	assert.True(t, os.IsNotExist(err))
}

func TestFSNoteWriter_CleanOrphanedFiles_EmptyDir(t *testing.T) {
	w, _ := newTestWriter(t, "", false)

	// No files written, nothing to clean.
	removed, err := w.CleanOrphanedFiles(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, removed)
}

func TestFSNoteWriter_SaveAttachment(t *testing.T) {
	w, base := newTestWriter(t, "", false)

	// First write a note.
	note := newTestNote("Note", "Notes", "Content\n")
	notePath, err := w.WriteNote(context.Background(), &note)
	require.NoError(t, err)

	// Save an attachment.
	attachment := &model.Attachment{
		Type: model.AttachmentImage,
		Name: "photo.png",
		Data: []byte("fake-png-data"),
	}

	relPath, err := w.SaveAttachment(context.Background(), notePath, attachment)
	require.NoError(t, err)

	assert.Contains(t, relPath, "_attachments")
	assert.Contains(t, relPath, "photo.png")

	data, err := os.ReadFile(filepath.Join(base, relPath))
	require.NoError(t, err)
	assert.Equal(t, "fake-png-data", string(data))
}

func TestFSNoteWriter_SaveAttachment_NilData(t *testing.T) {
	w, _ := newTestWriter(t, "", false)

	attachment := &model.Attachment{
		Type: model.AttachmentImage,
		Name: "photo.png",
		Data: nil,
	}

	relPath, err := w.SaveAttachment(context.Background(), "Notes/test.md", attachment)
	require.NoError(t, err)
	assert.Empty(t, relPath)
}

func TestFSNoteWriter_WriteNote_MetadataTableContent(t *testing.T) {
	w, base := newTestWriter(t, "", true)

	note := newTestNote("My Title", "Notes", "Body\n")
	note.Shared = true
	relPath, err := w.WriteNote(context.Background(), &note)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(base, relPath))
	require.NoError(t, err)

	s := string(content)
	// Title heading at top.
	assert.True(t, strings.HasPrefix(s, "# My Title\n"))
	// Body before divider.
	assert.Contains(t, s, "Body\n")
	// Metadata table with correct values.
	assert.Contains(t, s, "2026-03-18 16:00:00")
	assert.Contains(t, s, "2026-03-18 17:00:00")
	assert.Contains(t, s, "| iCloud |")
	assert.Contains(t, s, "| Yes |")
}

func TestFSNoteWriter_WriteNote_MetadataSharedNo(t *testing.T) {
	w, base := newTestWriter(t, "", true)

	note := newTestNote("Test", "Notes", "Content\n")
	relPath, err := w.WriteNote(context.Background(), &note)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(base, relPath))
	require.NoError(t, err)

	assert.Contains(t, string(content), "| No |")
}
