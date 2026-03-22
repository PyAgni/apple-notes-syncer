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

	assert.Contains(t, string(content), "---")
	assert.Contains(t, string(content), `title: "My Note"`)
	assert.Contains(t, string(content), `account: "iCloud"`)
	assert.Contains(t, string(content), "Hello world")
}

func TestFSNoteWriter_WriteNote_WithSubdir(t *testing.T) {
	w, _ := newTestWriter(t, "notes", true)

	note := newTestNote("Test", "Work", "Content\n")
	relPath, err := w.WriteNote(context.Background(), &note)
	require.NoError(t, err)

	assert.Equal(t, filepath.Join("notes", "Work", "Test.md"), relPath)
}

func TestFSNoteWriter_WriteNote_NoFrontMatter(t *testing.T) {
	w, base := newTestWriter(t, "", false)

	note := newTestNote("Simple", "Notes", "Just content\n")
	relPath, err := w.WriteNote(context.Background(), &note)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(base, relPath))
	require.NoError(t, err)

	assert.NotContains(t, string(content), "---")
	assert.Equal(t, "Just content\n", string(content))
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

func TestEscapeFrontMatterString(t *testing.T) {
	assert.Equal(t, `Hello \"World\"`, escapeFrontMatterString(`Hello "World"`))
	assert.Equal(t, "No quotes", escapeFrontMatterString("No quotes"))
}

func TestFSNoteWriter_WriteNote_FrontMatterContent(t *testing.T) {
	w, base := newTestWriter(t, "", true)

	note := newTestNote("Title With \"Quotes\"", "Notes", "Body\n")
	relPath, err := w.WriteNote(context.Background(), &note)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(base, relPath))
	require.NoError(t, err)

	s := string(content)
	assert.True(t, strings.HasPrefix(s, "---\n"))
	assert.Contains(t, s, `title: "Title With \"Quotes\""`)
	assert.Contains(t, s, "created: 2026-03-18T16:00:00Z")
	assert.Contains(t, s, "modified: 2026-03-18T17:00:00Z")
	assert.Contains(t, s, "shared: false")
}
