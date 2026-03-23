# Architecture

A macOS CLI tool that exports Apple Notes to a Git repository as Markdown files, with optional Google Drive sync via rclone. Single-run binary — scheduling is handled externally via launchd.

## Package Map

```
cmd/apple-notes-sync/main.go     Cobra CLI entrypoint, dependency wiring
internal/
  model/model.go                  Domain types: Note, Folder, Attachment, SyncResult
  shell/command.go                CommandExecutor interface — the single mock seam for all subprocesses
  config/config.go                Config loading via viper (CLI flags > ENV > YAML > defaults)
  logging/logger.go               Zap logger factory (NewLogger)
  applescript/
    scripts/*.applescript         AppleScript files embedded via go:embed
    parser.go                     Parses delimiter-separated osascript output into structs
    extractor.go                  NoteExtractor interface + AppleScriptExtractor implementation
  converter/converter.go          MarkdownConverter interface — HTML to Markdown via html-to-markdown/v2
  filesystem/writer.go            NoteWriter interface — writes .md files, saves attachments, cleans orphans
  gitops/git.go                   GitClient interface — init, add, commit, push via shell
  rclone/sync.go                  Syncer interface — rclone availability check + sync
  syncer/syncer.go                Orchestrator — runs the full sync pipeline
```

## Data Flow

```
AppleScript (osascript)
    │
    ▼
Parser (delimiter protocol → []model.Note with attachment metadata)
    │
    ▼
ResolveAttachments (walks ~/Library/Group Containers/group.com.apple.notes/ to read attachment files)
    │
    ▼
Filters (exclude folders, accounts, protected, shared)
    │
    ▼
Converter (HTML → Markdown)
    │
    ▼
Writer (Markdown files to disk + attachments to _attachments/)
    │
    ▼
Git (add, commit, push)
    │
    ▼
Rclone (sync to Google Drive)
```

## Interfaces

| Interface | Package | Implementation | Purpose |
|-----------|---------|----------------|---------|
| `CommandExecutor` | `shell` | `OSCommandExecutor` | Runs subprocesses (osascript, git, rclone) |
| `NoteExtractor` | `applescript` | `AppleScriptExtractor` | Extracts notes/folders, resolves attachments |
| `MarkdownConverter` | `converter` | `HTMLToMDConverter` | Converts Apple Notes HTML to Markdown |
| `NoteWriter` | `filesystem` | `FSNoteWriter` | Writes notes/attachments to disk, cleans orphans |
| `GitClient` | `gitops` | `ShellGitClient` | Git operations via subprocess |
| `Syncer` (rclone) | `rclone` | `RcloneSyncer` | Rclone availability check + sync |

## Key Design Decisions

### Delimiter protocol for AppleScript output
AppleScript has poor JSON support. Notes are output as fields separated by `|||FIELD|||` with records separated by `|||NOTE|||`. Attachments within a note use `|||ATTACH|||` and `|||AFIELD|||` sub-delimiters. The parser (`parser.go`) handles all delimiter splitting and date parsing.

### CommandExecutor as single mock seam
All external process calls (osascript, git, rclone) go through `shell.CommandExecutor`. Tests mock this one interface to control subprocess behavior. The syncer tests mock the higher-level interfaces (NoteExtractor, NoteWriter, etc.) instead.

### go:embed for AppleScript files
Scripts live in `internal/applescript/scripts/` and are embedded into the binary via `//go:embed scripts`. No external file dependencies at runtime.

### Attachment resolution
Attachments are extracted in two phases:
1. **AppleScript** outputs attachment metadata (name + content identifier) per note
2. **ResolveAttachments** walks `~/Library/Group Containers/group.com.apple.notes/` to build a filename→filepath index, then reads matching files. Content identifier is used for disambiguation when filenames collide.

### Real filesystem in writer tests
`filesystem` tests use `t.TempDir()` for real filesystem operations rather than mocking the FS. This catches real path handling issues.

### Note file format
Each `.md` file has: `# Title` heading at top, body content, then a `---` divider with a metadata table (ID, Created, Modified, Account, Shared) at the bottom. Configurable via `front_matter` setting.

## Configuration

Precedence: CLI flags > ENV vars (`ANS_` prefix) > YAML config (`~/.apple-notes-sync.yaml`) > defaults.

Key config: `repo_path` (required), `git.enabled/push`, `rclone.enabled`, `attachments.enabled/max_size_mb`, `filter.*`, `clean_orphans`, `dry_run`, `front_matter`.

See `configs/config.example.yaml` for full reference.

## Test Strategy

| Package | Approach |
|---------|----------|
| `shell` | Real subprocess calls (`echo`, `ls`) |
| `config` | Temp YAML files via `t.TempDir()` |
| `applescript/parser` | Pure functions, no mocks |
| `applescript/extractor` | Mock `CommandExecutor` |
| `converter` | Real html-to-markdown library |
| `filesystem` | Real FS via `t.TempDir()` |
| `gitops` | Mock `CommandExecutor` |
| `rclone` | Mock `CommandExecutor` |
| `syncer` | Mock all interfaces (extractor, converter, writer, git, rclone) |

Coverage target: ≥80%. CI runs on ubuntu-latest — all tests mock osascript so macOS is not required.

## Dependencies

- `spf13/cobra` — CLI framework
- `spf13/viper` — Configuration loading
- `go.uber.org/zap` — Structured logging
- `JohannesKaufmann/html-to-markdown/v2` — HTML→Markdown conversion
- `stretchr/testify` — Test assertions and mocks
