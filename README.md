# apple-notes-sync

A macOS CLI tool that exports Apple Notes to a Git repository as Markdown files, with optional Google Drive sync via rclone.

## Features

- Extracts all notes from the macOS Notes app via AppleScript
- Converts HTML note bodies to clean Markdown
- Mirrors the Notes folder hierarchy into the repository
- Adds YAML front matter (title, dates, account) to each note
- Commits with timestamped messages and pushes to your remote
- Optionally syncs to Google Drive via rclone
- Cleans up notes that were deleted from Apple Notes
- Configurable via CLI flags, environment variables, or YAML config file

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
go install github.com/agni/apple-notes-sync/cmd/apple-notes-sync@latest
```

## One-time repo setup

Create and initialize a Git repository for your notes:

```bash
mkdir ~/Notes
cd ~/Notes
git init
git remote add origin git@github.com:yourusername/my-notes.git
git commit --allow-empty -m "init"
git push -u origin main
```

## Configuration

Configuration is loaded from (in order of precedence):

1. CLI flags
2. Environment variables (prefix: `ANS_`)
3. YAML config file (`~/.apple-notes-sync.yaml`)
4. Defaults

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

### YAML config file

See [`configs/config.example.yaml`](configs/config.example.yaml) for a complete reference. Copy it to get started:

```bash
cp configs/config.example.yaml ~/.apple-notes-sync.yaml
# Edit with your settings
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
| `front_matter` | bool | `true` | Add YAML front matter |
| `clean_orphans` | bool | `true` | Remove deleted notes |
| `timeout` | duration | `120s` | AppleScript timeout |
| `commit_template` | string | see below | Commit message Go template |

Default commit template: `apple-notes-sync: {{.Timestamp}} \| {{.Written}} notes synced`

Template fields: `.Timestamp`, `.Written`, `.Total`, `.Skipped`

</details>

## Running manually

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

## Google Drive setup

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

## How renames and deletions are handled

- **Renamed notes**: A renamed note appears as a new file and the old filename is removed (if `clean_orphans: true`). This shows as a delete + add in git, which GitHub renders as a rename if content is similar.
- **Deleted notes**: When a note is deleted from Apple Notes, the corresponding `.md` file is removed on the next sync (if `clean_orphans: true`).
- **Moved notes**: Moving a note to a different folder creates the file in the new directory and removes it from the old one.

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b my-feature`
3. Make your changes
4. Ensure tests and linting pass: `make lint test`
5. Submit a pull request

### Development

```bash
make build          # Build the binary
make test           # Run tests with race detector
make check-coverage # Run tests and check coverage ≥80%
make lint           # Run go vet + staticcheck
make fmt            # Format code
make tidy           # Tidy go modules
```

## License

[MIT](LICENSE)
