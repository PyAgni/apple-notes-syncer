// Package applescript handles extracting notes and folders from Apple Notes
// via AppleScript executed through osascript.
package applescript

import (
	"fmt"
	"strings"
	"time"

	"github.com/PyAgni/apple-notes-syncer/internal/model"
)

const (
	// fieldDelimiter separates fields within a record in AppleScript output.
	fieldDelimiter = "|||FIELD|||"
	// noteDelimiter separates note records in AppleScript output.
	noteDelimiter = "|||NOTE|||"
	// folderDelimiter separates folder records in AppleScript output.
	folderDelimiter = "|||FOLDER|||"
)

// noteFieldCount is the number of fields expected per note record.
const noteFieldCount = 9

// folderFieldCount is the number of fields expected per folder record.
const folderFieldCount = 4

// ParseNotesOutput parses the raw osascript output into Note structs.
// The output is expected to use fieldDelimiter between fields and
// noteDelimiter between records.
func ParseNotesOutput(raw string) ([]model.Note, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	records := strings.Split(raw, noteDelimiter)
	var notes []model.Note

	for _, record := range records {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}

		fields := strings.Split(record, fieldDelimiter)
		if len(fields) != noteFieldCount {
			return nil, fmt.Errorf("expected %d fields per note, got %d in record: %q",
				noteFieldCount, len(fields), truncate(record, 100))
		}

		createdAt, err := ParseAppleScriptDate(strings.TrimSpace(fields[3]))
		if err != nil {
			return nil, fmt.Errorf("parsing creation date for note %q: %w", fields[1], err)
		}

		modifiedAt, err := ParseAppleScriptDate(strings.TrimSpace(fields[4]))
		if err != nil {
			return nil, fmt.Errorf("parsing modification date for note %q: %w", fields[1], err)
		}

		note := model.Note{
			ID:         strings.TrimSpace(fields[0]),
			Name:       strings.TrimSpace(fields[1]),
			BodyHTML:   strings.TrimSpace(fields[2]),
			FolderPath: strings.TrimSpace(fields[6]),
			Account:    strings.TrimSpace(fields[5]),
			CreatedAt:  createdAt,
			ModifiedAt: modifiedAt,
			Protected:  strings.TrimSpace(fields[7]) == "true",
			Shared:     strings.TrimSpace(fields[8]) == "true",
		}

		notes = append(notes, note)
	}

	return notes, nil
}

// ParseFoldersOutput parses the raw osascript output into Folder structs.
// The output is expected to use fieldDelimiter between fields and
// folderDelimiter between records.
func ParseFoldersOutput(raw string) ([]model.Folder, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	records := strings.Split(raw, folderDelimiter)
	var folders []model.Folder

	for _, record := range records {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}

		fields := strings.Split(record, fieldDelimiter)
		if len(fields) != folderFieldCount {
			return nil, fmt.Errorf("expected %d fields per folder, got %d in record: %q",
				folderFieldCount, len(fields), truncate(record, 100))
		}

		folder := model.Folder{
			Account: strings.TrimSpace(fields[0]),
			Name:    strings.TrimSpace(fields[1]),
			ID:      strings.TrimSpace(fields[2]),
			Path:    strings.TrimSpace(fields[3]),
		}

		folders = append(folders, folder)
	}

	return folders, nil
}

// appleScriptDateFormats lists the date formats AppleScript may produce.
// The exact format depends on the user's locale settings.
var appleScriptDateFormats = []string{
	// Common US format: "Monday, January 2, 2006 at 3:04:05 PM"
	"Monday, January 2, 2006 at 3:04:05 PM",
	// Without day of week: "January 2, 2006 at 3:04:05 PM"
	"January 2, 2006 at 3:04:05 PM",
	// 24-hour format: "Monday, January 2, 2006 at 15:04:05"
	"Monday, January 2, 2006 at 15:04:05",
	// ISO-ish format some locales produce.
	"2006-01-02 15:04:05 -0700",
	// Short format: "1/2/2006, 3:04:05 PM"
	"1/2/2006, 3:04:05 PM",
	// Another common variant: "2 January 2006 at 15:04:05"
	"2 January 2006 at 15:04:05",
	// UK/Indian locale: "Monday, 2 January 2006 at 3:04:05 PM"
	"Monday, 2 January 2006 at 3:04:05 PM",
	// UK/Indian without day of week: "2 January 2006 at 3:04:05 PM"
	"2 January 2006 at 3:04:05 PM",
	// Date-only formats as fallback.
	"Monday, January 2, 2006",
	"January 2, 2006",
	"Monday, 2 January 2006",
	"2 January 2006",
}

// ParseAppleScriptDate attempts to parse a date string from AppleScript output.
// AppleScript's date format depends on the user's macOS locale, so multiple
// formats are attempted.
func ParseAppleScriptDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	// Strip "date " prefix that AppleScript sometimes includes.
	s = strings.TrimPrefix(s, "date ")
	// Normalize Unicode whitespace characters (e.g. narrow no-break space \u202f)
	// that macOS inserts before AM/PM in some locales.
	s = normalizeWhitespace(s)

	for _, format := range appleScriptDateFormats {
		t, err := time.Parse(format, s)
		if err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date %q: no matching format found", s)
}

// normalizeWhitespace replaces Unicode whitespace characters (such as the
// narrow no-break space \u202f that macOS inserts before AM/PM) with regular spaces.
func normalizeWhitespace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\u202f', '\u00a0', '\u2009', '\u200a':
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// truncate shortens a string to maxLen characters for display in error messages.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
