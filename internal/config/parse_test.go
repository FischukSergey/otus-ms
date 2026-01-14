package config_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/FischukSergey/otus-ms/internal/config"
)

var configExamplePath string

func init() {
	_, currentFile, _, _ := runtime.Caller(0)
	configExamplePath = filepath.Join(filepath.Dir(currentFile), "..", "..", "configs", "config.example.yaml")
}

func TestParseAndValidate(t *testing.T) {
	cfg, err := config.ParseAndValidate(configExamplePath)
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.Log.Level)
}
