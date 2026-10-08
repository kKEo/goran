package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveLoadAndEnvPrecedence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "agent.json")
	in := &Config{Server: "http://file:8080", Key: "ga_file", Labels: []string{"eu"}}
	require.NoError(t, in.Save(path))

	out, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, in, out)

	t.Setenv("GORAN_SERVER", "http://env:8080")
	t.Setenv("GORAN_AGENT_LABELS", "us, prod")
	t.Setenv("GORAN_POLL_SECONDS", "7")
	out.ApplyEnv()
	assert.Equal(t, "http://env:8080", out.Server)
	assert.Equal(t, "ga_file", out.Key)
	assert.Equal(t, []string{"us", "prod"}, out.Labels)
	assert.Equal(t, 7, out.PollSeconds)
	assert.NoError(t, out.Validate())
}

func TestValidateAndDefaults(t *testing.T) {
	c := &Config{}
	assert.Error(t, c.Validate())
	assert.Equal(t, "work", c.WorkDirOrDefault())
	assert.Equal(t, "2s", c.PollInterval().String())
	_, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	assert.Error(t, err)
}
