# Plan: apple-notes-sync CLI Tool

## Context

Build a macOS CLI tool in Go that exports Apple Notes to a Git repository as Markdown files, with optional Google Drive sync via rclone. The tool is a single-run binary (not a daemon) — scheduling is handled externally via launchd. The project should be professional, open-source ready (MIT), with >80% test coverage.

---

## Project Structure

```
apple-notes-syncer/
├── cmd/
│   └── apple-notes-sync/
│       └── main.go                    # Entrypoint: cobra CLI, dependency wiring
├── internal/
│   ├── model/
│   │   └── model.go                   # Note, Folder, Attachment, SyncResult structs
│   ├── shell/
│   │   ├── command.go                 # CommandExecutor interface + OSCommandExecutor
│   │   └── command_test.go
│   ├── config/
│   │   ├── config.go                  # Config struct, Load(), Validate() via viper
│   │   └── config_test.go
│   ├── logging/
│   │   └── logger.go                  # Zap logger factory (helper, no tests needed)
│   ├── applescript/
│   │   ├── extractor.go               # NoteExtractor interface + AppleScriptExtractor
│   │   ├── extractor_test.go
│   │   ├── parser.go                  # Parse delimited osascript output → structs
│   │   └── parser_test.go
│   ├── converter/
│   │   ├── converter.go               # MarkdownConverter interface + HTMLToMDConverter
│   │   └── converter_test.go
│   ├── filesystem/
│   │   ├── writer.go                  # NoteWriter interface + FSNoteWriter
│   │   └── writer_test.go
│   ├── gitops/
│   │   ├── git.go                     # GitClient interface + ShellGitClient
│   │   └── git_test.go
│   ├── rclone/
│   │   ├── sync.go                    # Syncer interface + RcloneSyncer
│   │   └── sync_test.go
│   └── syncer/
│       ├── syncer.go                  # Orchestrator: full sync pipeline
│       └── syncer_test.go
├── scripts/
│   └── applescript/
│       ├── get_all_notes.applescript  # Extract all notes (embedded via go:embed)
│       └── get_folders.applescript    # Extract folder hierarchy
├── testdata/
│   ├── applescript_output/            # Sample osascript outputs for parser tests
│   ├── html_samples/                  # Real Apple Notes HTML for converter tests
│   └── configs/                       # Test config YAML files
├── configs/
│   └── config.example.yaml
├── launchd/
│   └── com.apple-notes-sync.plist
├── .github/
│   └── workflows/
│       └── ci.yml
├── docs/
│   └── Plan.md                        # This plan (copied here during implementation)
├── .gitignore
├── .golangci.yml
├── go.mod
├── go.sum
├── LICENSE
├── Makefile
└── README.md
```

---

## Interfaces & Key Types

### `internal/model/model.go` — Domain Types

```go
type AttachmentType string // "link", "image", "video", "file"

type Attachment struct {
    Type     AttachmentType
    Name     string
    URL      string   // URL for links, file path for local
    MIMEType string
    Data     []byte   // Raw data if available (may be nil)
}

type Note struct {
    ID           string
    Name         string
    BodyHTML     string
    BodyMarkdown string       // Populated after conversion
    PlainText    string
    FolderPath   string       // e.g. "Tech/Go Projects"
    Account      string
    CreatedAt    time.Time
    ModifiedAt   time.Time
    Shared       bool
    Protected    bool
    Attachments  []Attachment
}

type Folder struct {
    ID       string
    Name     string
    Account  string
    Path     string   // Full path e.g. "Study/Go"
}

type SyncResult struct {
    TotalNotes    int
    WrittenNotes  int
    SkippedNotes  int
    Errors        []error
    GitCommitHash string
    RcloneSynced  bool
    Duration      time.Duration
}
```

### `internal/shell/command.go` — Command Executor (foundation mock seam)

```go
type CommandResult struct {
    Stdout   string
    Stderr   string
    ExitCode int
}

type CommandExecutor interface {
    Execute(ctx context.Context, name string, args ...string) (*CommandResult, error)
    ExecuteInDir(ctx context.Context, dir string, name string, args ...string) (*CommandResult, error)
}
```

All external process calls (osascript, git, rclone) go through this single interface.

### `internal/applescript/extractor.go` — Notes Extraction

```go
type NoteExtractor interface {
    GetFolders(ctx context.Context) ([]model.Folder, error)
    GetAllNotes(ctx context.Context, accounts []string, folders []string) ([]model.Note, error)
}
```

Implementation calls `osascript` with embedded `.applescript` files (via `go:embed`). Output uses delimiter protocol (`|||FIELD|||` between fields, `|||NOTE|||` between records) parsed by `parser.go`.

### `internal/converter/converter.go` — HTML to Markdown

```go
type MarkdownConverter interface {
    Convert(html string) (string, error)
}
```

Uses `github.com/JohannesKaufmann/html-to-markdown/v2`. Handles Apple Notes HTML quirks (title `<h1>` duplication, checklist markup, `<div>` wrapping).

### `internal/filesystem/writer.go` — File Output

```go
type NoteWriter interface {
    WriteNote(ctx context.Context, note *model.Note) (string, error)
    WriteAll(ctx context.Context, notes []model.Note) ([]string, error)
    CleanOrphanedFiles(ctx context.Context, currentNotePaths []string) ([]string, error)
    SaveAttachment(ctx context.Context, notePath string, attachment *model.Attachment) (string, error)
}
```

Files get YAML front matter (opt-in, default true) with title, id, created, modified, account. Filenames sanitized from note title; duplicates get ID suffix.

### `internal/gitops/git.go` — Git Operations

```go
type GitClient interface {
    Init(ctx context.Context) error
    AddAll(ctx context.Context) error
    HasChanges(ctx context.Context) (bool, error)
    Commit(ctx context.Context, message string) (string, error)
    Push(ctx context.Context) error
}
```

### `internal/rclone/sync.go` — Cloud Sync

```go
type Syncer interface {
    Sync(ctx context.Context) error
    IsAvailable(ctx context.Context) (bool, error)
}
```

### `internal/syncer/syncer.go` — Orchestrator

```go
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

// Sync executes the full sync pipeline and returns a summary result.
func (s *Syncer) Sync(ctx context.Context) (*model.SyncResult, error)
```

Pipeline: Extract → Convert → Write → Clean orphans → Git commit/push → Rclone sync

---

## Configuration

### Config Struct (`internal/config/config.go`)

```go
type Config struct {
    RepoPath       string          `mapstructure:"repo_path"        yaml:"repo_path"`
    NotesSubdir    string          `mapstructure:"notes_subdir"     yaml:"notes_subdir"`    // default: ""
    Git            GitConfig       `mapstructure:"git"              yaml:"git"`
    Rclone         RcloneConfig    `mapstructure:"rclone"           yaml:"rclone"`
    Log            LogConfig       `mapstructure:"log"              yaml:"log"`
    Filter         FilterConfig    `mapstructure:"filter"           yaml:"filter"`
    Attachments    AttachmentConfig `mapstructure:"attachments"     yaml:"attachments"`
    DryRun         bool            `mapstructure:"dry_run"          yaml:"dry_run"`
    CommitTemplate string          `mapstructure:"commit_template"  yaml:"commit_template"`
    FrontMatter    bool            `mapstructure:"front_matter"     yaml:"front_matter"`    // default: true
    CleanOrphans   bool            `mapstructure:"clean_orphans"    yaml:"clean_orphans"`   // default: true
    Timeout        time.Duration   `mapstructure:"timeout"          yaml:"timeout"`          // default: 120s
}

type GitConfig struct {
    Enabled bool   `yaml:"enabled"` // default: true
    Remote  string `yaml:"remote"`  // default: "origin"
    Branch  string `yaml:"branch"`  // default: "main"
    Push    bool   `yaml:"push"`    // default: true
}

type RcloneConfig struct {
    Enabled    bool     `yaml:"enabled"`     // default: false
    RemoteName string   `yaml:"remote_name"`
    RemotePath string   `yaml:"remote_path"`
    ExtraFlags []string `yaml:"extra_flags"`
}

type LogConfig struct {
    Level  string `yaml:"level"`  // default: "info"
    File   string `yaml:"file"`   // default: "" (stderr)
    Format string `yaml:"format"` // "json" or "console", default: "console"
}

type FilterConfig struct {
    Accounts        []string `yaml:"accounts"`
    ExcludeAccounts []string `yaml:"exclude_accounts"`
    Folders         []string `yaml:"folders"`
    ExcludeFolders  []string `yaml:"exclude_folders"`  // default: ["Recently Deleted"]
    SkipProtected   bool     `yaml:"skip_protected"`   // default: true
    SkipShared      bool     `yaml:"skip_shared"`
}

type AttachmentConfig struct {
    Enabled   bool   `yaml:"enabled"`     // default: true
    MaxSizeMB int    `yaml:"max_size_mb"` // default: 50
    Dir       string `yaml:"dir"`         // default: "_attachments"
}
```

**Precedence:** CLI flags > ENV vars (`ANS_` prefix) > YAML config file > defaults

**CLI flags:** `--config/-c`, `--repo-path`, `--dry-run`, `--log-level`, `--verbose/-v`

---

## AppleScript Design

Two embedded scripts in `scripts/applescript/`:

**`get_all_notes.applescript`**: Iterates all accounts/folders, outputs each note as delimiter-separated fields:
```
id |||FIELD||| name |||FIELD||| body |||FIELD||| creation_date |||FIELD||| modification_date |||FIELD||| account |||FIELD||| folder |||FIELD||| protected |||FIELD||| shared |||NOTE|||
```

**`get_folders.applescript`**: Outputs folder hierarchy with account, name, ID, path.

Both are embedded into the binary via `//go:embed` — no external file dependencies at runtime.

**Key consideration**: For very large note libraries, the single-call approach may timeout. The `timeout` config (default 120s) controls this. If this becomes an issue in practice, a follow-up can batch by folder.

---

## Testing Strategy

### Mocking Approach
- **`stretchr/testify/mock`** for all mocks
- **Single mock seam**: `shell.CommandExecutor` is mocked for all external process tests (osascript, git, rclone)
- Each interface also gets its own mock for orchestration tests in `syncer`

### Per-Package Testing

| Package | What to test | Mock strategy |
|---------|-------------|---------------|
| `shell` | Real `echo`/`ls` calls | No mocks (real subprocess) |
| `config` | YAML loading, env override, validation, defaults | Temp files via `t.TempDir()` |
| `applescript/parser` | Parsing delimited output, edge cases (empty, malformed, special chars) | No mocks (pure functions) |
| `applescript/extractor` | Correct osascript invocation, account/folder filtering | Mock `CommandExecutor` |
| `converter` | HTML→MD conversion, Apple Notes HTML quirks | No mocks (real library) |
| `filesystem` | Dir creation, file content, front matter, sanitization, orphan cleanup | Real FS via `t.TempDir()` |
| `gitops` | Correct git commands, `HasChanges` parsing, error handling | Mock `CommandExecutor` |
| `rclone` | Correct rclone commands, `IsAvailable` check | Mock `CommandExecutor` |
| `syncer` | Full pipeline flow, dry-run, partial failures, rclone skip | Mock all interfaces |

### CI Pipeline (`.github/workflows/ci.yml`)

- **Runs on `ubuntu-latest`** — all unit tests mock `osascript` so macOS is not required
- Jobs: `lint` (golangci-lint), `test` (with coverage check ≥80%), `build`
- Go version: 1.22+

---

## Makefile Targets

```makefile
build          # go build -o bin/apple-notes-sync ./cmd/apple-notes-sync/
test           # go test ./... -race
test-coverage  # go test -coverprofile=coverage.out -covermode=atomic ./...
check-coverage # Fail if coverage < 80%
cover          # go tool cover -html=coverage.out (open in browser)
lint           # golangci-lint run ./...
fmt            # go fmt + goimports
vet            # go vet ./...
tidy           # go mod tidy && go mod verify
install        # go build then cp bin/apple-notes-sync /usr/local/bin/
launchd        # cp plist to ~/Library/LaunchAgents/ && launchctl load
unlaunchd      # launchctl unload && rm plist
clean          # rm -rf bin/ coverage.out
help           # List all targets
```

Build includes `-ldflags` for version, commit hash, and build date.

---

## Implementation Phases (Build Order)

### Phase 1: Skeleton & Foundation
1. `go mod init`, `.gitignore`, `LICENSE` (MIT)
2. `internal/model/model.go`
3. `internal/shell/command.go` + tests
4. `internal/config/config.go` + tests
5. `internal/logging/logger.go` (small helper, no separate tests)
6. `cmd/apple-notes-sync/main.go` — cobra skeleton (flags, config load, version)

### Phase 2: Extraction Pipeline
7. `scripts/applescript/get_all_notes.applescript`
8. `scripts/applescript/get_folders.applescript`
9. `internal/applescript/parser.go` + tests (with testdata samples)
10. `internal/applescript/extractor.go` + tests

### Phase 3: Conversion & Output
11. `internal/converter/converter.go` + tests
12. `internal/filesystem/writer.go` + tests

### Phase 4: Git & Rclone
13. `internal/gitops/git.go` + tests
14. `internal/rclone/sync.go` + tests

### Phase 5: Orchestration
15. `internal/syncer/syncer.go` + tests
16. Wire everything in `main.go`

### Phase 6: Polish & Documentation
17. `Makefile`
18. `.github/workflows/ci.yml`, `.golangci.yml`
19. `launchd/com.apple-notes-sync.plist`
20. `configs/config.example.yaml`
21. `README.md` (all 10 sections from requirements)
22. `docs/Plan.md` (copy of this plan)

---

## Key Dependencies

```
github.com/spf13/cobra         v1.8+
github.com/spf13/viper         v1.18+
go.uber.org/zap                v1.27+
github.com/JohannesKaufmann/html-to-markdown/v2  v2.3+
github.com/stretchr/testify    v1.9+
```

---

## Code Quality Standards

- **All errors must be wrapped** with `fmt.Errorf("context: %w", err)` — never return bare errors
- **All exported functions and types must have Go doc comments** for readability and open-source friendliness
- **`logging` is a small helper** (just a `NewLogger` factory function) — no separate package tests needed, it's exercised transitively
- **`syncer`** (not `runner`) is the orchestrator package name — it better describes the domain

---

## Design Decisions

1. **Delimiter protocol for AppleScript output** — AppleScript has poor JSON support; `|||FIELD|||` / `|||NOTE|||` delimiters are reliable and simple to parse
2. **`go:embed` for AppleScript files** — single binary with no external file dependencies
3. **`shell.CommandExecutor` as the single mock seam** — one interface covers osascript, git, and rclone subprocess calls
4. **Real filesystem in writer tests** — `t.TempDir()` is more realistic than FS mocking
5. **Front matter default on** — aids tools like Obsidian; configurable off
6. **CI on ubuntu-latest** — all unit tests mock external commands, no macOS runner needed

---

## Verification

After implementation, verify end-to-end:

1. **Unit tests**: `make test-coverage` — all pass, ≥80% coverage
2. **Lint**: `make lint` — no issues
3. **Build**: `make build` — binary in `bin/`
4. **Manual dry run**: `./bin/apple-notes-sync --dry-run --log-level=debug` — extracts notes, logs what would be written, no actual file changes
5. **Full run**: `./bin/apple-notes-sync --repo-path=/tmp/test-notes` — notes exported as `.md` files with correct hierarchy
6. **Git verification**: Check `/tmp/test-notes` has a git commit with the synced notes
7. **CI**: Push to GitHub, verify Actions workflow passes
