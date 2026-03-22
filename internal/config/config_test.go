package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load("")
	require.NoError(t, err)

	assert.True(t, cfg.Git.Enabled)
	assert.Equal(t, "origin", cfg.Git.Remote)
	assert.Equal(t, "main", cfg.Git.Branch)
	assert.True(t, cfg.Git.Push)

	assert.False(t, cfg.Rclone.Enabled)

	assert.Equal(t, "info", cfg.Log.Level)
	assert.Equal(t, "console", cfg.Log.Format)

	assert.True(t, cfg.FrontMatter)
	assert.True(t, cfg.CleanOrphans)
	assert.False(t, cfg.DryRun)

	assert.Equal(t, []string{"Recently Deleted"}, cfg.Filter.ExcludeFolders)
	assert.True(t, cfg.Filter.SkipProtected)

	assert.True(t, cfg.Attachments.Enabled)
	assert.Equal(t, 50, cfg.Attachments.MaxSizeMB)
	assert.Equal(t, "_attachments", cfg.Attachments.Dir)

	assert.Equal(t, DefaultCommitTemplate, cfg.CommitTemplate)
}

func TestLoad_FromYAMLFile(t *testing.T) {
	content := `
repo_path: /tmp/my-notes
dry_run: true
git:
  remote: upstream
  branch: develop
  push: false
rclone:
  enabled: true
  remote_name: gdrive
  remote_path: Notes
log:
  level: debug
  format: json
filter:
  accounts:
    - iCloud
  exclude_folders:
    - Trash
    - Recently Deleted
  skip_shared: true
attachments:
  max_size_mb: 25
timeout: 60s
`
	tmpFile := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(tmpFile, []byte(content), 0644))

	cfg, err := Load(tmpFile)
	require.NoError(t, err)

	assert.Equal(t, "/tmp/my-notes", cfg.RepoPath)
	assert.True(t, cfg.DryRun)
	assert.Equal(t, "upstream", cfg.Git.Remote)
	assert.Equal(t, "develop", cfg.Git.Branch)
	assert.False(t, cfg.Git.Push)
	assert.True(t, cfg.Rclone.Enabled)
	assert.Equal(t, "gdrive", cfg.Rclone.RemoteName)
	assert.Equal(t, "Notes", cfg.Rclone.RemotePath)
	assert.Equal(t, "debug", cfg.Log.Level)
	assert.Equal(t, "json", cfg.Log.Format)
	assert.Equal(t, []string{"iCloud"}, cfg.Filter.Accounts)
	assert.Equal(t, []string{"Trash", "Recently Deleted"}, cfg.Filter.ExcludeFolders)
	assert.True(t, cfg.Filter.SkipShared)
	assert.Equal(t, 25, cfg.Attachments.MaxSizeMB)
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("ANS_REPO_PATH", "/env/notes")
	t.Setenv("ANS_DRY_RUN", "true")
	t.Setenv("ANS_LOG_LEVEL", "debug")

	cfg, err := Load("")
	require.NoError(t, err)

	assert.Equal(t, "/env/notes", cfg.RepoPath)
	assert.True(t, cfg.DryRun)
	assert.Equal(t, "debug", cfg.Log.Level)
}

func TestLoad_NonexistentExplicitFile(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading config file")
}

func TestValidate_MissingRepoPath(t *testing.T) {
	cfg := &Config{
		Log:         LogConfig{Level: "info", Format: "console"},
		Timeout:     120,
		Attachments: AttachmentConfig{MaxSizeMB: 50},
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "repo_path is required")
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	cfg := &Config{
		RepoPath:    "/tmp/notes",
		Log:         LogConfig{Level: "verbose", Format: "console"},
		Timeout:     120,
		Attachments: AttachmentConfig{MaxSizeMB: 50},
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid log level")
}

func TestValidate_InvalidLogFormat(t *testing.T) {
	cfg := &Config{
		RepoPath:    "/tmp/notes",
		Log:         LogConfig{Level: "info", Format: "xml"},
		Timeout:     120,
		Attachments: AttachmentConfig{MaxSizeMB: 50},
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid log format")
}

func TestValidate_RcloneEnabledMissingFields(t *testing.T) {
	cfg := &Config{
		RepoPath:    "/tmp/notes",
		Log:         LogConfig{Level: "info", Format: "console"},
		Rclone:      RcloneConfig{Enabled: true},
		Timeout:     120,
		Attachments: AttachmentConfig{MaxSizeMB: 50},
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rclone.remote_name is required")

	cfg.Rclone.RemoteName = "gdrive"
	err = cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rclone.remote_path is required")
}

func TestValidate_InvalidTimeout(t *testing.T) {
	cfg := &Config{
		RepoPath:    "/tmp/notes",
		Log:         LogConfig{Level: "info", Format: "console"},
		Timeout:     0,
		Attachments: AttachmentConfig{MaxSizeMB: 50},
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout must be a positive duration")
}

func TestValidate_InvalidAttachmentSize(t *testing.T) {
	cfg := &Config{
		RepoPath:    "/tmp/notes",
		Log:         LogConfig{Level: "info", Format: "console"},
		Timeout:     120,
		Attachments: AttachmentConfig{MaxSizeMB: 0},
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "attachments.max_size_mb must be positive")
}

func TestValidate_ExpandTilde(t *testing.T) {
	cfg := &Config{
		RepoPath:    "~/notes",
		Log:         LogConfig{Level: "info", Format: "console"},
		Timeout:     120,
		Attachments: AttachmentConfig{MaxSizeMB: 50},
	}
	err := cfg.Validate()
	require.NoError(t, err)

	home, _ := os.UserHomeDir()
	assert.Equal(t, filepath.Join(home, "notes"), cfg.RepoPath)
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &Config{
		RepoPath: "/tmp/notes",
		Log:      LogConfig{Level: "info", Format: "console"},
		Rclone: RcloneConfig{
			Enabled:    true,
			RemoteName: "gdrive",
			RemotePath: "Notes",
		},
		Timeout:     120,
		Attachments: AttachmentConfig{MaxSizeMB: 50},
	}
	err := cfg.Validate()
	require.NoError(t, err)
}
