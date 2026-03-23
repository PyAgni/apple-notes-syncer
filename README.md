# apple-notes-sync

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Platform: macOS](https://img.shields.io/badge/platform-macOS-brightgreen)](#prerequisites)
[![CI](https://github.com/PyAgni/apple-notes-syncer/actions/workflows/ci.yml/badge.svg)](https://github.com/PyAgni/apple-notes-syncer/actions)

**Seamless, automatic Git backup of your Apple Notes — with optional Google Drive sync.**
One command (or hourly launchd) turns your Notes app into a version-controlled Markdown repo.

## Table of Contents

- [Quick Start](#quick-start)
- [Features](#features)
- [Why apple-notes-sync?](#why-apple-notes-sync)
- [In Action](#in-action)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Configuration](#configuration)
- [Running Manually](#running-manually)
- [Scheduling with launchd](#scheduling-with-launchd)
- [Google Drive Setup](#google-drive-setup)
- [How Renames and Deletions Are Handled](#how-renames-and-deletions-are-handled)
- [Limitations](#limitations)
- [Troubleshooting](#troubleshooting)
- [Alternatives](#alternatives)
- [Contributing](#contributing)
- [License](#license)

## Quick Start

```bash
# 1. Install
go install github.com/PyAgni/apple-notes-syncer/cmd/apple-notes-sync@latest

# 2. Create a Git repo for your notes
mkdir ~/Notes && cd ~/Notes
git init
git remote add origin git@github.com:yourusername/my-notes.git
git commit --allow-empty -m "init"
git push -u origin main

# 3. Configure
cp configs/config.example.yaml ~/.apple-notes-sync.yaml
# Edit repo_path in the config file

# 4. Run
apple-notes-sync --repo-path ~/Notes

# 5. (Optional) Schedule hourly syncs
make launchd
```

Done. Your notes now live in Git with full history.

## Features

- Extracts all notes from the macOS Notes app via AppleScript
- Converts HTML note bodies to clean Markdown
- Mirrors the Notes folder hierarchy into the repository
- Adds a metadata table (ID, dates, account) to each note
- Commits with timestamped messages and pushes to your remote
- Optionally syncs to Google Drive via rclone
- Cleans up notes that were deleted from Apple Notes
- Configurable via CLI flags, environment variables, or YAML config file

## Why apple-notes-sync?

Apple Notes is great for writing, but getting your notes *out* is painful. There's no export-all, no API, and no version history. If you work with AI agents or just want peace of mind, that's a problem.

I built this because I needed to feed my Apple Notes to LLMs while working on another project. Manually exporting each note was a non-starter. Now my notes live in a Git repo, and any agent — Claude Code, OpenClaw, Hermes, or whatever comes next — can access them instantly.

**What you get:**

- **Feed notes to AI agents** — point any tool at your GitHub repo and it has all your notes as context
- **Version history** — every edit is a Git commit; accidentally delete a note and it's one `git` command away, no digging through Apple backups required
- **Continuous sync, not a one-shot export** — schedule with launchd and forget about it
- **Orphan cleanup** — deleted notes disappear from the repo automatically, but tracked via git history
- **Folder mirroring** — your Notes folder structure is preserved exactly
- **rclone integration** — Google Drive as a bonus backup layer, and also useful for working with NotebookLM

> **Note**: This is a one-way export. Edits made to the Markdown files do not flow back to Apple Notes.

## In Action

<!-- TODO: Add demo GIF or screenshot -->
<!-- ![Demo](https://github.com/PyAgni/apple-notes-syncer/raw/main/assets/demo.gif) -->

Example output for a single note:

```markdown
# My Project Ideas

Content of the note converted to clean Markdown...

- Bullet points preserved
- [Links](https://example.com) converted properly
- **Bold** and *italic* formatting kept

---

| ID | Created | Modified | Account | Shared |
|----|---------|----------|---------|--------|
| x-coredata://abc123 | 2026-03-18 16:00:00 | 2026-03-20 09:30:00 | iCloud | No |
```

Repository structure mirrors your Notes folders:

```
~/Notes/
├── Work/
│   ├── Meeting Notes.md
│   └── Project Ideas.md
├── Personal/
│   ├── Travel Plans.md
│   └── Reading List.md
├── Recipes/
│   └── Pasta Carbonara.md
└── ...
```

## Prerequisites

- **macOS** (required — uses AppleScript to access Notes)
- **Go 1.26+** (for building from source)
- **git** (configured with SSH key or token for your remote)
- **rclone** (optional, only for Google Drive sync)

## Installation

### From source

```bash
git clone https://github.com/PyAgni/apple-notes-syncer.git
cd apple-notes-syncer
make install
```

### Using `go install`

```bash
go install github.com/PyAgni/apple-notes-syncer/cmd/apple-notes-sync@latest
```

> If you get `command not found: apple-notes-sync`, add Go's bin directory to your PATH:
> ```bash
> echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc && source ~/.zshrc
> ```

## Configuration

Configuration is loaded from (in order of precedence):

1. CLI flags
2. Environment variables (prefix: `ANS_`)
3. YAML config file (`~/.apple-notes-sync.yaml`)
4. Defaults

Minimal config to get started:

```yaml
repo_path: ~/Notes
```

See [`configs/config.example.yaml`](configs/config.example.yaml) for all options. Copy it:

```bash
cp configs/config.example.yaml ~/.apple-notes-sync.yaml
```

### CLI flags

```
--config, -c     Path to config file (default: ~/.apple-notes-sync.yaml)
--repo-path      Path to the git repository
--dry-run        Preview changes without writing or committing
--log-level      Log level: debug, info, warn, error
--verbose, -v    Shortcut for --log-level=debug
```

### Environment variables

All config keys can be set via environment variables with the `ANS_` prefix. Nested keys use underscores:

```bash
export ANS_REPO_PATH=~/Notes
export ANS_DRY_RUN=true
export ANS_GIT_PUSH=false
export ANS_LOG_LEVEL=debug
export ANS_RCLONE_ENABLED=true
export ANS_RCLONE_REMOTE_NAME=gdrive
export ANS_RCLONE_REMOTE_PATH=AppleNotes
```

<details>
<summary>Full config reference</summary>

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `repo_path` | string | *required* | Path to the git repository |
| `notes_subdir` | string | `""` | Subdirectory for notes within the repo |
| `git.enabled` | bool | `true` | Enable git operations |
| `git.remote` | string | `"origin"` | Git remote name |
| `git.branch` | string | `"main"` | Git branch |
| `git.push` | bool | `true` | Push after committing |
| `rclone.enabled` | bool | `false` | Enable rclone sync |
| `rclone.remote_name` | string | `""` | Rclone remote name |
| `rclone.remote_path` | string | `""` | Path on the remote |
| `rclone.extra_flags` | []string | `[]` | Additional rclone flags |
| `log.level` | string | `"info"` | Log level |
| `log.file` | string | `""` | Log file path (empty = stderr) |
| `log.format` | string | `"console"` | Log format: console or json |
| `filter.accounts` | []string | `[]` | Include only these accounts |
| `filter.exclude_accounts` | []string | `[]` | Exclude these accounts |
| `filter.folders` | []string | `[]` | Include only these folders |
| `filter.exclude_folders` | []string | `["Recently Deleted"]` | Exclude these folders |
| `filter.skip_protected` | bool | `true` | Skip password-protected notes |
| `filter.skip_shared` | bool | `false` | Skip shared notes |
| `attachments.enabled` | bool | `true` | Extract attachments |
| `attachments.max_size_mb` | int | `50` | Max attachment size in MB |
| `attachments.dir` | string | `"_attachments"` | Attachment subdirectory |
| `dry_run` | bool | `false` | Preview mode |
| `front_matter` | bool | `true` | Add metadata table to notes |
| `clean_orphans` | bool | `true` | Remove deleted notes |
| `timeout` | duration | `120s` | AppleScript timeout |
| `commit_template` | string | see below | Commit message Go template |

Default commit template: `apple-notes-sync: {{.Timestamp}} \| {{.Written}} notes synced`

Template fields: `.Timestamp`, `.Written`, `.Total`, `.Skipped`

</details>

## Running Manually

```bash
# Basic run
apple-notes-sync --repo-path ~/Notes

# Dry run (preview without writing)
apple-notes-sync --repo-path ~/Notes --dry-run --verbose

# With config file
apple-notes-sync --config ~/.apple-notes-sync.yaml
```

## Scheduling with launchd

The binary is a single-run CLI — scheduling is handled by macOS launchd.

1. Edit the plist template with your username and config path:

```bash
cp launchd/com.apple-notes-sync.plist ~/Library/LaunchAgents/
# Edit YOUR_USERNAME in the plist file
```

2. Or use the Makefile (after editing the plist):

```bash
make launchd      # Install and load
make unlaunchd    # Unload and remove
```

The default schedule runs every hour. Change `StartInterval` in the plist to adjust (value is in seconds).

Create the log directory:

```bash
mkdir -p ~/Library/Logs/apple-notes-sync
```

## Google Drive Setup

1. Install rclone: `brew install rclone`

2. Configure a Google Drive remote:

```bash
rclone config
# Choose "New remote"
# Name: gdrive
# Type: Google Drive
# Follow the OAuth flow
```

3. Enable rclone in your config:

```yaml
rclone:
  enabled: true
  remote_name: gdrive
  remote_path: AppleNotes
```

4. Test it manually first:

```bash
rclone sync ~/Notes gdrive:AppleNotes --dry-run
```

## How Renames and Deletions Are Handled

- **Renamed notes**: A renamed note appears as a new file and the old filename is removed (if `clean_orphans: true`). This shows as a delete + add in git, which GitHub renders as a rename if content is similar.
- **Deleted notes**: When a note is deleted from Apple Notes, the corresponding `.md` file is removed on the next sync (if `clean_orphans: true`).
- **Moved notes**: Moving a note to a different folder creates the file in the new directory and removes it from the old one.

## Limitations

- **macOS only** — relies on AppleScript to access the Notes app
- **One-way export** — edits to Markdown files do not sync back to Apple Notes
- **AppleScript permissions required** — on first run, macOS will prompt to allow automation access
- **Attachments >50 MB skipped** by default (configurable via `attachments.max_size_mb`)
- **Large note libraries** may take a few minutes on the first sync (AppleScript extraction is the bottleneck)

## Troubleshooting

| Problem | Solution |
|---------|----------|
| AppleScript timeout | Increase `timeout: 300s` in your config |
| Permission denied on Notes | System Settings > Privacy & Security > Automation > allow Terminal (or your terminal app) |
| rclone OAuth expired | Run `rclone config` again to re-authenticate |
| Unicode errors in dates | Already handled — the parser normalizes Unicode whitespace from macOS locales |
| "repo_path is required" | Set `repo_path` in your config file or pass `--repo-path` |
| `command not found: apple-notes-sync` | Add Go's bin to your PATH: `export PATH="$PATH:$(go env GOPATH)/bin"` (add to `~/.zshrc` to persist) |

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.

1. Fork the repository
2. Create a feature branch: `git checkout -b my-feature`
3. Make your changes
4. Ensure tests and linting pass: `make lint test`
5. Submit a pull request

### Development

```bash
make build          # Build the binary
make test           # Run tests with race detector
make check-coverage # Run tests and check coverage >= 80%
make lint           # Run go vet + staticcheck
make fmt            # Format code
make tidy           # Tidy go modules
make help           # Show all available targets
```

## License

[MIT](LICENSE)
