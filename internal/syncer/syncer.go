// Package syncer orchestrates the full Apple Notes sync pipeline:
// extract notes → convert HTML to Markdown → write to disk → git commit/push → rclone sync.
package syncer

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"go.uber.org/zap"

	"github.com/PyAgni/apple-notes-syncer/internal/applescript"
	"github.com/PyAgni/apple-notes-syncer/internal/config"
	"github.com/PyAgni/apple-notes-syncer/internal/converter"
	"github.com/PyAgni/apple-notes-syncer/internal/filesystem"
	"github.com/PyAgni/apple-notes-syncer/internal/gitops"
	"github.com/PyAgni/apple-notes-syncer/internal/model"
	"github.com/PyAgni/apple-notes-syncer/internal/rclone"
)

// Syncer orchestrates the full Apple Notes sync pipeline.
type Syncer struct {
	cfg       *config.Config
	extractor applescript.NoteExtractor
	converter converter.MarkdownConverter
	writer    filesystem.NoteWriter
	git       gitops.GitClient
	rclone    rclone.Syncer
	logger    *zap.Logger
}

// NewSyncer creates a new Syncer with all required dependencies.
func NewSyncer(
	cfg *config.Config,
	extractor applescript.NoteExtractor,
	converter converter.MarkdownConverter,
	writer filesystem.NoteWriter,
	git gitops.GitClient,
	rclone rclone.Syncer,
	logger *zap.Logger,
) *Syncer {
	return &Syncer{
		cfg:       cfg,
		extractor: extractor,
		converter: converter,
		writer:    writer,
		git:       git,
		rclone:    rclone,
		logger:    logger,
	}
}

// commitTemplateData holds the data available to the commit message template.
type commitTemplateData struct {
	Timestamp string
	Written   int
	Total     int
	Skipped   int
}

// Sync executes the full sync pipeline and returns a summary of results.
func (s *Syncer) Sync(ctx context.Context) (*model.SyncResult, error) {
	start := time.Now()
	result := &model.SyncResult{}

	// Step 1: Extract notes from Apple Notes.
	s.logger.Info("extracting notes from Apple Notes")
	notes, err := s.extractor.GetAllNotes(ctx, s.cfg.Filter.Accounts, s.cfg.Filter.Folders)
	if err != nil {
		return result, fmt.Errorf("extracting notes: %w", err)
	}
	result.TotalNotes = len(notes)

	// Step 1.5: Resolve attachment file data from the Notes media directory.
	if s.cfg.Attachments.Enabled {
		s.logger.Info("resolving attachments")
		if err := s.extractor.ResolveAttachments(ctx, notes, s.cfg.Attachments.MaxSizeMB); err != nil {
			s.logger.Warn("failed to resolve attachments", zap.Error(err))
			result.Errors = append(result.Errors, fmt.Errorf("resolving attachments: %w", err))
		}
	}

	// Step 2: Apply filters (exclude folders, protected, shared).
	notes = s.applyFilters(notes)

	// Step 3: Convert HTML to Markdown.
	s.logger.Info("converting notes to markdown", zap.Int("count", len(notes)))
	for i := range notes {
		md, err := s.converter.Convert(notes[i].BodyHTML)
		if err != nil {
			s.logger.Warn("failed to convert note",
				zap.String("name", notes[i].Name),
				zap.Error(err),
			)
			result.Errors = append(result.Errors, fmt.Errorf("converting note %q: %w", notes[i].Name, err))
			result.SkippedNotes++
			continue
		}
		notes[i].BodyMarkdown = md
	}

	// Step 4: Write notes to disk.
	if s.cfg.DryRun {
		s.logger.Info("dry run: skipping file writes", zap.Int("notes", len(notes)))
		result.WrittenNotes = len(notes) - result.SkippedNotes
		result.Duration = time.Since(start)
		return result, nil
	}

	// Step 4a: Save attachments as separate files and rewrite markdown
	// references before writing the note files.
	if s.cfg.Attachments.Enabled {
		for i := range notes {
			s.saveAndRewriteAttachments(ctx, &notes[i], result)
		}
	}

	s.logger.Info("writing notes to disk")
	writtenPaths, err := s.writer.WriteAll(ctx, notes)
	if err != nil {
		return result, fmt.Errorf("writing notes to disk: %w", err)
	}
	result.WrittenNotes = len(writtenPaths)
	result.SkippedNotes = result.TotalNotes - result.WrittenNotes

	// Step 5: Clean orphaned files.
	if s.cfg.CleanOrphans {
		removed, err := s.writer.CleanOrphanedFiles(ctx, writtenPaths)
		if err != nil {
			s.logger.Warn("failed to clean orphaned files", zap.Error(err))
			result.Errors = append(result.Errors, fmt.Errorf("cleaning orphans: %w", err))
		} else if len(removed) > 0 {
			s.logger.Info("cleaned orphaned files", zap.Int("count", len(removed)))
		}
	}

	// Step 6: Git operations.
	if s.cfg.Git.Enabled {
		hash, err := s.gitSync(ctx, result)
		if err != nil {
			return result, fmt.Errorf("git sync: %w", err)
		}
		result.GitCommitHash = hash
	}

	// Step 7: Rclone sync.
	if s.cfg.Rclone.Enabled {
		if err := s.rcloneSync(ctx); err != nil {
			s.logger.Warn("rclone sync failed", zap.Error(err))
			result.Errors = append(result.Errors, fmt.Errorf("rclone sync: %w", err))
		} else {
			result.RcloneSynced = true
		}
	}

	result.Duration = time.Since(start)
	s.logger.Info("sync completed",
		zap.Int("written", result.WrittenNotes),
		zap.Int("skipped", result.SkippedNotes),
		zap.Duration("duration", result.Duration),
	)

	return result, nil
}

// applyFilters removes notes that match exclude criteria.
func (s *Syncer) applyFilters(notes []model.Note) []model.Note {
	excludeFolders := make(map[string]bool)
	for _, f := range s.cfg.Filter.ExcludeFolders {
		excludeFolders[f] = true
	}
	excludeAccounts := make(map[string]bool)
	for _, a := range s.cfg.Filter.ExcludeAccounts {
		excludeAccounts[a] = true
	}

	var filtered []model.Note
	for _, note := range notes {
		if excludeFolders[note.FolderPath] {
			s.logger.Debug("skipping note in excluded folder",
				zap.String("name", note.Name),
				zap.String("folder", note.FolderPath),
			)
			continue
		}
		if excludeAccounts[note.Account] {
			s.logger.Debug("skipping note in excluded account",
				zap.String("name", note.Name),
				zap.String("account", note.Account),
			)
			continue
		}
		if s.cfg.Filter.SkipProtected && note.Protected {
			s.logger.Debug("skipping protected note", zap.String("name", note.Name))
			continue
		}
		if s.cfg.Filter.SkipShared && note.Shared {
			s.logger.Debug("skipping shared note", zap.String("name", note.Name))
			continue
		}
		filtered = append(filtered, note)
	}

	return filtered
}

// gitSync performs git add, commit, and push operations.
func (s *Syncer) gitSync(ctx context.Context, result *model.SyncResult) (string, error) {
	if err := s.git.Init(ctx); err != nil {
		return "", fmt.Errorf("git init: %w", err)
	}

	if err := s.git.AddAll(ctx); err != nil {
		return "", fmt.Errorf("git add: %w", err)
	}

	hasChanges, err := s.git.HasChanges(ctx)
	if err != nil {
		return "", fmt.Errorf("checking changes: %w", err)
	}

	if !hasChanges {
		s.logger.Info("no changes to commit")
		return "", nil
	}

	msg, err := s.buildCommitMessage(result)
	if err != nil {
		return "", fmt.Errorf("building commit message: %w", err)
	}

	hash, err := s.git.Commit(ctx, msg)
	if err != nil {
		return "", fmt.Errorf("committing: %w", err)
	}

	if s.cfg.Git.Push {
		if err := s.git.Push(ctx); err != nil {
			return hash, fmt.Errorf("pushing: %w", err)
		}
	}

	return hash, nil
}

// buildCommitMessage renders the commit message template.
func (s *Syncer) buildCommitMessage(result *model.SyncResult) (string, error) {
	tmpl, err := template.New("commit").Parse(s.cfg.CommitTemplate)
	if err != nil {
		return "", fmt.Errorf("parsing commit template: %w", err)
	}

	data := commitTemplateData{
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Written:   result.WrittenNotes,
		Total:     result.TotalNotes,
		Skipped:   result.SkippedNotes,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing commit template: %w", err)
	}

	return buf.String(), nil
}

// dataURIImageRegex matches markdown images with data: URIs (inline base64 images).
// Example: ![alt](data:image/png;base64,iVBOR...)
var dataURIImageRegex = regexp.MustCompile(`!\[([^\]]*)\]\(data:[^)]+\)`)

// cidImageRegex matches markdown images with cid: URIs (Apple Notes content ID references).
// Example: ![alt](cid:ABC-123-DEF)
var cidImageRegex = regexp.MustCompile(`!\[([^\]]*)\]\(cid:([^)]+)\)`)

// saveAndRewriteAttachments saves attachment files to disk and rewrites the
// note's markdown body to reference the saved files instead of inline data
// or cid: URIs.
func (s *Syncer) saveAndRewriteAttachments(ctx context.Context, note *model.Note, result *model.SyncResult) {
	if len(note.Attachments) == 0 {
		return
	}

	notePath := s.writer.NoteRelPath(note)
	noteDir := filepath.Dir(notePath)

	// Save each attachment and build a content ID → relative path map.
	cidToPath := make(map[string]string)
	var savedNames []string

	for j := range note.Attachments {
		att := &note.Attachments[j]
		if att.Data == nil {
			continue
		}

		savedPath, err := s.writer.SaveAttachment(ctx, notePath, att)
		if err != nil {
			s.logger.Warn("failed to save attachment",
				zap.String("note", note.Name),
				zap.String("attachment", att.Name),
				zap.Error(err),
			)
			result.Errors = append(result.Errors, fmt.Errorf("saving attachment %q for note %q: %w", att.Name, note.Name, err))
			continue
		}

		// Compute relative path from note's directory to the saved attachment.
		relFromNote, _ := filepath.Rel(noteDir, savedPath)

		if att.ContentID != "" {
			cidToPath[att.ContentID] = relFromNote
		}
		savedNames = append(savedNames, relFromNote)
		s.logger.Debug("saved attachment", zap.String("path", savedPath))
	}

	if len(savedNames) == 0 {
		return
	}

	// Rewrite cid: references in markdown with the actual file paths.
	md := note.BodyMarkdown
	md = cidImageRegex.ReplaceAllStringFunc(md, func(match string) string {
		sub := cidImageRegex.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		alt, cid := sub[1], sub[2]
		if path, ok := cidToPath[cid]; ok {
			return fmt.Sprintf("![%s](%s)", alt, path)
		}
		return match
	})

	// Replace inline data: URI images with the first available saved attachment
	// that hasn't been mapped via cid. This handles base64-embedded images.
	nameIdx := 0
	md = dataURIImageRegex.ReplaceAllStringFunc(md, func(match string) string {
		sub := dataURIImageRegex.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		alt := sub[1]

		// Find the next saved attachment that wasn't already used for a cid ref.
		for nameIdx < len(savedNames) {
			path := savedNames[nameIdx]
			nameIdx++
			// Skip paths already used as cid replacements.
			alreadyUsed := false
			for _, v := range cidToPath {
				if v == path {
					alreadyUsed = true
					break
				}
			}
			if !alreadyUsed {
				return fmt.Sprintf("![%s](%s)", alt, path)
			}
		}

		// If we run out of saved attachments, keep the original.
		return match
	})

	// Also replace any remaining raw data: URIs that might appear as plain
	// links (not images) — e.g. <data:image/png;base64,...>
	md = strings.ReplaceAll(md, "\n\n\n", "\n\n")

	note.BodyMarkdown = md
}

// rcloneSync performs the rclone sync operation.
func (s *Syncer) rcloneSync(ctx context.Context) error {
	available, err := s.rclone.IsAvailable(ctx)
	if err != nil {
		return fmt.Errorf("checking rclone availability: %w", err)
	}
	if !available {
		return fmt.Errorf("rclone is not available or remote %q is not configured", s.cfg.Rclone.RemoteName)
	}

	return s.rclone.Sync(ctx)
}
