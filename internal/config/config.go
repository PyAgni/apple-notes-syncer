// Package config handles loading, validating, and providing default values
// for the apple-notes-sync configuration. Configuration is sourced from
// CLI flags, environment variables (ANS_ prefix), and a YAML config file,
// with that precedence order.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration.
type Config struct {
	// RepoPath is the path to the git repository where notes are stored.
	RepoPath string `mapstructure:"repo_path" yaml:"repo_path"`
	// NotesSubdir is an optional subdirectory within the repo for notes.
	// Empty string means notes are stored at the repo root.
	NotesSubdir string `mapstructure:"notes_subdir" yaml:"notes_subdir"`

	// Git holds git-related configuration.
	Git GitConfig `mapstructure:"git" yaml:"git"`
	// Rclone holds rclone sync configuration.
	Rclone RcloneConfig `mapstructure:"rclone" yaml:"rclone"`
	// Log holds logging configuration.
	Log LogConfig `mapstructure:"log" yaml:"log"`
	// Filter holds note filtering configuration.
	Filter FilterConfig `mapstructure:"filter" yaml:"filter"`
	// Attachments holds attachment handling configuration.
	Attachments AttachmentConfig `mapstructure:"attachments" yaml:"attachments"`

	// DryRun when true prevents writing files, committing, or pushing.
	DryRun bool `mapstructure:"dry_run" yaml:"dry_run"`
	// CommitTemplate is a Go text/template string for commit messages.
	// Available fields: .Timestamp, .Written, .Total, .Skipped.
	CommitTemplate string `mapstructure:"commit_template" yaml:"commit_template"`
	// FrontMatter controls whether YAML front matter is added to note files.
	FrontMatter bool `mapstructure:"front_matter" yaml:"front_matter"`
	// CleanOrphans controls whether notes deleted from Apple Notes are removed from disk.
	CleanOrphans bool `mapstructure:"clean_orphans" yaml:"clean_orphans"`
	// Timeout is the maximum duration for AppleScript execution.
	Timeout time.Duration `mapstructure:"timeout" yaml:"timeout"`
}

// GitConfig holds git-related settings.
type GitConfig struct {
	// Enabled controls whether git operations are performed.
	Enabled bool `mapstructure:"enabled" yaml:"enabled"`
	// Remote is the git remote name (e.g. "origin").
	Remote string `mapstructure:"remote" yaml:"remote"`
	// Branch is the git branch to commit to (e.g. "main").
	Branch string `mapstructure:"branch" yaml:"branch"`
	// Push controls whether commits are pushed to the remote.
	Push bool `mapstructure:"push" yaml:"push"`
}

// RcloneConfig holds rclone sync settings.
type RcloneConfig struct {
	// Enabled controls whether rclone sync is performed after git operations.
	Enabled bool `mapstructure:"enabled" yaml:"enabled"`
	// RemoteName is the rclone remote name (e.g. "gdrive").
	RemoteName string `mapstructure:"remote_name" yaml:"remote_name"`
	// RemotePath is the path on the remote (e.g. "AppleNotes").
	RemotePath string `mapstructure:"remote_path" yaml:"remote_path"`
	// ExtraFlags are additional flags passed to rclone (e.g. ["--verbose"]).
	ExtraFlags []string `mapstructure:"extra_flags" yaml:"extra_flags"`
}

// LogConfig holds logging settings.
type LogConfig struct {
	// Level is the log level: "debug", "info", "warn", or "error".
	Level string `mapstructure:"level" yaml:"level"`
	// File is the path to a log file. Empty string means stderr only.
	File string `mapstructure:"file" yaml:"file"`
	// Format is the log format: "json" or "console".
	Format string `mapstructure:"format" yaml:"format"`
}

// FilterConfig holds note filtering settings.
type FilterConfig struct {
	// Accounts limits sync to these account names. Empty means all accounts.
	Accounts []string `mapstructure:"accounts" yaml:"accounts"`
	// ExcludeAccounts skips these account names.
	ExcludeAccounts []string `mapstructure:"exclude_accounts" yaml:"exclude_accounts"`
	// Folders limits sync to these folder paths. Empty means all folders.
	Folders []string `mapstructure:"folders" yaml:"folders"`
	// ExcludeFolders skips these folder paths. Defaults to ["Recently Deleted"].
	ExcludeFolders []string `mapstructure:"exclude_folders" yaml:"exclude_folders"`
	// SkipProtected skips password-protected notes (which have empty bodies).
	SkipProtected bool `mapstructure:"skip_protected" yaml:"skip_protected"`
	// SkipShared skips notes that are shared with others.
	SkipShared bool `mapstructure:"skip_shared" yaml:"skip_shared"`
}

// AttachmentConfig holds attachment handling settings.
type AttachmentConfig struct {
	// Enabled controls whether attachments are extracted and saved.
	Enabled bool `mapstructure:"enabled" yaml:"enabled"`
	// MaxSizeMB is the maximum attachment size in megabytes.
	MaxSizeMB int `mapstructure:"max_size_mb" yaml:"max_size_mb"`
	// Dir is the subdirectory name for attachments relative to each note.
	Dir string `mapstructure:"dir" yaml:"dir"`
}

// DefaultCommitTemplate is the default Go template string for commit messages.
const DefaultCommitTemplate = `apple-notes-sync: {{.Timestamp}} | {{.Written}} notes synced`

// setDefaults configures default values in the viper instance.
func setDefaults(v *viper.Viper) {
	v.SetDefault("repo_path", "")
	v.SetDefault("notes_subdir", "")
	v.SetDefault("dry_run", false)
	v.SetDefault("commit_template", DefaultCommitTemplate)
	v.SetDefault("front_matter", true)
	v.SetDefault("clean_orphans", true)
	v.SetDefault("timeout", "120s")

	v.SetDefault("git.enabled", true)
	v.SetDefault("git.remote", "origin")
	v.SetDefault("git.branch", "main")
	v.SetDefault("git.push", true)

	v.SetDefault("rclone.enabled", false)
	v.SetDefault("rclone.remote_name", "")
	v.SetDefault("rclone.remote_path", "")

	v.SetDefault("log.level", "info")
	v.SetDefault("log.file", "")
	v.SetDefault("log.format", "console")

	v.SetDefault("filter.exclude_folders", []string{"Recently Deleted"})
	v.SetDefault("filter.skip_protected", true)
	v.SetDefault("filter.skip_shared", false)

	v.SetDefault("attachments.enabled", true)
	v.SetDefault("attachments.max_size_mb", 50)
	v.SetDefault("attachments.dir", "_attachments")
}

// Load reads configuration from the given config file path, environment
// variables, and applies defaults. If configPath is empty, it looks for
// .apple-notes-sync.yaml in the user's home directory.
func Load(configPath string) (*Config, error) {
	v := viper.New()
	setDefaults(v)

	// Environment variable binding with ANS_ prefix.
	v.SetEnvPrefix("ANS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("finding home directory: %w", err)
		}
		v.SetConfigName(".apple-notes-sync")
		v.SetConfigType("yaml")
		v.AddConfigPath(home)
		v.AddConfigPath(".")
	}

	// Read config file (not required to exist).
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Only return error if file exists but can't be read.
			if configPath != "" {
				return nil, fmt.Errorf("reading config file %q: %w", configPath, err)
			}
			// Silently ignore if no explicit config path was provided.
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	return &cfg, nil
}

// Validate checks that the configuration is valid and all required fields are set.
func (c *Config) Validate() error {
	if c.RepoPath == "" {
		return fmt.Errorf("repo_path is required")
	}

	// Expand ~ in repo path.
	if strings.HasPrefix(c.RepoPath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("expanding repo_path: %w", err)
		}
		c.RepoPath = filepath.Join(home, c.RepoPath[2:])
	}

	validLogLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true,
	}
	if !validLogLevels[c.Log.Level] {
		return fmt.Errorf("invalid log level %q: must be one of debug, info, warn, error", c.Log.Level)
	}

	validLogFormats := map[string]bool{
		"json": true, "console": true,
	}
	if !validLogFormats[c.Log.Format] {
		return fmt.Errorf("invalid log format %q: must be json or console", c.Log.Format)
	}

	if c.Rclone.Enabled {
		if c.Rclone.RemoteName == "" {
			return fmt.Errorf("rclone.remote_name is required when rclone is enabled")
		}
		if c.Rclone.RemotePath == "" {
			return fmt.Errorf("rclone.remote_path is required when rclone is enabled")
		}
	}

	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be a positive duration")
	}

	if c.Attachments.MaxSizeMB <= 0 {
		return fmt.Errorf("attachments.max_size_mb must be positive")
	}

	return nil
}
