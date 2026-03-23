package applescript

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/PyAgni/apple-notes-syncer/internal/model"
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

func newTestLogger() *zap.Logger {
	return zap.NewNop()
}

func TestAppleScriptExtractor_GetFolders(t *testing.T) {
	mockExec := new(MockCommandExecutor)
	extractor := NewAppleScriptExtractor(mockExec, newTestLogger())

	output := "iCloud|||FIELD|||Notes|||FIELD|||id1|||FIELD|||Notes|||FOLDER|||" +
		"iCloud|||FIELD|||Work|||FIELD|||id2|||FIELD|||Work|||FOLDER|||"

	mockExec.On("Execute", mock.Anything, "osascript", mock.Anything).
		Return(&shell.CommandResult{Stdout: output}, nil)

	folders, err := extractor.GetFolders(context.Background())
	require.NoError(t, err)
	require.Len(t, folders, 2)

	assert.Equal(t, "Notes", folders[0].Name)
	assert.Equal(t, "Work", folders[1].Name)

	mockExec.AssertExpectations(t)
}

func TestAppleScriptExtractor_GetFolders_Error(t *testing.T) {
	mockExec := new(MockCommandExecutor)
	extractor := NewAppleScriptExtractor(mockExec, newTestLogger())

	mockExec.On("Execute", mock.Anything, "osascript", mock.Anything).
		Return(nil, assert.AnError)

	_, err := extractor.GetFolders(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "executing get_folders AppleScript")

	mockExec.AssertExpectations(t)
}

func TestAppleScriptExtractor_GetAllNotes(t *testing.T) {
	mockExec := new(MockCommandExecutor)
	extractor := NewAppleScriptExtractor(mockExec, newTestLogger())

	output := "id1|||FIELD|||Note 1|||FIELD|||<p>body1</p>|||FIELD|||Monday, January 1, 2026 at 10:00:00 AM|||FIELD|||Monday, January 1, 2026 at 10:00:00 AM|||FIELD|||iCloud|||FIELD|||Notes|||FIELD|||false|||FIELD|||false|||FIELD||||||NOTE|||" +
		"id2|||FIELD|||Note 2|||FIELD|||<p>body2</p>|||FIELD|||Tuesday, January 2, 2026 at 11:00:00 AM|||FIELD|||Tuesday, January 2, 2026 at 11:00:00 AM|||FIELD|||Gmail|||FIELD|||Work|||FIELD|||false|||FIELD|||false|||FIELD||||||NOTE|||"

	mockExec.On("Execute", mock.Anything, "osascript", mock.Anything).
		Return(&shell.CommandResult{Stdout: output}, nil)

	notes, err := extractor.GetAllNotes(context.Background(), nil, nil)
	require.NoError(t, err)
	require.Len(t, notes, 2)

	assert.Equal(t, "Note 1", notes[0].Name)
	assert.Equal(t, "Note 2", notes[1].Name)

	mockExec.AssertExpectations(t)
}

func TestAppleScriptExtractor_GetAllNotes_FilterByAccount(t *testing.T) {
	mockExec := new(MockCommandExecutor)
	extractor := NewAppleScriptExtractor(mockExec, newTestLogger())

	output := "id1|||FIELD|||Note 1|||FIELD|||<p>body1</p>|||FIELD|||Monday, January 1, 2026 at 10:00:00 AM|||FIELD|||Monday, January 1, 2026 at 10:00:00 AM|||FIELD|||iCloud|||FIELD|||Notes|||FIELD|||false|||FIELD|||false|||FIELD||||||NOTE|||" +
		"id2|||FIELD|||Note 2|||FIELD|||<p>body2</p>|||FIELD|||Tuesday, January 2, 2026 at 11:00:00 AM|||FIELD|||Tuesday, January 2, 2026 at 11:00:00 AM|||FIELD|||Gmail|||FIELD|||Work|||FIELD|||false|||FIELD|||false|||FIELD||||||NOTE|||"

	mockExec.On("Execute", mock.Anything, "osascript", mock.Anything).
		Return(&shell.CommandResult{Stdout: output}, nil)

	notes, err := extractor.GetAllNotes(context.Background(), []string{"iCloud"}, nil)
	require.NoError(t, err)
	require.Len(t, notes, 1)
	assert.Equal(t, "Note 1", notes[0].Name)

	mockExec.AssertExpectations(t)
}

func TestAppleScriptExtractor_GetAllNotes_FilterByFolder(t *testing.T) {
	mockExec := new(MockCommandExecutor)
	extractor := NewAppleScriptExtractor(mockExec, newTestLogger())

	output := "id1|||FIELD|||Note 1|||FIELD|||<p>body1</p>|||FIELD|||Monday, January 1, 2026 at 10:00:00 AM|||FIELD|||Monday, January 1, 2026 at 10:00:00 AM|||FIELD|||iCloud|||FIELD|||Notes|||FIELD|||false|||FIELD|||false|||FIELD||||||NOTE|||" +
		"id2|||FIELD|||Note 2|||FIELD|||<p>body2</p>|||FIELD|||Tuesday, January 2, 2026 at 11:00:00 AM|||FIELD|||Tuesday, January 2, 2026 at 11:00:00 AM|||FIELD|||iCloud|||FIELD|||Work|||FIELD|||false|||FIELD|||false|||FIELD||||||NOTE|||"

	mockExec.On("Execute", mock.Anything, "osascript", mock.Anything).
		Return(&shell.CommandResult{Stdout: output}, nil)

	notes, err := extractor.GetAllNotes(context.Background(), nil, []string{"Work"})
	require.NoError(t, err)
	require.Len(t, notes, 1)
	assert.Equal(t, "Note 2", notes[0].Name)

	mockExec.AssertExpectations(t)
}

func TestAppleScriptExtractor_GetAllNotes_ExecutionError(t *testing.T) {
	mockExec := new(MockCommandExecutor)
	extractor := NewAppleScriptExtractor(mockExec, newTestLogger())

	mockExec.On("Execute", mock.Anything, "osascript", mock.Anything).
		Return(nil, assert.AnError)

	_, err := extractor.GetAllNotes(context.Background(), nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "executing get_all_notes AppleScript")

	mockExec.AssertExpectations(t)
}

func TestFilterNotes_NoFilters(t *testing.T) {
	notes := []model.Note{
		{Name: "A", Account: "iCloud", FolderPath: "Notes"},
		{Name: "B", Account: "Gmail", FolderPath: "Work"},
	}

	result := filterNotes(notes, nil, nil)
	assert.Len(t, result, 2)
}

func TestFilterNotes_AccountFilter(t *testing.T) {
	notes := []model.Note{
		{Name: "A", Account: "iCloud", FolderPath: "Notes"},
		{Name: "B", Account: "Gmail", FolderPath: "Work"},
	}

	result := filterNotes(notes, []string{"iCloud"}, nil)
	require.Len(t, result, 1)
	assert.Equal(t, "A", result[0].Name)
}

func TestFilterNotes_FolderFilter(t *testing.T) {
	notes := []model.Note{
		{Name: "A", Account: "iCloud", FolderPath: "Notes"},
		{Name: "B", Account: "iCloud", FolderPath: "Work"},
	}

	result := filterNotes(notes, nil, []string{"Work"})
	require.Len(t, result, 1)
	assert.Equal(t, "B", result[0].Name)
}

func TestFilterNotes_BothFilters(t *testing.T) {
	notes := []model.Note{
		{Name: "A", Account: "iCloud", FolderPath: "Notes"},
		{Name: "B", Account: "iCloud", FolderPath: "Work"},
		{Name: "C", Account: "Gmail", FolderPath: "Work"},
	}

	result := filterNotes(notes, []string{"iCloud"}, []string{"Work"})
	require.Len(t, result, 1)
	assert.Equal(t, "B", result[0].Name)
}

func TestToSet(t *testing.T) {
	assert.Nil(t, toSet(nil))
	assert.Nil(t, toSet([]string{}))

	s := toSet([]string{"a", "b"})
	assert.True(t, s["a"])
	assert.True(t, s["b"])
	assert.False(t, s["c"])
}

func TestBuildFileIndex(t *testing.T) {
	dir := t.TempDir()

	// Create some files in subdirectories.
	subDir := filepath.Join(dir, "sub1", "sub2")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "photo.jpg"), []byte("img1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "photo.jpg"), []byte("img2"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "doc.pdf"), []byte("pdf"), 0644))

	index, err := buildFileIndex(context.Background(), dir)
	require.NoError(t, err)

	assert.Len(t, index["photo.jpg"], 2)
	assert.Len(t, index["doc.pdf"], 1)
	assert.Empty(t, index["missing.txt"])
}

func TestBuildFileIndex_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	index, err := buildFileIndex(context.Background(), dir)
	require.NoError(t, err)
	assert.Empty(t, index)
}

func TestResolveAttachmentsFromDir(t *testing.T) {
	// Create a fake Notes media directory.
	dir := t.TempDir()
	subDir := filepath.Join(dir, "uuid-123")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "photo.jpg"), []byte("fake-image"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "huge.bin"), make([]byte, 2*1024*1024), 0644))

	mockExec := new(MockCommandExecutor)
	extractor := NewAppleScriptExtractor(mockExec, newTestLogger())

	notes := []model.Note{
		{
			Name: "Test Note",
			Attachments: []model.Attachment{
				{Name: "photo.jpg", ContentID: "cid-1"},
				{Name: "huge.bin", ContentID: "cid-2"},
				{Name: "missing.png", ContentID: "cid-3"},
			},
		},
	}

	// Use 1 MB max to test size filtering.
	err := extractor.resolveAttachmentsFromDir(context.Background(), dir, notes, 1)
	require.NoError(t, err)

	// photo.jpg should be resolved (< 1 MB).
	assert.Equal(t, []byte("fake-image"), notes[0].Attachments[0].Data)
	// huge.bin should be skipped (> 1 MB).
	assert.Nil(t, notes[0].Attachments[1].Data)
	// missing.png should be nil.
	assert.Nil(t, notes[0].Attachments[2].Data)
}

func TestResolveAttachmentsFromDir_ContentIDMatch(t *testing.T) {
	// Create two files with the same name in different dirs.
	dir := t.TempDir()
	dir1 := filepath.Join(dir, "uuid-AAA")
	dir2 := filepath.Join(dir, "uuid-BBB")
	require.NoError(t, os.MkdirAll(dir1, 0755))
	require.NoError(t, os.MkdirAll(dir2, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir1, "image.png"), []byte("wrong"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "image.png"), []byte("correct"), 0644))

	mockExec := new(MockCommandExecutor)
	extractor := NewAppleScriptExtractor(mockExec, newTestLogger())

	notes := []model.Note{
		{
			Name: "Note",
			Attachments: []model.Attachment{
				{Name: "image.png", ContentID: "BBB"},
			},
		},
	}

	err := extractor.resolveAttachmentsFromDir(context.Background(), dir, notes, 50)
	require.NoError(t, err)

	// Should match the one containing "BBB" in its path.
	assert.Equal(t, []byte("correct"), notes[0].Attachments[0].Data)
}

func TestResolveAttachmentsFromDir_EmptyName(t *testing.T) {
	dir := t.TempDir()
	mockExec := new(MockCommandExecutor)
	extractor := NewAppleScriptExtractor(mockExec, newTestLogger())

	notes := []model.Note{
		{
			Name: "Note",
			Attachments: []model.Attachment{
				{Name: "", ContentID: "cid"},
			},
		},
	}

	err := extractor.resolveAttachmentsFromDir(context.Background(), dir, notes, 50)
	require.NoError(t, err)
	assert.Nil(t, notes[0].Attachments[0].Data)
}
