package applescript

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PyAgni/apple-notes-syncer/internal/model"
)

func TestParseNotesOutput_SingleNote(t *testing.T) {
	raw := "x-coredata://123|||FIELD|||My Note|||FIELD|||<div><h1>My Note</h1></div>|||FIELD|||Monday, March 18, 2026 at 4:39:41 PM|||FIELD|||Monday, March 18, 2026 at 4:40:05 PM|||FIELD|||iCloud|||FIELD|||Notes|||FIELD|||false|||FIELD|||false|||FIELD||||||NOTE|||"

	notes, err := ParseNotesOutput(raw)
	require.NoError(t, err)
	require.Len(t, notes, 1)

	assert.Equal(t, "x-coredata://123", notes[0].ID)
	assert.Equal(t, "My Note", notes[0].Name)
	assert.Equal(t, "<div><h1>My Note</h1></div>", notes[0].BodyHTML)
	assert.Equal(t, "iCloud", notes[0].Account)
	assert.Equal(t, "Notes", notes[0].FolderPath)
	assert.False(t, notes[0].Protected)
	assert.False(t, notes[0].Shared)
	assert.Equal(t, 2026, notes[0].CreatedAt.Year())
	assert.Equal(t, 2026, notes[0].ModifiedAt.Year())
	assert.Empty(t, notes[0].Attachments)
}

func TestParseNotesOutput_MultipleNotes(t *testing.T) {
	raw := "id1|||FIELD|||Note 1|||FIELD|||<p>body1</p>|||FIELD|||Monday, January 1, 2026 at 10:00:00 AM|||FIELD|||Monday, January 1, 2026 at 10:00:00 AM|||FIELD|||iCloud|||FIELD|||Work|||FIELD|||false|||FIELD|||false|||FIELD||||||NOTE|||" +
		"id2|||FIELD|||Note 2|||FIELD|||<p>body2</p>|||FIELD|||Tuesday, January 2, 2026 at 11:00:00 AM|||FIELD|||Tuesday, January 2, 2026 at 11:00:00 AM|||FIELD|||Gmail|||FIELD|||Personal|||FIELD|||true|||FIELD|||true|||FIELD||||||NOTE|||"

	notes, err := ParseNotesOutput(raw)
	require.NoError(t, err)
	require.Len(t, notes, 2)

	assert.Equal(t, "Note 1", notes[0].Name)
	assert.Equal(t, "iCloud", notes[0].Account)
	assert.False(t, notes[0].Protected)

	assert.Equal(t, "Note 2", notes[1].Name)
	assert.Equal(t, "Gmail", notes[1].Account)
	assert.True(t, notes[1].Protected)
	assert.True(t, notes[1].Shared)
}

func TestParseNotesOutput_EmptyInput(t *testing.T) {
	notes, err := ParseNotesOutput("")
	require.NoError(t, err)
	assert.Nil(t, notes)
}

func TestParseNotesOutput_WhitespaceOnly(t *testing.T) {
	notes, err := ParseNotesOutput("   \n  \t  ")
	require.NoError(t, err)
	assert.Nil(t, notes)
}

func TestParseNotesOutput_InvalidFieldCount(t *testing.T) {
	raw := "id|||FIELD|||name|||FIELD|||body|||NOTE|||"
	_, err := ParseNotesOutput(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected 10 fields")
}

func TestParseNotesOutput_InvalidDate(t *testing.T) {
	raw := "id1|||FIELD|||Note|||FIELD|||<p>body</p>|||FIELD|||not-a-date|||FIELD|||Monday, January 1, 2026 at 10:00:00 AM|||FIELD|||iCloud|||FIELD|||Notes|||FIELD|||false|||FIELD|||false|||FIELD||||||NOTE|||"
	_, err := ParseNotesOutput(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing creation date")
}

func TestParseNotesOutput_ProtectedNote(t *testing.T) {
	raw := "id1|||FIELD|||Secret|||FIELD||||||FIELD|||Monday, January 1, 2026 at 10:00:00 AM|||FIELD|||Monday, January 1, 2026 at 10:00:00 AM|||FIELD|||iCloud|||FIELD|||Private|||FIELD|||true|||FIELD|||false|||FIELD||||||NOTE|||"

	notes, err := ParseNotesOutput(raw)
	require.NoError(t, err)
	require.Len(t, notes, 1)

	assert.Equal(t, "Secret", notes[0].Name)
	assert.Empty(t, notes[0].BodyHTML)
	assert.True(t, notes[0].Protected)
}

func TestParseNotesOutput_NestedFolderPath(t *testing.T) {
	raw := "id1|||FIELD|||Note|||FIELD|||<p>body</p>|||FIELD|||Monday, January 1, 2026 at 10:00:00 AM|||FIELD|||Monday, January 1, 2026 at 10:00:00 AM|||FIELD|||iCloud|||FIELD|||Work/Projects/Go|||FIELD|||false|||FIELD|||false|||FIELD||||||NOTE|||"

	notes, err := ParseNotesOutput(raw)
	require.NoError(t, err)
	require.Len(t, notes, 1)

	assert.Equal(t, "Work/Projects/Go", notes[0].FolderPath)
}

func TestParseFoldersOutput_SingleFolder(t *testing.T) {
	raw := "iCloud|||FIELD|||Notes|||FIELD|||folder-id-1|||FIELD|||Notes|||FOLDER|||"

	folders, err := ParseFoldersOutput(raw)
	require.NoError(t, err)
	require.Len(t, folders, 1)

	assert.Equal(t, "iCloud", folders[0].Account)
	assert.Equal(t, "Notes", folders[0].Name)
	assert.Equal(t, "folder-id-1", folders[0].ID)
	assert.Equal(t, "Notes", folders[0].Path)
}

func TestParseFoldersOutput_MultipleFolders(t *testing.T) {
	raw := "iCloud|||FIELD|||Notes|||FIELD|||id1|||FIELD|||Notes|||FOLDER|||" +
		"iCloud|||FIELD|||Work|||FIELD|||id2|||FIELD|||Work|||FOLDER|||" +
		"Gmail|||FIELD|||Personal|||FIELD|||id3|||FIELD|||Personal|||FOLDER|||"

	folders, err := ParseFoldersOutput(raw)
	require.NoError(t, err)
	require.Len(t, folders, 3)

	assert.Equal(t, "Work", folders[1].Name)
	assert.Equal(t, "Gmail", folders[2].Account)
}

func TestParseFoldersOutput_NestedFolder(t *testing.T) {
	raw := "iCloud|||FIELD|||Go|||FIELD|||id1|||FIELD|||Work/Projects/Go|||FOLDER|||"

	folders, err := ParseFoldersOutput(raw)
	require.NoError(t, err)
	require.Len(t, folders, 1)

	assert.Equal(t, "Go", folders[0].Name)
	assert.Equal(t, "Work/Projects/Go", folders[0].Path)
}

func TestParseFoldersOutput_EmptyInput(t *testing.T) {
	folders, err := ParseFoldersOutput("")
	require.NoError(t, err)
	assert.Nil(t, folders)
}

func TestParseFoldersOutput_InvalidFieldCount(t *testing.T) {
	raw := "iCloud|||FIELD|||Notes|||FOLDER|||"
	_, err := ParseFoldersOutput(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected 4 fields")
}

func TestParseAppleScriptDate_USFormat(t *testing.T) {
	d, err := ParseAppleScriptDate("Monday, March 18, 2026 at 4:39:41 PM")
	require.NoError(t, err)
	assert.Equal(t, 2026, d.Year())
	assert.Equal(t, 3, int(d.Month()))
	assert.Equal(t, 18, d.Day())
}

func TestParseAppleScriptDate_WithDatePrefix(t *testing.T) {
	d, err := ParseAppleScriptDate("date Monday, March 18, 2026 at 4:39:41 PM")
	require.NoError(t, err)
	assert.Equal(t, 2026, d.Year())
}

func TestParseAppleScriptDate_24HourFormat(t *testing.T) {
	d, err := ParseAppleScriptDate("Monday, March 18, 2026 at 16:39:41")
	require.NoError(t, err)
	assert.Equal(t, 16, d.Hour())
}

func TestParseAppleScriptDate_ShortFormat(t *testing.T) {
	d, err := ParseAppleScriptDate("3/18/2026, 4:39:41 PM")
	require.NoError(t, err)
	assert.Equal(t, 2026, d.Year())
	assert.Equal(t, 3, int(d.Month()))
}

func TestParseAppleScriptDate_WithoutDayOfWeek(t *testing.T) {
	d, err := ParseAppleScriptDate("March 18, 2026 at 4:39:41 PM")
	require.NoError(t, err)
	assert.Equal(t, 18, d.Day())
}

func TestParseAppleScriptDate_UKIndianLocale(t *testing.T) {
	d, err := ParseAppleScriptDate("Wednesday, 18 March 2026 at 4:39:41 PM")
	require.NoError(t, err)
	assert.Equal(t, 2026, d.Year())
	assert.Equal(t, 3, int(d.Month()))
	assert.Equal(t, 18, d.Day())
	assert.Equal(t, 16, d.Hour())
}

func TestParseAppleScriptDate_NarrowNoBreakSpace(t *testing.T) {
	// macOS inserts \u202f (narrow no-break space) before AM/PM.
	d, err := ParseAppleScriptDate("Wednesday, 18 March 2026 at 4:39:41\u202fPM")
	require.NoError(t, err)
	assert.Equal(t, 2026, d.Year())
	assert.Equal(t, 16, d.Hour())
}

func TestNormalizeWhitespace(t *testing.T) {
	assert.Equal(t, "4:39 PM", normalizeWhitespace("4:39\u202fPM"))
	assert.Equal(t, "no change", normalizeWhitespace("no change"))
	assert.Equal(t, "a b", normalizeWhitespace("a\u00a0b"))
}

func TestParseAppleScriptDate_InvalidFormat(t *testing.T) {
	_, err := ParseAppleScriptDate("not a date at all")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no matching format found")
}

func TestParseNotesOutput_WithAttachments(t *testing.T) {
	raw := "id1|||FIELD|||Note|||FIELD|||<p>body</p>|||FIELD|||Monday, January 1, 2026 at 10:00:00 AM|||FIELD|||Monday, January 1, 2026 at 10:00:00 AM|||FIELD|||iCloud|||FIELD|||Notes|||FIELD|||false|||FIELD|||false|||FIELD|||photo.jpg|||AFIELD|||ABC-123|||ATTACH|||doc.pdf|||AFIELD|||DEF-456|||NOTE|||"

	notes, err := ParseNotesOutput(raw)
	require.NoError(t, err)
	require.Len(t, notes, 1)
	require.Len(t, notes[0].Attachments, 2)

	assert.Equal(t, "photo.jpg", notes[0].Attachments[0].Name)
	assert.Equal(t, "ABC-123", notes[0].Attachments[0].ContentID)
	assert.Equal(t, model.AttachmentImage, notes[0].Attachments[0].Type)

	assert.Equal(t, "doc.pdf", notes[0].Attachments[1].Name)
	assert.Equal(t, "DEF-456", notes[0].Attachments[1].ContentID)
	assert.Equal(t, model.AttachmentFile, notes[0].Attachments[1].Type)
}

func TestParseAttachments_Empty(t *testing.T) {
	result := parseAttachments("")
	assert.Nil(t, result)
}

func TestParseAttachments_Single(t *testing.T) {
	result := parseAttachments("image.png|||AFIELD|||CID-123")
	require.Len(t, result, 1)
	assert.Equal(t, "image.png", result[0].Name)
	assert.Equal(t, "CID-123", result[0].ContentID)
	assert.Equal(t, model.AttachmentImage, result[0].Type)
}

func TestParseAttachments_Multiple(t *testing.T) {
	result := parseAttachments("a.jpg|||AFIELD|||cid1|||ATTACH|||b.mov|||AFIELD|||cid2")
	require.Len(t, result, 2)
	assert.Equal(t, model.AttachmentImage, result[0].Type)
	assert.Equal(t, model.AttachmentVideo, result[1].Type)
}

func TestInferAttachmentType(t *testing.T) {
	tests := []struct {
		name     string
		expected model.AttachmentType
	}{
		{"photo.jpg", model.AttachmentImage},
		{"PHOTO.JPEG", model.AttachmentImage},
		{"image.png", model.AttachmentImage},
		{"pic.heic", model.AttachmentImage},
		{"video.mp4", model.AttachmentVideo},
		{"clip.mov", model.AttachmentVideo},
		{"document.pdf", model.AttachmentFile},
		{"notes.txt", model.AttachmentFile},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, inferAttachmentType(tt.name), tt.name)
	}
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "hello", truncate("hello", 10))
	assert.Equal(t, "hel...", truncate("hello world", 3))
	assert.Equal(t, "", truncate("", 5))
}
