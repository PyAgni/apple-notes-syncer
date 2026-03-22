// Package model defines the core domain types used throughout apple-notes-sync.
package model

import "time"

// AttachmentType classifies the kind of attachment embedded in a note.
type AttachmentType string

const (
	// AttachmentLink represents a URL or hyperlink attachment.
	AttachmentLink AttachmentType = "link"
	// AttachmentImage represents an image attachment.
	AttachmentImage AttachmentType = "image"
	// AttachmentVideo represents a video attachment.
	AttachmentVideo AttachmentType = "video"
	// AttachmentFile represents a generic file attachment.
	AttachmentFile AttachmentType = "file"
)

// Attachment represents a file, image, video, or link embedded in a note.
type Attachment struct {
	// Type indicates the kind of attachment (link, image, video, file).
	Type AttachmentType
	// Name is the original filename or link text.
	Name string
	// URL is the hyperlink for link attachments or the file path for local files.
	URL string
	// MIMEType is the media type (e.g. "image/png").
	MIMEType string
	// Data holds the raw bytes of the attachment, if available. May be nil.
	Data []byte
}

// Note represents a single Apple Note extracted from the Notes app.
type Note struct {
	// ID is the Apple Notes internal identifier (e.g. "x-coredata://...").
	ID string
	// Name is the note title.
	Name string
	// BodyHTML is the raw HTML body as returned by AppleScript.
	BodyHTML string
	// BodyMarkdown is the converted Markdown body, populated after conversion.
	BodyMarkdown string
	// PlainText is the plain text content of the note.
	PlainText string
	// FolderPath is the slash-separated folder hierarchy (e.g. "Tech/Go Projects").
	FolderPath string
	// Account is the Notes account name (e.g. "iCloud").
	Account string
	// CreatedAt is when the note was created.
	CreatedAt time.Time
	// ModifiedAt is when the note was last modified.
	ModifiedAt time.Time
	// Shared indicates whether the note is shared with others.
	Shared bool
	// Protected indicates whether the note is password-protected.
	// Protected notes typically have an empty body.
	Protected bool
	// Attachments holds all files, images, videos, and links embedded in the note.
	Attachments []Attachment
}

// Folder represents a folder in the Apple Notes hierarchy.
type Folder struct {
	// ID is the Apple Notes internal folder identifier.
	ID string
	// Name is the folder display name.
	Name string
	// Account is the Notes account this folder belongs to.
	Account string
	// Path is the full slash-separated path (e.g. "Study/Go").
	Path string
}

// SyncResult summarizes the outcome of a sync run.
type SyncResult struct {
	// TotalNotes is the number of notes found in Apple Notes.
	TotalNotes int
	// WrittenNotes is the number of notes successfully written to disk.
	WrittenNotes int
	// SkippedNotes is the number of notes skipped (filtered, protected, etc.).
	SkippedNotes int
	// Errors collects non-fatal errors encountered during the sync.
	Errors []error
	// GitCommitHash is the hash of the git commit created, if any.
	GitCommitHash string
	// RcloneSynced indicates whether rclone sync was performed.
	RcloneSynced bool
	// Duration is how long the entire sync took.
	Duration time.Duration
}

// SanitizedFileName returns a filesystem-safe version of the note name
// suitable for use as a markdown filename (without extension).
func (n *Note) SanitizedFileName() string {
	name := n.Name
	if name == "" {
		name = "untitled"
	}

	// Replace characters that are problematic on filesystems.
	var sanitized []rune
	for _, r := range name {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			sanitized = append(sanitized, '-')
		default:
			sanitized = append(sanitized, r)
		}
	}

	result := string(sanitized)

	// Trim trailing dots and spaces (problematic on Windows).
	for len(result) > 0 && (result[len(result)-1] == '.' || result[len(result)-1] == ' ') {
		result = result[:len(result)-1]
	}

	if result == "" {
		result = "untitled"
	}

	// Limit length to 200 characters to avoid filesystem limits.
	if len(result) > 200 {
		result = result[:200]
	}

	return result
}
