package syncer

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/PyAgni/apple-notes-syncer/internal/config"
	"github.com/PyAgni/apple-notes-syncer/internal/model"
)

// --- Mock implementations ---

type mockExtractor struct{ mock.Mock }

func (m *mockExtractor) GetFolders(ctx context.Context) ([]model.Folder, error) {
	args := m.Called(ctx)
	return args.Get(0).([]model.Folder), args.Error(1)
}

func (m *mockExtractor) GetAllNotes(ctx context.Context, accounts []string, folders []string) ([]model.Note, error) {
	args := m.Called(ctx, accounts, folders)
	return args.Get(0).([]model.Note), args.Error(1)
}

func (m *mockExtractor) ResolveAttachments(ctx context.Context, notes []model.Note, maxSizeMB int) error {
	args := m.Called(ctx, notes, maxSizeMB)
	return args.Error(0)
}

type mockConverter struct{ mock.Mock }

func (m *mockConverter) Convert(html string) (string, error) {
	args := m.Called(html)
	return args.String(0), args.Error(1)
}

type mockWriter struct{ mock.Mock }

func (m *mockWriter) WriteNote(ctx context.Context, note *model.Note) (string, error) {
	args := m.Called(ctx, note)
	return args.String(0), args.Error(1)
}

func (m *mockWriter) WriteAll(ctx context.Context, notes []model.Note) ([]string, error) {
	args := m.Called(ctx, notes)
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockWriter) CleanOrphanedFiles(ctx context.Context, currentNotePaths []string) ([]string, error) {
	args := m.Called(ctx, currentNotePaths)
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockWriter) SaveAttachment(ctx context.Context, notePath string, attachment *model.Attachment) (string, error) {
	args := m.Called(ctx, notePath, attachment)
	return args.String(0), args.Error(1)
}

func (m *mockWriter) NoteRelPath(note *model.Note) string {
	args := m.Called(note)
	return args.String(0)
}

type mockGit struct{ mock.Mock }

func (m *mockGit) Init(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

func (m *mockGit) AddAll(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

func (m *mockGit) HasChanges(ctx context.Context) (bool, error) {
	args := m.Called(ctx)
	return args.Bool(0), args.Error(1)
}

func (m *mockGit) Commit(ctx context.Context, message string) (string, error) {
	args := m.Called(ctx, message)
	return args.String(0), args.Error(1)
}

func (m *mockGit) Push(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

type mockRclone struct{ mock.Mock }

func (m *mockRclone) Sync(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

func (m *mockRclone) IsAvailable(ctx context.Context) (bool, error) {
	args := m.Called(ctx)
	return args.Bool(0), args.Error(1)
}

// --- Helper ---

func defaultConfig() *config.Config {
	return &config.Config{
		RepoPath:       "/tmp/notes",
		CommitTemplate: config.DefaultCommitTemplate,
		FrontMatter:    true,
		CleanOrphans:   true,
		Timeout:        120 * time.Second,
		Git: config.GitConfig{
			Enabled: true,
			Remote:  "origin",
			Branch:  "main",
			Push:    true,
		},
		Rclone: config.RcloneConfig{Enabled: false},
		Filter: config.FilterConfig{
			ExcludeFolders: []string{"Recently Deleted"},
			SkipProtected:  true,
		},
		Attachments: config.AttachmentConfig{
			Enabled:   true,
			MaxSizeMB: 50,
			Dir:       "_attachments",
		},
	}
}

func testNotes() []model.Note {
	return []model.Note{
		{
			ID: "1", Name: "Note 1", BodyHTML: "<p>body1</p>",
			FolderPath: "Notes", Account: "iCloud",
			CreatedAt: time.Now(), ModifiedAt: time.Now(),
		},
		{
			ID: "2", Name: "Note 2", BodyHTML: "<p>body2</p>",
			FolderPath: "Work", Account: "iCloud",
			CreatedAt: time.Now(), ModifiedAt: time.Now(),
		},
	}
}

// --- Tests ---

func TestSyncer_Sync_FullPipeline(t *testing.T) {
	cfg := defaultConfig()
	ext := new(mockExtractor)
	conv := new(mockConverter)
	wr := new(mockWriter)
	git := new(mockGit)
	rc := new(mockRclone)

	s := NewSyncer(cfg, ext, conv, wr, git, rc, zap.NewNop())

	notes := testNotes()

	ext.On("GetAllNotes", mock.Anything, []string(nil), []string(nil)).Return(notes, nil)
	ext.On("ResolveAttachments", mock.Anything, mock.Anything, 50).Return(nil)
	conv.On("Convert", "<p>body1</p>").Return("body1\n", nil)
	conv.On("Convert", "<p>body2</p>").Return("body2\n", nil)
	wr.On("WriteAll", mock.Anything, mock.Anything).Return([]string{"Notes/Note 1.md", "Work/Note 2.md"}, nil)
	wr.On("CleanOrphanedFiles", mock.Anything, mock.Anything).Return([]string{}, nil)
	git.On("Init", mock.Anything).Return(nil)
	git.On("AddAll", mock.Anything).Return(nil)
	git.On("HasChanges", mock.Anything).Return(true, nil)
	git.On("Commit", mock.Anything, mock.Anything).Return("abc1234", nil)
	git.On("Push", mock.Anything).Return(nil)

	result, err := s.Sync(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 2, result.TotalNotes)
	assert.Equal(t, 2, result.WrittenNotes)
	assert.Equal(t, "abc1234", result.GitCommitHash)
	assert.Empty(t, result.Errors)

	ext.AssertExpectations(t)
	conv.AssertExpectations(t)
	wr.AssertExpectations(t)
	git.AssertExpectations(t)
}

func TestSyncer_Sync_DryRun(t *testing.T) {
	cfg := defaultConfig()
	cfg.DryRun = true
	ext := new(mockExtractor)
	conv := new(mockConverter)
	wr := new(mockWriter)
	git := new(mockGit)
	rc := new(mockRclone)

	s := NewSyncer(cfg, ext, conv, wr, git, rc, zap.NewNop())

	notes := testNotes()
	ext.On("GetAllNotes", mock.Anything, []string(nil), []string(nil)).Return(notes, nil)
	ext.On("ResolveAttachments", mock.Anything, mock.Anything, 50).Return(nil)
	conv.On("Convert", mock.Anything).Return("markdown\n", nil)

	result, err := s.Sync(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 2, result.WrittenNotes)

	// WriteAll, git, rclone should NOT be called in dry run.
	wr.AssertNotCalled(t, "WriteAll")
	git.AssertNotCalled(t, "Init")
}

func TestSyncer_Sync_NoChanges(t *testing.T) {
	cfg := defaultConfig()
	ext := new(mockExtractor)
	conv := new(mockConverter)
	wr := new(mockWriter)
	git := new(mockGit)
	rc := new(mockRclone)

	s := NewSyncer(cfg, ext, conv, wr, git, rc, zap.NewNop())

	notes := testNotes()
	ext.On("GetAllNotes", mock.Anything, mock.Anything, mock.Anything).Return(notes, nil)
	ext.On("ResolveAttachments", mock.Anything, mock.Anything, 50).Return(nil)
	conv.On("Convert", mock.Anything).Return("md\n", nil)
	wr.On("WriteAll", mock.Anything, mock.Anything).Return([]string{"a.md", "b.md"}, nil)
	wr.On("CleanOrphanedFiles", mock.Anything, mock.Anything).Return([]string{}, nil)
	git.On("Init", mock.Anything).Return(nil)
	git.On("AddAll", mock.Anything).Return(nil)
	git.On("HasChanges", mock.Anything).Return(false, nil)

	result, err := s.Sync(context.Background())
	require.NoError(t, err)

	assert.Empty(t, result.GitCommitHash)
	git.AssertNotCalled(t, "Commit")
	git.AssertNotCalled(t, "Push")
}

func TestSyncer_Sync_GitDisabled(t *testing.T) {
	cfg := defaultConfig()
	cfg.Git.Enabled = false
	ext := new(mockExtractor)
	conv := new(mockConverter)
	wr := new(mockWriter)
	git := new(mockGit)
	rc := new(mockRclone)

	s := NewSyncer(cfg, ext, conv, wr, git, rc, zap.NewNop())

	ext.On("GetAllNotes", mock.Anything, mock.Anything, mock.Anything).Return(testNotes(), nil)
	ext.On("ResolveAttachments", mock.Anything, mock.Anything, 50).Return(nil)
	conv.On("Convert", mock.Anything).Return("md\n", nil)
	wr.On("WriteAll", mock.Anything, mock.Anything).Return([]string{"a.md"}, nil)
	wr.On("CleanOrphanedFiles", mock.Anything, mock.Anything).Return([]string{}, nil)

	result, err := s.Sync(context.Background())
	require.NoError(t, err)

	assert.Empty(t, result.GitCommitHash)
	git.AssertNotCalled(t, "Init")
}

func TestSyncer_Sync_WithRclone(t *testing.T) {
	cfg := defaultConfig()
	cfg.Rclone.Enabled = true
	cfg.Rclone.RemoteName = "gdrive"
	cfg.Git.Enabled = false
	ext := new(mockExtractor)
	conv := new(mockConverter)
	wr := new(mockWriter)
	git := new(mockGit)
	rc := new(mockRclone)

	s := NewSyncer(cfg, ext, conv, wr, git, rc, zap.NewNop())

	ext.On("GetAllNotes", mock.Anything, mock.Anything, mock.Anything).Return(testNotes(), nil)
	ext.On("ResolveAttachments", mock.Anything, mock.Anything, 50).Return(nil)
	conv.On("Convert", mock.Anything).Return("md\n", nil)
	wr.On("WriteAll", mock.Anything, mock.Anything).Return([]string{"a.md"}, nil)
	wr.On("CleanOrphanedFiles", mock.Anything, mock.Anything).Return([]string{}, nil)
	rc.On("IsAvailable", mock.Anything).Return(true, nil)
	rc.On("Sync", mock.Anything).Return(nil)

	result, err := s.Sync(context.Background())
	require.NoError(t, err)
	assert.True(t, result.RcloneSynced)

	rc.AssertExpectations(t)
}

func TestSyncer_Sync_ExtractError(t *testing.T) {
	cfg := defaultConfig()
	ext := new(mockExtractor)
	conv := new(mockConverter)
	wr := new(mockWriter)
	git := new(mockGit)
	rc := new(mockRclone)

	s := NewSyncer(cfg, ext, conv, wr, git, rc, zap.NewNop())

	ext.On("GetAllNotes", mock.Anything, mock.Anything, mock.Anything).Return([]model.Note(nil), assert.AnError)

	_, err := s.Sync(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "extracting notes")
}

func TestSyncer_ApplyFilters_ExcludeFolders(t *testing.T) {
	cfg := defaultConfig()
	cfg.Filter.ExcludeFolders = []string{"Recently Deleted", "Trash"}
	s := NewSyncer(cfg, nil, nil, nil, nil, nil, zap.NewNop())

	notes := []model.Note{
		{Name: "Keep", FolderPath: "Notes"},
		{Name: "Delete1", FolderPath: "Recently Deleted"},
		{Name: "Delete2", FolderPath: "Trash"},
	}

	filtered := s.applyFilters(notes)
	require.Len(t, filtered, 1)
	assert.Equal(t, "Keep", filtered[0].Name)
}

func TestSyncer_ApplyFilters_SkipProtected(t *testing.T) {
	cfg := defaultConfig()
	cfg.Filter.SkipProtected = true
	s := NewSyncer(cfg, nil, nil, nil, nil, nil, zap.NewNop())

	notes := []model.Note{
		{Name: "Public", Protected: false},
		{Name: "Secret", Protected: true},
	}

	filtered := s.applyFilters(notes)
	require.Len(t, filtered, 1)
	assert.Equal(t, "Public", filtered[0].Name)
}

func TestSyncer_ApplyFilters_SkipShared(t *testing.T) {
	cfg := defaultConfig()
	cfg.Filter.SkipShared = true
	s := NewSyncer(cfg, nil, nil, nil, nil, nil, zap.NewNop())

	notes := []model.Note{
		{Name: "Mine", Shared: false},
		{Name: "Shared", Shared: true},
	}

	filtered := s.applyFilters(notes)
	require.Len(t, filtered, 1)
	assert.Equal(t, "Mine", filtered[0].Name)
}

func TestSyncer_ApplyFilters_ExcludeAccounts(t *testing.T) {
	cfg := defaultConfig()
	cfg.Filter.ExcludeAccounts = []string{"Gmail"}
	s := NewSyncer(cfg, nil, nil, nil, nil, nil, zap.NewNop())

	notes := []model.Note{
		{Name: "Keep", Account: "iCloud"},
		{Name: "Skip", Account: "Gmail"},
	}

	filtered := s.applyFilters(notes)
	require.Len(t, filtered, 1)
	assert.Equal(t, "Keep", filtered[0].Name)
}

func TestSyncer_BuildCommitMessage(t *testing.T) {
	cfg := defaultConfig()
	s := NewSyncer(cfg, nil, nil, nil, nil, nil, zap.NewNop())

	result := &model.SyncResult{
		TotalNotes:   10,
		WrittenNotes: 8,
		SkippedNotes: 2,
	}

	msg, err := s.buildCommitMessage(result)
	require.NoError(t, err)
	assert.Contains(t, msg, "apple-notes-sync:")
	assert.Contains(t, msg, "8 notes synced")
}

func TestSyncer_Sync_PushDisabled(t *testing.T) {
	cfg := defaultConfig()
	cfg.Git.Push = false
	ext := new(mockExtractor)
	conv := new(mockConverter)
	wr := new(mockWriter)
	git := new(mockGit)
	rc := new(mockRclone)

	s := NewSyncer(cfg, ext, conv, wr, git, rc, zap.NewNop())

	ext.On("GetAllNotes", mock.Anything, mock.Anything, mock.Anything).Return(testNotes(), nil)
	ext.On("ResolveAttachments", mock.Anything, mock.Anything, 50).Return(nil)
	conv.On("Convert", mock.Anything).Return("md\n", nil)
	wr.On("WriteAll", mock.Anything, mock.Anything).Return([]string{"a.md"}, nil)
	wr.On("CleanOrphanedFiles", mock.Anything, mock.Anything).Return([]string{}, nil)
	git.On("Init", mock.Anything).Return(nil)
	git.On("AddAll", mock.Anything).Return(nil)
	git.On("HasChanges", mock.Anything).Return(true, nil)
	git.On("Commit", mock.Anything, mock.Anything).Return("def5678", nil)

	result, err := s.Sync(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "def5678", result.GitCommitHash)

	git.AssertNotCalled(t, "Push")
}

func TestSyncer_Sync_ConvertError_NonFatal(t *testing.T) {
	cfg := defaultConfig()
	ext := new(mockExtractor)
	conv := new(mockConverter)
	wr := new(mockWriter)
	git := new(mockGit)
	rc := new(mockRclone)

	s := NewSyncer(cfg, ext, conv, wr, git, rc, zap.NewNop())

	notes := []model.Note{
		{ID: "1", Name: "Good", BodyHTML: "<p>ok</p>", FolderPath: "Notes", Account: "iCloud"},
		{ID: "2", Name: "Bad", BodyHTML: "<p>broken</p>", FolderPath: "Notes", Account: "iCloud"},
	}

	ext.On("GetAllNotes", mock.Anything, mock.Anything, mock.Anything).Return(notes, nil)
	ext.On("ResolveAttachments", mock.Anything, mock.Anything, 50).Return(nil)
	conv.On("Convert", "<p>ok</p>").Return("ok\n", nil)
	conv.On("Convert", "<p>broken</p>").Return("", assert.AnError)
	wr.On("WriteAll", mock.Anything, mock.Anything).Return([]string{"Notes/Good.md", "Notes/Bad.md"}, nil)
	wr.On("CleanOrphanedFiles", mock.Anything, mock.Anything).Return([]string{}, nil)
	git.On("Init", mock.Anything).Return(nil)
	git.On("AddAll", mock.Anything).Return(nil)
	git.On("HasChanges", mock.Anything).Return(true, nil)
	git.On("Commit", mock.Anything, mock.Anything).Return("abc", nil)
	git.On("Push", mock.Anything).Return(nil)

	result, err := s.Sync(context.Background())
	require.NoError(t, err)

	// One conversion error is non-fatal.
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "converting note")
}

func TestSyncer_Sync_WithAttachments(t *testing.T) {
	cfg := defaultConfig()
	cfg.Git.Enabled = false
	ext := new(mockExtractor)
	conv := new(mockConverter)
	wr := new(mockWriter)
	git := new(mockGit)
	rc := new(mockRclone)

	s := NewSyncer(cfg, ext, conv, wr, git, rc, zap.NewNop())

	notes := []model.Note{
		{
			ID: "1", Name: "Note With Image", BodyHTML: "<p>body</p>",
			FolderPath: "Notes", Account: "iCloud",
			CreatedAt: time.Now(), ModifiedAt: time.Now(),
			Attachments: []model.Attachment{
				{Name: "photo.jpg", ContentID: "CID-123", Type: model.AttachmentImage, Data: []byte("fake-image-data")},
			},
		},
	}

	ext.On("GetAllNotes", mock.Anything, mock.Anything, mock.Anything).Return(notes, nil)
	ext.On("ResolveAttachments", mock.Anything, mock.Anything, 50).Return(nil)
	conv.On("Convert", mock.Anything).Return("![image](cid:CID-123)\n", nil)
	wr.On("NoteRelPath", mock.Anything).Return("Notes/Note With Image.md")
	wr.On("SaveAttachment", mock.Anything, "Notes/Note With Image.md", mock.Anything).Return("Notes/_attachments/photo.jpg", nil)
	wr.On("WriteAll", mock.Anything, mock.Anything).Return([]string{"Notes/Note With Image.md"}, nil)
	wr.On("CleanOrphanedFiles", mock.Anything, mock.Anything).Return([]string{}, nil)

	result, err := s.Sync(context.Background())
	require.NoError(t, err)
	assert.Empty(t, result.Errors)

	// Attachment should be saved before WriteAll.
	wr.AssertCalled(t, "SaveAttachment", mock.Anything, "Notes/Note With Image.md", mock.Anything)
}

func TestSyncer_Sync_AttachmentRewritesCidRefs(t *testing.T) {
	cfg := defaultConfig()
	cfg.Git.Enabled = false
	ext := new(mockExtractor)
	conv := new(mockConverter)
	wr := new(mockWriter)
	git := new(mockGit)
	rc := new(mockRclone)

	s := NewSyncer(cfg, ext, conv, wr, git, rc, zap.NewNop())

	notes := []model.Note{
		{
			ID: "1", Name: "Image Note", BodyHTML: "<p>body</p>",
			FolderPath: "Notes", Account: "iCloud",
			CreatedAt: time.Now(), ModifiedAt: time.Now(),
			Attachments: []model.Attachment{
				{Name: "pic.png", ContentID: "ABC-123", Data: []byte("png-data")},
			},
		},
	}

	ext.On("GetAllNotes", mock.Anything, mock.Anything, mock.Anything).Return(notes, nil)
	ext.On("ResolveAttachments", mock.Anything, mock.Anything, 50).Return(nil)
	conv.On("Convert", mock.Anything).Return("Some text\n\n![photo](cid:ABC-123)\n\nMore text\n", nil)
	wr.On("NoteRelPath", mock.Anything).Return("Notes/Image Note.md")
	wr.On("SaveAttachment", mock.Anything, "Notes/Image Note.md", mock.Anything).Return("Notes/_attachments/pic.png", nil)
	wr.On("WriteAll", mock.Anything, mock.MatchedBy(func(notes []model.Note) bool {
		// The cid: reference should have been rewritten to the attachment path.
		return strings.Contains(notes[0].BodyMarkdown, "_attachments/pic.png") &&
			!strings.Contains(notes[0].BodyMarkdown, "cid:")
	})).Return([]string{"Notes/Image Note.md"}, nil)
	wr.On("CleanOrphanedFiles", mock.Anything, mock.Anything).Return([]string{}, nil)

	result, err := s.Sync(context.Background())
	require.NoError(t, err)
	assert.Empty(t, result.Errors)
	wr.AssertExpectations(t)
}

func TestSyncer_Sync_AttachmentRewritesDataURI(t *testing.T) {
	cfg := defaultConfig()
	cfg.Git.Enabled = false
	ext := new(mockExtractor)
	conv := new(mockConverter)
	wr := new(mockWriter)
	git := new(mockGit)
	rc := new(mockRclone)

	s := NewSyncer(cfg, ext, conv, wr, git, rc, zap.NewNop())

	notes := []model.Note{
		{
			ID: "1", Name: "Data URI Note", BodyHTML: "<p>body</p>",
			FolderPath: "Work", Account: "iCloud",
			CreatedAt: time.Now(), ModifiedAt: time.Now(),
			Attachments: []model.Attachment{
				{Name: "screenshot.png", Data: []byte("png-data")},
			},
		},
	}

	ext.On("GetAllNotes", mock.Anything, mock.Anything, mock.Anything).Return(notes, nil)
	ext.On("ResolveAttachments", mock.Anything, mock.Anything, 50).Return(nil)
	conv.On("Convert", mock.Anything).Return("Text\n\n![](data:image/png;base64,aVeryLongBase64String)\n", nil)
	wr.On("NoteRelPath", mock.Anything).Return("Work/Data URI Note.md")
	wr.On("SaveAttachment", mock.Anything, "Work/Data URI Note.md", mock.Anything).Return("Work/_attachments/screenshot.png", nil)
	wr.On("WriteAll", mock.Anything, mock.MatchedBy(func(notes []model.Note) bool {
		return strings.Contains(notes[0].BodyMarkdown, "_attachments/screenshot.png") &&
			!strings.Contains(notes[0].BodyMarkdown, "data:image")
	})).Return([]string{"Work/Data URI Note.md"}, nil)
	wr.On("CleanOrphanedFiles", mock.Anything, mock.Anything).Return([]string{}, nil)

	result, err := s.Sync(context.Background())
	require.NoError(t, err)
	assert.Empty(t, result.Errors)
	wr.AssertExpectations(t)
}

func TestSyncer_Sync_AttachmentsDisabled(t *testing.T) {
	cfg := defaultConfig()
	cfg.Attachments.Enabled = false
	cfg.Git.Enabled = false
	ext := new(mockExtractor)
	conv := new(mockConverter)
	wr := new(mockWriter)
	git := new(mockGit)
	rc := new(mockRclone)

	s := NewSyncer(cfg, ext, conv, wr, git, rc, zap.NewNop())

	ext.On("GetAllNotes", mock.Anything, mock.Anything, mock.Anything).Return(testNotes(), nil)
	conv.On("Convert", mock.Anything).Return("md\n", nil)
	wr.On("WriteAll", mock.Anything, mock.Anything).Return([]string{"a.md", "b.md"}, nil)
	wr.On("CleanOrphanedFiles", mock.Anything, mock.Anything).Return([]string{}, nil)

	result, err := s.Sync(context.Background())
	require.NoError(t, err)
	assert.Empty(t, result.Errors)

	// ResolveAttachments and SaveAttachment should NOT be called when disabled.
	ext.AssertNotCalled(t, "ResolveAttachments")
	wr.AssertNotCalled(t, "SaveAttachment")
}
