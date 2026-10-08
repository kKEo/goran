package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvHelpers(t *testing.T) {
	t.Setenv("GORAN_TEST_STR", "x")
	t.Setenv("GORAN_TEST_INT", "42")
	assert.Equal(t, "x", envOr("GORAN_TEST_STR", "def"))
	assert.Equal(t, "def", envOr("GORAN_TEST_MISSING", "def"))
	assert.Equal(t, 42, envInt("GORAN_TEST_INT", 1))
	assert.Equal(t, 1, envInt("GORAN_TEST_MISSING", 1))
}

func TestBootstrapCommandCreatesDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "goran.sqlite")
	require.NoError(t, bootstrap([]string{"--db", dbPath, "--user", "admin", "--workspace", "acme"}))
	// Running it twice must reuse the user and workspace instead of failing.
	require.NoError(t, bootstrap([]string{"--db", dbPath, "--user", "admin", "--workspace", "acme"}))
	assert.FileExists(t, dbPath)
}

func TestKeygen(t *testing.T) {
	require.NoError(t, keygen())
}

func TestServeRequiresMasterKey(t *testing.T) {
	t.Setenv("GORAN_MASTER_KEY", "")
	err := serve([]string{"--db", filepath.Join(t.TempDir(), "x.sqlite")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GORAN_MASTER_KEY")
}
