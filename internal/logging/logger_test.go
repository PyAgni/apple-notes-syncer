package logging

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogger_Console(t *testing.T) {
	logger, err := NewLogger("info", "console", "")
	require.NoError(t, err)
	assert.NotNil(t, logger)
}

func TestNewLogger_JSON(t *testing.T) {
	logger, err := NewLogger("debug", "json", "")
	require.NoError(t, err)
	assert.NotNil(t, logger)
}

func TestNewLogger_InvalidLevel(t *testing.T) {
	_, err := NewLogger("invalid", "console", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing log level")
}

func TestNewLogger_AllLevels(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		logger, err := NewLogger(level, "console", "")
		require.NoError(t, err, "level: %s", level)
		assert.NotNil(t, logger, "level: %s", level)
	}
}
