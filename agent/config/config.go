// Package config loads the agent settings from a JSON file, environment
// variables and flags, in that order of increasing precedence.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config is persisted by `goran-agent register` and read by `goran-agent run`.
type Config struct {
	Server      string   `json:"server"`
	Key         string   `json:"key"`
	Name        string   `json:"name,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	WorkDir     string   `json:"workdir,omitempty"`
	PollSeconds int      `json:"poll_seconds,omitempty"`
}

// DefaultPath is $GORAN_AGENT_CONFIG or ./agent.json.
func DefaultPath() string {
	if v := os.Getenv("GORAN_AGENT_CONFIG"); v != "" {
		return v
	}
	return "agent.json"
}

// Load reads the file. A missing file yields an empty config and os.ErrNotExist.
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return &Config{}, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &c, nil
}

// Save writes the file readable by the owner only, since it holds the agent key.
func (c *Config) Save(path string) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}

// ApplyEnv overlays GORAN_* environment variables.
func (c *Config) ApplyEnv() {
	if v := os.Getenv("GORAN_SERVER"); v != "" {
		c.Server = v
	}
	if v := os.Getenv("GORAN_AGENT_KEY"); v != "" {
		c.Key = v
	}
	if v := os.Getenv("GORAN_AGENT_NAME"); v != "" {
		c.Name = v
	}
	if v := os.Getenv("GORAN_AGENT_LABELS"); v != "" {
		c.Labels = SplitLabels(v)
	}
	if v := os.Getenv("GORAN_WORKDIR"); v != "" {
		c.WorkDir = v
	}
	if v, err := strconv.Atoi(os.Getenv("GORAN_POLL_SECONDS")); err == nil && v > 0 {
		c.PollSeconds = v
	}
}

// Validate checks the fields `run` cannot do without.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.Server) == "" {
		return errors.New("server URL is required (--server, GORAN_SERVER or the config file)")
	}
	if strings.TrimSpace(c.Key) == "" {
		return errors.New("agent key is required: run `goran-agent register` first, or set GORAN_AGENT_KEY")
	}
	return nil
}

// PollInterval defaults to two seconds.
func (c *Config) PollInterval() time.Duration {
	if c.PollSeconds <= 0 {
		return 2 * time.Second
	}
	return time.Duration(c.PollSeconds) * time.Second
}

// WorkDirOrDefault defaults to ./work.
func (c *Config) WorkDirOrDefault() string {
	if c.WorkDir == "" {
		return "work"
	}
	return c.WorkDir
}

// SplitLabels parses "a, b,c" into ["a","b","c"].
func SplitLabels(s string) []string {
	var out []string
	for _, l := range strings.Split(s, ",") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}
